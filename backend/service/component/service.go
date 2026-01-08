package component

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	// 核心组件（sing-box/clash）使用幂等创建：缺失时补齐默认配置，存在则直接返回
	if comp.Kind == domain.ComponentSingBox || comp.Kind == domain.ComponentClash {
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

// Uninstall 卸载组件：删除本地安装文件并清除安装信息。
//
// 注意：默认仅允许卸载位于 artifacts 目录下的安装路径，避免误删任意目录。
func (s *Service) Uninstall(ctx context.Context, id string) (domain.CoreComponent, error) {
	s.mu.Lock()
	if _, ok := s.installing[id]; ok {
		s.mu.Unlock()
		return domain.CoreComponent{}, fmt.Errorf("%w: %w", repository.ErrInvalidData, ErrInstallInProgress)
	}
	s.mu.Unlock()

	comp, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.CoreComponent{}, err
	}

	installDir := strings.TrimSpace(comp.InstallDir)
	if installDir == "" {
		if info := s.detectInstalled(comp.Kind); info != nil {
			installDir = strings.TrimSpace(info.dir)
		}
	}

	if installDir != "" {
		installDir = filepath.Clean(installDir)

		var rootMatched string
		for _, root := range shared.ArtifactsSearchRoots() {
			root = filepath.Clean(strings.TrimSpace(root))
			if root == "" {
				continue
			}
			rel, relErr := filepath.Rel(root, installDir)
			if relErr != nil {
				continue
			}
			rel = filepath.Clean(rel)
			if rel == "." || rel == "" {
				continue
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			rootMatched = root
			break
		}

		if rootMatched == "" {
			return domain.CoreComponent{}, fmt.Errorf("%w: uninstall path is outside artifacts root", repository.ErrInvalidData)
		}

		if err := os.RemoveAll(installDir); err != nil {
			return domain.CoreComponent{}, err
		}
	}

	if err := s.repo.ClearInstalled(ctx, id); err != nil {
		return domain.CoreComponent{}, err
	}
	return s.repo.Get(ctx, id)
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
	var subdir string
	var binaries []string

	switch kind {
	case domain.ComponentSingBox:
		subdir = "core/sing-box"
		binaries = []string{"sing-box", "sing-box.exe"}
	case domain.ComponentClash:
		subdir = "core/clash"
		binaries = []string{"mihomo", "mihomo.exe", "clash", "clash.exe"}
	default:
		return nil
	}

	for _, root := range shared.ArtifactsSearchRoots() {
		dir := filepath.Join(root, subdir)
		binaryPath, err := shared.FindBinaryInDir(dir, binaries)
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
	case domain.ComponentSingBox:
		kindStr = "singbox"
	case domain.ComponentClash:
		kindStr = "clash"
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

	// mihomo 的发布包在 Linux/macOS 通常是单文件 gzip，文件名可能带版本/平台后缀；这里把解压产物规整为固定可执行文件名。
	if comp.Kind == domain.ComponentClash {
		if err := normalizeClashInstall(installDir); err != nil {
			s.repo.UpdateInstallStatus(ctx, id, domain.InstallStatusError, 0, "安装后处理失败: "+err.Error())
			return
		}
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

	// 确保 clash(mihomo) 组件存在
	if _, err := s.repo.GetByKind(ctx, domain.ComponentClash); err != nil {
		if !errors.Is(err, repository.ErrComponentNotFound) {
			return err
		}
		if _, err := s.repo.Create(ctx, domain.CoreComponent{
			Name: "clash",
			Kind: domain.ComponentClash,
			Meta: map[string]string{
				"repo": "MetaCubeX/mihomo",
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
	case domain.ComponentSingBox:
		return filepath.Join(base, "sing-box")
	case domain.ComponentClash:
		return filepath.Join(base, "clash")
	default:
		return filepath.Join(base, comp.Name)
	}
}

// setExecutablePermissions 设置二进制文件的执行权限
func (s *Service) setExecutablePermissions(dir, kind string) {
	var binaries []string
	switch kind {
	case "singbox":
		binaries = []string{"sing-box", "sing-box.exe"}
	case "clash":
		binaries = []string{"mihomo", "mihomo.exe", "clash", "clash.exe"}
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

func normalizeClashInstall(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("install dir is empty")
	}

	targetName := "mihomo"
	if runtime.GOOS == "windows" {
		targetName = "mihomo.exe"
	}
	targetPath := filepath.Join(dir, targetName)
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}

	// 兜底：zip 解压后可能带版本/平台后缀，这里做一次 best-effort 归一化，确保后续能找到二进制。
	isCandidate := func(name string) bool {
		lower := strings.ToLower(strings.TrimSpace(name))
		if lower == "" {
			return false
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(lower, ".exe") {
			return false
		}
		return strings.HasPrefix(lower, "mihomo") || strings.HasPrefix(lower, "clash")
	}

	var tryRenameErr error
	tryRename := func(path string) bool {
		if path == "" {
			return false
		}

		if err := os.Rename(path, targetPath); err != nil {
			tryRenameErr = fmt.Errorf("rename %s -> %s: %w", path, targetPath, err)
			return false
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(targetPath, 0o755); err != nil {
				tryRenameErr = fmt.Errorf("chmod %s: %w", targetPath, err)
				// best-effort rollback to avoid leaving an unusable binary at targetPath
				_ = os.Rename(targetPath, path)
				return false
			}
		}
		return true
	}

	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if isCandidate(entry.Name()) {
				if tryRename(filepath.Join(dir, entry.Name())) {
					return nil
				}
			}
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sub := filepath.Join(dir, entry.Name())
			subEntries, subErr := os.ReadDir(sub)
			if subErr != nil {
				continue
			}
			for _, subEntry := range subEntries {
				if subEntry.IsDir() {
					continue
				}
				if isCandidate(subEntry.Name()) {
					if tryRename(filepath.Join(sub, subEntry.Name())) {
						return nil
					}
				}
			}
		}
	}

	if tryRenameErr != nil {
		return fmt.Errorf("failed to normalize clash binary in %s: %w", dir, tryRenameErr)
	}
	return fmt.Errorf("clash binary not found after extraction in %s", dir)
}
