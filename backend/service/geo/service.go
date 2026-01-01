package geo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service/shared"
)

// 错误定义
var (
	ErrGeoNotFound    = errors.New("geo resource not found")
	ErrDownloadFailed = errors.New("download failed")
)

// 默认 Geo 资源 URL
const (
	DefaultGeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	DefaultGeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
)

// Service Geo 资源服务
type Service struct {
	repo repository.GeoRepository
}

// NewService 创建 Geo 服务
func NewService(repo repository.GeoRepository) *Service {
	return &Service{repo: repo}
}

// ========== CRUD 操作 ==========

// List 列出所有 Geo 资源
func (s *Service) List(ctx context.Context) ([]domain.GeoResource, error) {
	return s.repo.List(ctx)
}

// Get 获取 Geo 资源
func (s *Service) Get(ctx context.Context, id string) (domain.GeoResource, error) {
	return s.repo.Get(ctx, id)
}

// GetByType 按类型获取 Geo 资源
func (s *Service) GetByType(ctx context.Context, geoType domain.GeoResourceType) (domain.GeoResource, error) {
	return s.repo.GetByType(ctx, geoType)
}

// Upsert 插入或更新 Geo 资源
func (s *Service) Upsert(ctx context.Context, geo domain.GeoResource) (domain.GeoResource, error) {
	return s.repo.Upsert(ctx, geo)
}

// Delete 删除 Geo 资源
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ========== 同步操作 ==========

// Sync 同步单个 Geo 资源
func (s *Service) Sync(ctx context.Context, id string) error {
	geo, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if geo.SourceURL == "" {
		return nil
	}

	// 确定保存路径
	geoDir := filepath.Join(shared.ArtifactsRoot, shared.GeoDir)
	if err := os.MkdirAll(geoDir, 0755); err != nil {
		return err
	}

	var filename string
	switch geo.Type {
	case domain.GeoIP:
		filename = "geoip.dat"
	case domain.GeoSite:
		filename = "geosite.dat"
	default:
		filename = geo.Name + ".dat"
	}
	savePath := filepath.Join(geoDir, filename)

	// 下载
	checksum, fileSize, err := s.downloadFile(geo.SourceURL, savePath)
	if err != nil {
		geo.LastSyncError = err.Error()
		s.repo.Update(ctx, id, geo)
		return err
	}

	// 更新资源信息
	geo.ArtifactPath = savePath
	geo.Checksum = checksum
	geo.FileSizeBytes = fileSize
	geo.LastSynced = time.Now()
	geo.LastSyncError = ""

	s.repo.Update(ctx, id, geo)
	return nil
}

// SyncAll 同步所有 Geo 资源
func (s *Service) SyncAll(ctx context.Context) {
	resources, err := s.repo.List(ctx)
	if err != nil {
		return
	}

	for _, geo := range resources {
		if geo.SourceURL != "" {
			if err := s.Sync(ctx, geo.ID); err != nil {
				log.Printf("[GeoSync] sync failed for %s: %v", geo.ID, err)
			}
		}
	}
}

// EnsureDefaultResources 确保默认资源存在
func (s *Service) EnsureDefaultResources(ctx context.Context) error {
	// 确保 GeoIP 存在
	if _, err := s.repo.GetByType(ctx, domain.GeoIP); err != nil {
		s.repo.Create(ctx, domain.GeoResource{
			Name:      "GeoIP",
			Type:      domain.GeoIP,
			SourceURL: DefaultGeoIPURL,
		})
	}

	// 确保 GeoSite 存在
	if _, err := s.repo.GetByType(ctx, domain.GeoSite); err != nil {
		s.repo.Create(ctx, domain.GeoResource{
			Name:      "GeoSite",
			Type:      domain.GeoSite,
			SourceURL: DefaultGeoSiteURL,
		})
	}

	return nil
}

// ========== 内部方法 ==========

func (s *Service) downloadFile(url, savePath string) (checksum string, fileSize int64, err error) {
	resp, err := shared.HTTPClient.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, errors.New("download failed: " + resp.Status)
	}

	// 创建临时文件
	tmpPath := savePath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", 0, err
	}

	// 同时计算校验和和复制数据
	hash := sha256.New()
	reader := io.TeeReader(io.LimitReader(resp.Body, shared.MaxDownloadSize), hash)

	n, err := io.Copy(f, reader)
	f.Close()

	if err != nil {
		os.Remove(tmpPath)
		return "", 0, err
	}

	// 原子性移动
	if err := os.Rename(tmpPath, savePath); err != nil {
		os.Remove(tmpPath)
		return "", 0, err
	}

	return hex.EncodeToString(hash.Sum(nil)), n, nil
}

// GetGeoFilePaths 获取 Geo 文件路径
func (s *Service) GetGeoFilePaths(ctx context.Context) (geoIP, geoSite string) {
	geoDir := filepath.Join(shared.ArtifactsRoot, shared.GeoDir)
	geoIP = filepath.Join(geoDir, "geoip.dat")
	geoSite = filepath.Join(geoDir, "geosite.dat")
	return
}
