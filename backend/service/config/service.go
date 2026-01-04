package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service/node"
	"vea/backend/service/nodes"
	"vea/backend/service/shared"
)

// 常量定义
const (
	// subscriptionUserAgent 订阅服务专用 User-Agent
	// 使用 Clash 风格的 UA 以确保被大多数订阅服务接受
	subscriptionUserAgent = "ClashForAndroid/2.5.12"
)

// 错误定义
var (
	ErrConfigNotFound = errors.New("config not found")
	ErrSyncFailed     = errors.New("sync failed")
)

// Service 配置服务
type Service struct {
	repo        repository.ConfigRepository
	nodeService *nodes.Service
}

type subscriptionParseError struct {
	configID string
	message  string
}

func (e *subscriptionParseError) Error() string {
	if e == nil {
		return "subscription parse failed"
	}
	if strings.TrimSpace(e.configID) == "" {
		return e.message
	}
	return "config " + e.configID + ": " + e.message
}

func (e *subscriptionParseError) Unwrap() error {
	return repository.ErrInvalidData
}

// NewService 创建配置服务
func NewService(repo repository.ConfigRepository, nodeService *nodes.Service) *Service {
	return &Service{
		repo:        repo,
		nodeService: nodeService,
	}
}

// ========== CRUD 操作 ==========

// List 列出所有配置
func (s *Service) List(ctx context.Context) ([]domain.Config, error) {
	return s.repo.List(ctx)
}

// Get 获取配置
func (s *Service) Get(ctx context.Context, id string) (domain.Config, error) {
	return s.repo.Get(ctx, id)
}

// Create 创建配置
func (s *Service) Create(ctx context.Context, cfg domain.Config) (domain.Config, error) {
	// 如果有 SourceURL，先同步
	if cfg.SourceURL != "" {
		payload, checksum, err := s.downloadConfig(cfg.SourceURL)
		if err != nil {
			cfg.LastSyncError = err.Error()
		} else {
			cfg.Payload = payload
			cfg.Checksum = checksum
			cfg.LastSyncedAt = time.Now()
		}
	}

	created, err := s.repo.Create(ctx, cfg)
	if err != nil {
		return domain.Config{}, err
	}

	// 解析并同步节点（为避免破坏用户配置，解析失败时不清空旧节点）。
	if err := s.syncNodesFromPayload(ctx, created.ID, created.Payload); err != nil {
		// 创建配置时不应因为解析失败就直接失败：记录错误即可。
		if updateErr := s.repo.UpdateSyncStatus(ctx, created.ID, created.Payload, created.Checksum, err); updateErr != nil {
			log.Printf("[ConfigCreate] failed to update sync status for %s after parse error: %v", created.ID, updateErr)
		}
	}

	return created, nil
}

// Update 更新配置
func (s *Service) Update(ctx context.Context, id string, cfg domain.Config) (domain.Config, error) {
	return s.repo.Update(ctx, id, cfg)
}

// Delete 删除配置
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ========== 同步操作 ==========

// Sync 同步配置
func (s *Service) Sync(ctx context.Context, id string) error {
	cfg, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if cfg.SourceURL == "" {
		return nil // 没有 SourceURL，无需同步
	}

	payload, checksum, err := s.downloadConfig(cfg.SourceURL)
	if err != nil {
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, err); updateErr != nil {
			log.Printf("[ConfigSync] failed to update sync status for %s after download error: %v", id, updateErr)
		}
		return err
	}

	// 如果内容没变，只更新同步时间
	if checksum == cfg.Checksum {
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, nil); updateErr != nil {
			log.Printf("[ConfigSync] failed to update sync status for %s when checksum unchanged: %v", id, updateErr)
		}
		// 内容不变也要保证解析状态正确：否则会把 LastSyncError “误清空”。
		if err := s.syncNodesFromPayload(ctx, id, cfg.Payload); err != nil {
			if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, err); updateErr != nil {
				log.Printf("[ConfigSync] failed to update sync status for %s after parse error: %v", id, updateErr)
			}
			return err
		}
		return nil
	}

	// 更新内容
	if updateErr := s.repo.UpdateSyncStatus(ctx, id, payload, checksum, nil); updateErr != nil {
		log.Printf("[ConfigSync] failed to update sync status for %s after download: %v", id, updateErr)
	}

	// 解析并更新节点（解析失败时不清空旧节点）。
	if err := s.syncNodesFromPayload(ctx, id, payload); err != nil {
		// 下载成功但解析失败：保留旧节点，同时把错误记录到配置上，便于前端展示。
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, payload, checksum, err); updateErr != nil {
			log.Printf("[ConfigSync] failed to update sync status for %s after parse error: %v", id, updateErr)
		}
		return err
	}

	return nil
}

// SyncAll 同步所有配置
func (s *Service) SyncAll(ctx context.Context) {
	configs, err := s.repo.List(ctx)
	if err != nil {
		return
	}

	for _, cfg := range configs {
		if cfg.SourceURL != "" && cfg.AutoUpdateInterval > 0 {
			// 检查是否需要同步
			if time.Since(cfg.LastSyncedAt) >= cfg.AutoUpdateInterval {
				if err := s.Sync(ctx, cfg.ID); err != nil {
					log.Printf("[ConfigSync] sync failed for %s: %v", cfg.ID, err)
				}
			}
		}
	}
}

// ========== 内部方法 ==========

func (s *Service) syncNodesFromPayload(ctx context.Context, configID, payload string) error {
	if s.nodeService == nil || strings.TrimSpace(configID) == "" {
		return nil
	}

	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		if _, err := s.nodeService.ReplaceNodesForConfig(ctx, configID, nil); err != nil {
			log.Printf("[ConfigSync] clear nodes failed for %s: %v", configID, err)
			return err
		}
		return nil
	}

	nodes, errs := node.ParseMultipleLinks(payload)
	if len(errs) > 0 {
		log.Printf("[ConfigSync] parse errors for %s: %d", configID, len(errs))
	}

	if len(nodes) == 0 {
		// payload 非空但解析不到节点：这通常是订阅格式不支持（如 Clash YAML/HTML 错误页/升级提示）。
		// 为避免破坏已有配置，这里不清空旧节点。
		return &subscriptionParseError{
			configID: configID,
			message:  "订阅内容无法解析为节点（仅支持 vmess/vless/trojan/ss 分享链接）；已保留现有节点",
		}
	}

	if _, err := s.nodeService.ReplaceNodesForConfig(ctx, configID, nodes); err != nil {
		log.Printf("[ConfigSync] update nodes failed for %s: %v", configID, err)
		return err
	}
	return nil
}

func (s *Service) downloadConfig(sourceURL string) (payload, checksum string, err error) {
	// 使用订阅专用 User-Agent
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", subscriptionUserAgent)

	resp, err := shared.HTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", errors.New("download failed: " + resp.Status)
	}

	// 限制下载大小
	limitedReader := io.LimitReader(resp.Body, shared.MaxDownloadSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", "", err
	}

	// 计算校验和
	hash := sha256.Sum256(data)
	checksum = hex.EncodeToString(hash[:])

	return string(data), checksum, nil
}
