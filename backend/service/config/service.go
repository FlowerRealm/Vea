package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
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

	// 如果有 payload，解析并创建节点
	if created.Payload != "" {
		nodes, _ := node.ParseMultipleLinks(created.Payload)
		if len(nodes) > 0 {
			s.nodeService.ReplaceNodesForConfig(ctx, created.ID, nodes)
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
		s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, err)
		return err
	}

	// 如果内容没变，只更新同步时间
	if checksum == cfg.Checksum {
		s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, nil)
		return nil
	}

	// 更新内容
	s.repo.UpdateSyncStatus(ctx, id, payload, checksum, nil)

	// 解析并更新节点
	nodes, _ := node.ParseMultipleLinks(payload)
	if len(nodes) > 0 {
		s.nodeService.ReplaceNodesForConfig(ctx, id, nodes)
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
				s.Sync(ctx, cfg.ID)
			}
		}
	}
}

// ========== 内部方法 ==========

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
