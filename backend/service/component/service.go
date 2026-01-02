package component

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service/shared"
)

// 错误定义
var (
	ErrComponentNotFound = errors.New("component not found")
	ErrInstallInProgress = errors.New("installation already in progress")
	ErrDownloadFailed    = errors.New("download failed")
	ErrExtractionFailed  = errors.New("extraction failed")
)

// Service 组件服务
type Service struct {
	repo repository.ComponentRepository

	mu         sync.Mutex
	installing map[string]struct{}
}

// NewService 创建组件服务
func NewService(repo repository.ComponentRepository) *Service {
	return &Service{
		repo:       repo,
		installing: make(map[string]struct{}),
	}
}

// ========== CRUD 操作 ==========

// List 列出所有组件
func (s *Service) List(ctx context.Context) ([]domain.CoreComponent, error) {
	if err := s.EnsureDefaultComponents(ctx); err != nil {
		return nil, err
	}

	components, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// 检测本地安装
	for i, comp := range components {
		if comp.InstallDir == "" {
			if info := s.detectInstalled(comp.Kind); info != nil {
				comp.InstallDir = info.dir
				comp.LastInstalledAt = info.modTime
				components[i] = comp
				// 更新存储
				s.repo.SetInstalled(ctx, comp.ID, info.dir, comp.LastVersion, comp.Checksum)
			}
		}
	}

	return components, nil
}

// Get 获取组件
func (s *Service) Get(ctx context.Context, id string) (domain.CoreComponent, error) {
	return s.repo.Get(ctx, id)
}

// Create 创建组件
func (s *Service) Create(ctx context.Context, comp domain.CoreComponent) (domain.CoreComponent, error) {
	// 核心组件（xray/sing-box）使用幂等创建：缺失时补齐默认配置，存在则直接返回
	if comp.Kind == domain.ComponentXray || comp.Kind == domain.ComponentSingBox {
		if err := s.EnsureDefaultComponents(ctx); err != nil {
			return domain.CoreComponent{}, err
		}
		return s.repo.GetByKind(ctx, comp.Kind)
	}
	return s.repo.Create(ctx, comp)
}

// Update 更新组件
func (s *Service) Update(ctx context.Context, id string, comp domain.CoreComponent) (domain.CoreComponent, error) {
	return s.repo.Update(ctx, id, comp)
}

// Delete 删除组件
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ========== 安装操作 ==========

// Install 安装组件（异步）
func (s *Service) Install(ctx context.Context, id string) (domain.CoreComponent, error) {
	s.mu.Lock()
	if _, ok := s.installing[id]; ok {
		s.mu.Unlock()
		return s.repo.Get(ctx, id)
	}
	s.installing[id] = struct{}{}
	s.mu.Unlock()

	// 更新状态为安装中
	s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusDownloading, 0, "Starting download...")

	// 异步安装
	go s.doInstall(id)

	return s.repo.Get(ctx, id)
}

// ========== 内部方法 ==========

type installInfo struct {
	dir     string
	modTime time.Time
}

func (s *Service) detectInstalled(kind domain.CoreComponentKind) *installInfo {
	var subdir, binary string

	switch kind {
	case domain.ComponentXray:
		subdir = "core/xray"
		binary = "xray"
	case domain.ComponentSingBox:
		subdir = "core/sing-box"
		binary = "sing-box"
	default:
		return nil
	}

	for _, root := range shared.ArtifactsSearchRoots() {
		dir := filepath.Join(root, subdir)
		binaryPath, err := shared.FindBinaryInDir(dir, []string{binary, binary + ".exe"})
		if err != nil {
			continue
		}
		info, err := os.Stat(binaryPath)
		if err != nil {
			continue
		}

		return &installInfo{
			dir:     dir,
			modTime: info.ModTime(),
		}
	}

	return nil
}

func (s *Service) doInstall(id string) {
	ctx := context.Background()

	defer func() {
		s.mu.Lock()
		delete(s.installing, id)
		s.mu.Unlock()
	}()

	comp, err := s.repo.Get(ctx, id)
	if err != nil {
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, err.Error())
		return
	}

	// 确定组件类型和仓库
	var kindStr string
	switch comp.Kind {
	case domain.ComponentXray:
		kindStr = "xray"
	case domain.ComponentSingBox:
		kindStr = "singbox"
	default:
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, "Unknown component kind")
		return
	}

	// 获取仓库和资源候选
	repo := shared.GetComponentRepo(kindStr)
	candidates, err := shared.GetComponentAssetCandidates(kindStr)
	if err != nil {
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, err.Error())
		return
	}

	// 更新状态：获取下载信息
	s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusDownloading, 10, "正在获取下载地址...")

	// 获取下载信息
	releaseInfo, err := shared.GetComponentDownloadInfo(repo, candidates)
	if err != nil {
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, "获取下载信息失败: "+err.Error())
		return
	}

	downloadURL := releaseInfo.DownloadURL
	archiveType := shared.InferArchiveType(releaseInfo.Name)

	// 更新状态：开始下载
	s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusDownloading, 20, "正在下载...")

	// 下载资源
	data, checksum, err := shared.DownloadWithProgress(downloadURL, func(downloaded, total int64, percent int) {
		progress := 20 + (percent * 50 / 100)
		var message string
		if total > 0 {
			downloadedMB := float64(downloaded) / (1024 * 1024)
			totalMB := float64(total) / (1024 * 1024)
			message = fmt.Sprintf("正在下载... %.2f/%.2f MB (%d%%)", downloadedMB, totalMB, percent)
		} else {
			downloadedMB := float64(downloaded) / (1024 * 1024)
			message = fmt.Sprintf("正在下载... %.2f MB", downloadedMB)
		}
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusDownloading, progress, message)
	})

	if err != nil {
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, "下载失败: "+err.Error())
		return
	}

	// 更新状态：解压中
	s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusExtracting, 70, "正在解压安装...")

	// 确定安装目录
	targetDir := s.resolveInstallDir(comp)

	// 解压
	installDir, err := shared.ExtractArchive(targetDir, archiveType, data)
	if err != nil {
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, "解压失败: "+err.Error())
		return
	}

	// 清理多余文件（Xray 特有）
	if comp.Kind == domain.ComponentXray {
		s.cleanupXrayInstall(installDir)
	}

	// 设置二进制文件执行权限
	s.setExecutablePermissions(installDir, kindStr)

	// sing-box 依赖 rule-set（.srs）文件；缺失会在运行期直接 FATAL
	if comp.Kind == domain.ComponentSingBox {
		s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusDownloading, 85, "正在下载 rule-set...")
		if err := shared.EnsureSingBoxRuleSets(nil); err != nil {
			s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, "rule-set 下载失败: "+err.Error())
			return
		}
	}

	// 更新状态：完成
	s.repo.SetInstalled(ctx, id, installDir, releaseInfo.Version, checksum)
	s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusDone, 100, "安装完成")
}

// EnsureDefaultComponents 确保默认组件存在
func (s *Service) EnsureDefaultComponents(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保 Xray 组件存在
	if _, err := s.repo.GetByKind(ctx, domain.ComponentXray); err != nil {
		if !errors.Is(err, repository.ErrComponentNotFound) {
			return err
		}
		if _, err := s.repo.Create(ctx, domain.CoreComponent{
			Name: "Xray",
			Kind: domain.ComponentXray,
			Meta: map[string]string{
				"repo": "XTLS/Xray-core",
			},
		}); err != nil {
			return err
		}
	}

	// 确保 sing-box 组件存在
	if _, err := s.repo.GetByKind(ctx, domain.ComponentSingBox); err != nil {
		if !errors.Is(err, repository.ErrComponentNotFound) {
			return err
		}
		if _, err := s.repo.Create(ctx, domain.CoreComponent{
			Name: "sing-box",
			Kind: domain.ComponentSingBox,
			Meta: map[string]string{
				"repo": "SagerNet/sing-box",
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

// resolveInstallDir 确定组件的安装目录
func (s *Service) resolveInstallDir(comp domain.CoreComponent) string {
	base := filepath.Join(shared.ArtifactsRoot, "core")
	switch comp.Kind {
	case domain.ComponentXray:
		return filepath.Join(base, "xray")
	case domain.ComponentSingBox:
		return filepath.Join(base, "sing-box")
	default:
		return filepath.Join(base, comp.Name)
	}
}

// cleanupXrayInstall 清理 Xray 安装目录中的多余文件
func (s *Service) cleanupXrayInstall(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	allowed := map[string]struct{}{
		"xray":                {},
		"xray.exe":            {},
		"geoip.dat":           {},
		"geosite.dat":         {},
		"config.json":         {},
		"config-measure.json": {},
		"license":             {},
		"license.txt":         {},
	}

	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if _, ok := allowed[lower]; ok {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	return nil
}

// setExecutablePermissions 设置二进制文件的执行权限
func (s *Service) setExecutablePermissions(dir, kind string) {
	var binaries []string
	switch kind {
	case "xray":
		binaries = []string{"xray", "xray.exe"}
	case "singbox":
		binaries = []string{"sing-box", "sing-box.exe"}
	}

	for _, bin := range binaries {
		path := filepath.Join(dir, bin)
		if _, err := os.Stat(path); err == nil {
			os.Chmod(path, 0755)
		}
	}

	// 递归搜索子目录（sing-box 解压后可能在子目录）
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(dir, entry.Name())
			for _, bin := range binaries {
				path := filepath.Join(subdir, bin)
				if _, err := os.Stat(path); err == nil {
					os.Chmod(path, 0755)
				}
			}
		}
	}
}
