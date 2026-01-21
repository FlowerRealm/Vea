package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"reflect"
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
	subscriptionUserAgent = "ClashForAndroid/2.6.0"
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
	frouterRepo repository.FRouterRepository
	bgCtx       context.Context
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
func NewService(bgCtx context.Context, repo repository.ConfigRepository, nodeService *nodes.Service, frouterRepo repository.FRouterRepository) *Service {
	if bgCtx == nil {
		bgCtx = context.Background()
	}
	return &Service{
		repo:        repo,
		nodeService: nodeService,
		frouterRepo: frouterRepo,
		bgCtx:       bgCtx,
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
	created, err := s.repo.Create(ctx, cfg)
	if err != nil {
		return domain.Config{}, err
	}

	// 手动配置（无 SourceURL）需要立即从 payload 解析节点，否则该配置会一直为空。
	if strings.TrimSpace(created.SourceURL) == "" {
		if strings.TrimSpace(created.Payload) != "" {
			if parseErr := s.syncNodesFromPayload(ctx, created.ID, created.Payload); parseErr != nil {
				log.Printf("[ConfigCreate] parse payload failed for %s: %v", created.ID, parseErr)
			}
		}
		return created, nil
	}

	if strings.TrimSpace(created.SourceURL) != "" {
		createdID := created.ID
		fallbackPayload := created.Payload
		go func() {
			bgCtx := s.bgCtx
			if err := s.Sync(bgCtx, createdID); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				if strings.TrimSpace(fallbackPayload) != "" {
					if parseErr := s.syncNodesFromPayload(bgCtx, createdID, fallbackPayload); parseErr != nil {
						log.Printf("[ConfigCreate] initial sync failed for %s: %v (fallback parse failed: %v)", createdID, err, parseErr)
					} else {
						hash := sha256.Sum256([]byte(fallbackPayload))
						checksum := hex.EncodeToString(hash[:])
						if updateErr := s.repo.UpdateSyncStatus(bgCtx, createdID, fallbackPayload, checksum, nil, nil, nil); updateErr != nil {
							log.Printf("[ConfigCreate] initial sync failed for %s: %v (fallback parsed but update sync status failed: %v)", createdID, err, updateErr)
						} else {
							log.Printf("[ConfigCreate] initial sync failed for %s: %v (fallback payload parsed successfully)", createdID, err)
						}
					}
					return
				}
				log.Printf("[ConfigCreate] initial sync failed for %s: %v", createdID, err)
			}
		}()
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

	payload, checksum, usedBytes, totalBytes, err := s.downloadConfig(ctx, cfg.SourceURL)
	if err != nil {
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, err, nil, nil); updateErr != nil {
			log.Printf("[ConfigSync] failed to update sync status for %s after download error: %v", id, updateErr)
		}
		return err
	}

	// 订阅返回空内容时，保留现有节点与旧 payload，避免数据丢失与 FRouter 引用断裂。
	// 空订阅通常意味着服务端异常/限流/返回错误页等，不应被当作“清空节点”的指令。
	if strings.TrimSpace(payload) == "" {
		emptyErr := &subscriptionParseError{
			configID: id,
			message:  "订阅内容为空；未更新节点（如有现有节点将保持不变）",
		}
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, emptyErr, usedBytes, totalBytes); updateErr != nil {
			log.Printf("[ConfigSync] failed to update sync status for %s after empty payload: %v", id, updateErr)
		}
		return emptyErr
	}

	// 如果内容没变，只更新同步时间
	if checksum == cfg.Checksum {
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, nil, usedBytes, totalBytes); updateErr != nil {
			log.Printf("[ConfigSync] failed to update sync status for %s when checksum unchanged: %v", id, updateErr)
		}
		// 内容不变也要保证解析状态正确：否则会把 LastSyncError “误清空”。
		if err := s.syncNodesFromPayload(ctx, id, cfg.Payload); err != nil {
			if updateErr := s.repo.UpdateSyncStatus(ctx, id, cfg.Payload, cfg.Checksum, err, usedBytes, totalBytes); updateErr != nil {
				log.Printf("[ConfigSync] failed to update sync status for %s after parse error: %v", id, updateErr)
			}
			return err
		}
		return nil
	}

	// 更新内容
	if updateErr := s.repo.UpdateSyncStatus(ctx, id, payload, checksum, nil, usedBytes, totalBytes); updateErr != nil {
		log.Printf("[ConfigSync] failed to update sync status for %s after download: %v", id, updateErr)
	}

	// 解析并更新节点（解析失败时不清空旧节点）。
	if err := s.syncNodesFromPayload(ctx, id, payload); err != nil {
		// 下载成功但解析失败：保留旧节点，同时把错误记录到配置上，便于前端展示。
		if updateErr := s.repo.UpdateSyncStatus(ctx, id, payload, checksum, err, usedBytes, totalBytes); updateErr != nil {
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
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
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

	existingNodes := []domain.Node(nil)
	existingIndex := existingNodeIDIndex{}
	if existing, err := s.nodeService.ListByConfigID(ctx, configID); err == nil && len(existing) > 0 {
		existingNodes = existing
		existingIndex = buildExistingNodeIDIndex(existing)
	}
	existingSubIndex := buildExistingSubscriptionNodeIndex(existingNodes)

	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		// 空 payload 不应触发“清空节点”。订阅可能短暂返回空内容，清空会造成不可逆的数据丢失。
		return nil
	}

	nodes, errs := node.ParseMultipleLinks(payload)
	if len(errs) > 0 {
		log.Printf("[ConfigSync] parse errors for %s: %d", configID, len(errs))
	}

	if len(nodes) > 0 {
		// 分享链接订阅：仅更新节点；如之前生成过 Clash YAML 的订阅 FRouter，清理掉以避免残留。
		nodes = normalizeAndDisambiguateSubscriptionSourceKeys(nodes)
		nodes, _ = reuseNodeIDs(existingIndex, nodes)
		nodes, _ = reuseNodeIDsBySubscriptionKey(existingSubIndex, nodes)

		nextNodes, err := s.nodeService.ReplaceNodesForConfig(ctx, configID, nodes)
		if err != nil {
			log.Printf("[ConfigSync] update nodes failed for %s: %v", configID, err)
			return err
		}
		if s.frouterRepo != nil {
			frouterID := stableFRouterIDForConfig(configID)
			if err := s.frouterRepo.Delete(ctx, frouterID); err != nil && !errors.Is(err, repository.ErrFRouterNotFound) {
				log.Printf("[ConfigSync] clear frouter failed for %s: %v", configID, err)
				return err
			}
		}
		idMap := buildSubscriptionNodeIDRewriteMap(existingNodes, nextNodes)
		if err := s.rewriteFRoutersNodeIDs(ctx, idMap); err != nil {
			log.Printf("[ConfigSync] rewrite frouters failed for %s: %v", configID, err)
			return err
		}
		return nil
	}

	// ParseMultipleLinks() 解析不到节点时，才尝试 Clash YAML。
	// 避免对明显不是 Clash YAML 的内容做 YAML 解析（HTML 错误页/升级提示等），减少无意义的二次失败。
	if !looksLikeClashSubscriptionYAML(trimmed) {
		return &subscriptionParseError{
			configID: configID,
			message:  "订阅内容无法解析为节点（支持 vmess/vless/trojan/ss 分享链接与 Clash YAML）；已保留现有节点",
		}
	}

	// 尝试解析 Clash YAML 订阅（nodes + rules/proxy-groups）并生成订阅 FRouter。
	clashResult, err := parseClashSubscription(configID, payload)
	if err != nil {
		// payload 非空但解析不到节点：这通常是订阅格式不支持（如 HTML 错误页/升级提示）。
		// 为避免破坏已有配置，这里不清空旧节点。
		return &subscriptionParseError{
			configID: configID,
			message:  "订阅内容无法解析为节点（支持 vmess/vless/trojan/ss 分享链接与 Clash YAML）；已保留现有节点",
		}
	}
	if len(clashResult.Warnings) > 0 {
		log.Printf("[ConfigSync] clash parse warnings for %s: %d", configID, len(clashResult.Warnings))
		for i, w := range clashResult.Warnings {
			if i >= 8 {
				break
			}
			log.Printf("[ConfigSync] clash warning: %s", w)
		}
	}
	var idMap map[string]string
	clashResult.Nodes, idMap = reuseNodeIDs(existingIndex, clashResult.Nodes)
	clashResult.Chain = rewriteChainProxyNodeIDs(clashResult.Chain, idMap)
	if _, err := s.nodeService.ReplaceNodesForConfig(ctx, configID, clashResult.Nodes); err != nil {
		log.Printf("[ConfigSync] update nodes failed for %s: %v", configID, err)
		return err
	}
	if s.frouterRepo != nil {
		cfg, getErr := s.repo.Get(ctx, configID)
		if getErr != nil {
			log.Printf("[ConfigSync] get config failed for %s when upserting frouter: %v", configID, getErr)
		}
		frouterID := stableFRouterIDForConfig(configID)
		next := domain.FRouter{
			ID:             frouterID,
			Name:           "订阅: " + strings.TrimSpace(cfg.Name),
			SourceConfigID: configID,
			ChainProxy:     clashResult.Chain,
		}
		if strings.TrimSpace(cfg.Name) == "" {
			next.Name = "订阅 FRouter"
		}

		existing, getErr := s.frouterRepo.Get(ctx, frouterID)
		if getErr == nil {
			next.Tags = existing.Tags
			next.LastLatencyMS = existing.LastLatencyMS
			next.LastLatencyAt = existing.LastLatencyAt
			next.LastLatencyError = existing.LastLatencyError
			next.LastSpeedMbps = existing.LastSpeedMbps
			next.LastSpeedAt = existing.LastSpeedAt
			next.LastSpeedError = existing.LastSpeedError
			if _, err := s.frouterRepo.Update(ctx, frouterID, next); err != nil {
				log.Printf("[ConfigSync] update frouter failed for %s: %v", configID, err)
				return err
			}
			return nil
		}
		if !errors.Is(getErr, repository.ErrFRouterNotFound) {
			log.Printf("[ConfigSync] get frouter failed for %s: %v", configID, getErr)
			return getErr
		}
		if _, err := s.frouterRepo.Create(ctx, next); err != nil {
			log.Printf("[ConfigSync] create frouter failed for %s: %v", configID, err)
			return err
		}
	}
	if err := s.rewriteFRoutersNodeIDs(ctx, idMap); err != nil {
		log.Printf("[ConfigSync] rewrite frouters failed for %s: %v", configID, err)
		return err
	}
	return nil
}

func looksLikeClashSubscriptionYAML(payload string) bool {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return false
	}
	// 只做“很明显”的正向特征匹配，避免误伤：
	// Clash YAML 常见顶层字段：proxies / proxy-groups / rules。
	lower := strings.ToLower(payload)
	keys := []string{"proxies:", "proxy-groups:", "rules:"}
	for _, k := range keys {
		if strings.HasPrefix(lower, k) || strings.Contains(lower, "\n"+k) || strings.Contains(lower, "\r\n"+k) {
			return true
		}
	}
	return false
}

func (s *Service) rewriteFRoutersNodeIDs(ctx context.Context, idMap map[string]string) error {
	if s == nil || s.frouterRepo == nil || len(idMap) == 0 {
		return nil
	}

	frouters, err := s.frouterRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, fr := range frouters {
		chain := fr.ChainProxy
		if len(chain.Edges) == 0 && len(chain.Slots) == 0 && len(chain.Positions) == 0 {
			continue
		}
		next := rewriteChainProxyNodeIDs(chain, idMap)
		if reflect.DeepEqual(chain, next) {
			continue
		}
		fr.ChainProxy = next
		if _, err := s.frouterRepo.Update(ctx, fr.ID, fr); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) downloadConfig(ctx context.Context, sourceURL string) (payload, checksum string, usedBytes, totalBytes *int64, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// 使用订阅专用 User-Agent
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", "", nil, nil, err
	}
	req.Header.Set("User-Agent", subscriptionUserAgent)

	resp, err := shared.HTTPClient.Do(req)
	if err != nil {
		return "", "", nil, nil, err
	}
	defer resp.Body.Close()

	usedBytes, totalBytes = parseSubscriptionUserinfo(resp.Header.Get("subscription-userinfo"))

	if resp.StatusCode != http.StatusOK {
		return "", "", usedBytes, totalBytes, errors.New("download failed: " + resp.Status)
	}

	// 限制下载大小
	limitedReader := io.LimitReader(resp.Body, shared.MaxDownloadSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", "", usedBytes, totalBytes, err
	}

	// 计算校验和
	hash := sha256.Sum256(data)
	checksum = hex.EncodeToString(hash[:])

	return string(data), checksum, usedBytes, totalBytes, nil
}
