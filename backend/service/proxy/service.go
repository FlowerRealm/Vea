package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service/adapters"
	"vea/backend/service/nodegroup"
	"vea/backend/service/shared"
)

// 错误定义
var (
	ErrEngineNotInstalled = errors.New("engine not installed")
	ErrProxyNotRunning    = errors.New("proxy not running")
)

// Service 代理服务
type Service struct {
	frouters   repository.FRouterRepository
	nodes      repository.NodeRepository
	components repository.ComponentRepository
	settings   repository.SettingsRepository

	adapters map[domain.CoreEngineKind]adapters.CoreAdapter

	mu         sync.Mutex
	mainHandle *adapters.ProcessHandle
	mainEngine domain.CoreEngineKind
	activeCfg  domain.ProxyConfig

	kernelLogPath      string
	kernelLogSession   uint64
	kernelLogEngine    domain.CoreEngineKind
	kernelLogStartedAt time.Time
}

// NewService 创建代理服务
func NewService(
	frouters repository.FRouterRepository,
	nodes repository.NodeRepository,
	components repository.ComponentRepository,
	settings repository.SettingsRepository,
) *Service {
	return &Service{
		frouters:   frouters,
		nodes:      nodes,
		components: components,
		settings:   settings,
		adapters: map[domain.CoreEngineKind]adapters.CoreAdapter{
			domain.EngineXray:    &adapters.XrayAdapter{},
			domain.EngineSingBox: &adapters.SingBoxAdapter{},
		},
	}
}

// ========== 代理控制 ==========

// Start 启动代理
func (s *Service) Start(ctx context.Context, cfg domain.ProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previousTunInterface := ""
	if s.activeCfg.InboundMode == domain.InboundTUN {
		previousTunInterface = "tun0"
		if s.activeCfg.TUNSettings != nil && s.activeCfg.TUNSettings.InterfaceName != "" {
			previousTunInterface = s.activeCfg.TUNSettings.InterfaceName
		}
	}
	previousConfigPath := ""
	if s.mainHandle != nil {
		previousConfigPath = strings.TrimSpace(s.mainHandle.ConfigPath)
	}

	// 停止现有代理
	s.stopLocked()
	if previousTunInterface != "" {
		// 避免重启过快导致 tun 设备尚未释放就再次创建，触发 "device or resource busy"。
		if err := s.waitForTUNAbsent(previousTunInterface, 10*time.Second); err != nil {
			// 处理历史遗留：曾经用 pkexec 启动过 sing-box 时，后端无法以普通用户态 stop root 进程，
			// 可能留下孤儿 sing-box 占用 tun 设备。这里做一次 best-effort 清理。
			s.killProcessesUsingConfig(previousConfigPath)
			if err2 := s.waitForTUNAbsent(previousTunInterface, 10*time.Second); err2 != nil {
				return fmt.Errorf("previous TUN is still busy: %w", err2)
			}
		}
	}
	cfg = s.applyConfigDefaults(cfg)

	// 获取 FRouter 与链式代理设置
	frouter, err := s.resolveFRouter(ctx, cfg)
	if err != nil {
		return err
	}

	nodes := []domain.Node(nil)
	if s.nodes != nil {
		nodes, _ = s.nodes.List(ctx)
	}

	// 选择引擎
	engine, err := s.selectEngine(ctx, cfg, frouter, nodes)
	if err != nil {
		return err
	}

	// 获取适配器
	adapter := s.adapters[engine]
	if adapter == nil {
		return fmt.Errorf("adapter not found for engine: %s", engine)
	}

	// 构建配置
	geo := s.prepareGeoFiles(engine)

	plan, err := nodegroup.CompileProxyPlan(engine, cfg, frouter, nodes)
	if err != nil {
		return fmt.Errorf("compile frouter: %w", err)
	}
	configBytes, err := adapter.BuildConfig(plan, geo)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	// sing-box 的 route.rule_set 依赖本地 .srs 文件；缺失时会在运行期直接 FATAL。
	if engine == domain.EngineSingBox {
		tags, err := shared.ExtractSingBoxRuleSetTagsFromConfig(configBytes)
		if err != nil {
			return fmt.Errorf("extract sing-box rule-set: %w", err)
		}
		if err := shared.EnsureSingBoxRuleSets(tags); err != nil {
			return fmt.Errorf("ensure sing-box rule-set: %w", err)
		}
	}

	// 获取引擎二进制路径
	binaryPath, err := s.getEngineBinary(ctx, engine, adapter)
	if err != nil {
		return err
	}

	// 写入配置文件
	configDir := engineConfigDir(engine)
	configPath := filepath.Join(configDir, "config.json")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// 写入可读诊断信息（不影响启动流程）
	explainPath := filepath.Join(configDir, "config.explain.txt")
	_ = os.WriteFile(explainPath, []byte(plan.Explain()), 0600)

	// 启动进程
	if err := s.startProcess(adapter, engine, binaryPath, configPath, cfg); err != nil {
		return err
	}

	if s.settings != nil {
		if stored, err := s.settings.UpdateProxyConfig(ctx, cfg); err == nil {
			cfg = stored
		} else {
			return fmt.Errorf("save proxy config: %w", err)
		}
	}

	s.activeCfg = cfg
	return nil
}

// Stop 停止代理
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopLocked()
	return nil
}

// Status 获取代理状态
func (s *Service) Status(ctx context.Context) map[string]interface{} {
	if !s.mu.TryLock() {
		return map[string]interface{}{
			"running": false,
			"busy":    true,
		}
	}
	defer s.mu.Unlock()

	running := s.mainHandle != nil && s.mainHandle.Cmd != nil && s.mainHandle.Cmd.Process != nil
	status := map[string]interface{}{
		"running": running,
	}

	if running {
		status["pid"] = s.mainHandle.Cmd.Process.Pid
		if s.mainEngine != "" {
			status["engine"] = string(s.mainEngine)
		}
	}

	if s.activeCfg.FRouterID != "" {
		status["frouterId"] = s.activeCfg.FRouterID
	}
	if s.activeCfg.InboundMode != "" {
		status["inboundMode"] = string(s.activeCfg.InboundMode)
	}
	if s.activeCfg.InboundPort > 0 {
		status["inboundPort"] = s.activeCfg.InboundPort
	}

	return status
}

// ========== 内部方法 ==========

func (s *Service) stopLocked() {
	if s.mainHandle == nil {
		return
	}

	handle := s.mainHandle
	done := handle.Done
	adapter := s.adapters[s.mainEngine]
	if adapter != nil {
		_ = adapter.Stop(handle)
	} else if handle.Cmd != nil && handle.Cmd.Process != nil {
		_ = handle.Cmd.Process.Kill()
		if done != nil {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
	}

	s.mainHandle = nil
	s.mainEngine = ""
}

func (s *Service) resolveFRouter(ctx context.Context, cfg domain.ProxyConfig) (domain.FRouter, error) {
	if s.frouters == nil {
		return domain.FRouter{}, errors.New("frouter repository not configured")
	}
	if cfg.FRouterID == "" {
		return domain.FRouter{}, fmt.Errorf("%w: proxyConfig.frouterId is required", repository.ErrInvalidData)
	}
	return s.frouters.Get(ctx, cfg.FRouterID)
}

func (s *Service) applyConfigDefaults(cfg domain.ProxyConfig) domain.ProxyConfig {
	ctx := context.Background()

	if cfg.InboundMode == "" {
		cfg.InboundMode = domain.InboundMixed
	}
	if cfg.InboundPort == 0 && cfg.InboundMode != domain.InboundTUN {
		cfg.InboundPort = 1080
	}

	// 内核日志统一开到 debug（按用户需求：看完整日志，而不是“只有启动几行”）。
	if cfg.LogConfig == nil {
		cfg.LogConfig = &domain.LogConfiguration{Timestamp: true}
	}
	cfg.LogConfig.Level = "debug"

	// TUN 模式强制 sing-box
	if cfg.InboundMode == domain.InboundTUN {
		cfg.PreferredEngine = domain.EngineSingBox
		if cfg.TUNSettings == nil {
			cfg.TUNSettings = &domain.TUNConfiguration{
				InterfaceName: "tun0",
				MTU:           9000,
				Address:       []string{"172.19.0.1/30"},
				AutoRoute:     true,
				StrictRoute:   true,
				Stack:         "mixed",
				DNSHijack:     true,
			}
		}
		if cfg.TUNSettings.InterfaceName == "" {
			cfg.TUNSettings.InterfaceName = "tun0"
		}
		if cfg.TUNSettings.MTU <= 0 {
			cfg.TUNSettings.MTU = 9000
		}
		if len(cfg.TUNSettings.Address) == 0 {
			cfg.TUNSettings.Address = []string{"172.19.0.1/30"}
		}
		if cfg.TUNSettings.Stack == "" {
			cfg.TUNSettings.Stack = "mixed"
		}
	}

	// 设置智能默认引擎
	if cfg.PreferredEngine == "" {
		cfg.PreferredEngine = s.GetEffectiveDefaultEngine(ctx)
	}

	return cfg
}

func (s *Service) selectEngine(ctx context.Context, cfg domain.ProxyConfig, frouter domain.FRouter, nodes []domain.Node) (domain.CoreEngineKind, error) {
	// TUN 模式强制 sing-box
	if cfg.InboundMode == domain.InboundTUN {
		return domain.EngineSingBox, nil
	}

	engine, _, err := selectEngineForFRouter(ctx, frouter, nodes, cfg.PreferredEngine, s.components, s.settings, s.adapters)
	return engine, err
}

func (s *Service) getEngineBinary(ctx context.Context, engine domain.CoreEngineKind, adapter adapters.CoreAdapter) (string, error) {
	var kind domain.CoreComponentKind

	switch engine {
	case domain.EngineXray:
		kind = domain.ComponentXray
	case domain.EngineSingBox:
		kind = domain.ComponentSingBox
	default:
		return "", fmt.Errorf("unknown engine: %s", engine)
	}

	comp, err := s.components.GetByKind(ctx, kind)
	if err != nil {
		return "", ErrEngineNotInstalled
	}

	if comp.InstallDir == "" {
		return "", ErrEngineNotInstalled
	}

	candidates := []string(nil)
	if adapter != nil {
		candidates = adapter.BinaryNames()
	}
	if len(candidates) == 0 {
		switch engine {
		case domain.EngineXray:
			candidates = []string{"xray", "xray.exe"}
		case domain.EngineSingBox:
			candidates = []string{"sing-box", "sing-box.exe"}
		}
	}

	path, err := shared.FindBinaryInDir(comp.InstallDir, candidates)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEngineNotInstalled, err)
	}
	return path, nil
}

func (s *Service) prepareGeoFiles(engine domain.CoreEngineKind) adapters.GeoFiles {
	geoDir := filepath.Join(shared.ArtifactsRoot, shared.GeoDir)
	return adapters.GeoFiles{
		GeoIP:        filepath.Join(geoDir, "geoip.dat"),
		GeoSite:      filepath.Join(geoDir, "geosite.dat"),
		ArtifactsDir: shared.ArtifactsRoot,
	}
}

func (s *Service) startProcess(adapter adapters.CoreAdapter, engine domain.CoreEngineKind, binaryPath, configPath string, cfg domain.ProxyConfig) error {
	if adapter == nil {
		return errors.New("adapter is nil")
	}

	procCfg := adapters.ProcessConfig{
		BinaryPath: binaryPath,
		ConfigDir:  filepath.Dir(configPath),
	}

	if adapter.RequiresPrivileges(cfg) {
		// 约定：TUN 在 Linux 下使用 setcap（cap_net_admin 等）来实现“仅首次配置需要提权”。
		// 每次用 pkexec 启动内核会带来两个严重问题：
		// 1) 后端（普通用户态）无法可靠 stop root 进程，容易留下孤儿 sing-box，占用 tun 设备；
		// 2) 重启/更新配置时会频繁弹出提权框，UX 很差。
		configured, err := shared.CheckTUNCapabilities()
		if err != nil {
			return fmt.Errorf("check TUN capabilities: %w", err)
		}
		if !configured && runtime.GOOS == "linux" {
			_, err := shared.EnsureTUNCapabilities()
			if err != nil {
				return fmt.Errorf("configure TUN capabilities: %w", err)
			}
			configured, err = shared.CheckTUNCapabilities()
			if err != nil {
				return fmt.Errorf("check TUN capabilities: %w", err)
			}
		}
		if !configured {
			switch runtime.GOOS {
			case "linux":
				return fmt.Errorf("TUN 模式未配置：请运行 `sudo ./vea setup-tun` 或调用 POST /tun/setup")
			case "darwin":
				return fmt.Errorf("TUN 模式需要 root 权限：请使用 sudo 运行 vea")
			case "windows":
				return fmt.Errorf("TUN 模式需要管理员权限：请以管理员身份运行 Vea")
			default:
				return fmt.Errorf("TUN 模式需要特权权限，但当前未配置")
			}
		}
	}

	// Linux + sing-box(TUN): sing-box 会调用 resolvectl 配置 systemd-resolved，触发多次 polkit 授权。
	// 做法：用 PATH shim 拦截 resolvectl，转发到同一生命周期内的 root helper（只需授权一次）。
	if runtime.GOOS == "linux" && engine == domain.EngineSingBox && cfg.InboundMode == domain.InboundTUN {
		shimDir, err := shared.EnsureResolvectlShim()
		if err != nil {
			log.Printf("[TUN] ensure resolvectl shim failed: %v", err)
		} else if shimDir != "" {
			socketPath := shared.ResolvectlHelperSocketPath()
			exePath, err := os.Executable()
			if err == nil {
				if realPath, err := filepath.EvalSymlinks(exePath); err == nil && realPath != "" {
					exePath = realPath
				}
				if abs, err := filepath.Abs(exePath); err == nil && abs != "" {
					exePath = abs
				}
			}

			procCfg.Environment = append(procCfg.Environment,
				shared.EnvVeaExecutable+"="+exePath,
				shared.EnvResolvectlSocket+"="+socketPath,
				fmt.Sprintf("%s=%d", shared.EnvResolvectlUID, os.Getuid()),
				fmt.Sprintf("%s=%d", shared.EnvResolvectlHelperParent, os.Getpid()),
			)

			pathValue := os.Getenv("PATH")
			if pathValue != "" {
				pathValue = shimDir + string(os.PathListSeparator) + pathValue
			} else {
				pathValue = shimDir
			}
			procCfg.Environment = append(procCfg.Environment, "PATH="+pathValue)
		}
	}

	logPath := kernelLogPathForConfigDir(procCfg.ConfigDir)
	var logFile *os.File
	if logPath != "" {
		openTrunc := func(path string) (*os.File, error) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		}

		f, err := openTrunc(logPath)
		if err != nil {
			log.Printf("[KernelLog] 打开日志文件失败（%s）: %v", logPath, err)

			// 回退：configDir 可能在某些发行方式下不可写（比如历史 sudo runs / 打包资源目录）。
			// runtime 目录由 ArtifactsRoot 统一保证可写。
			fallback := filepath.Join(shared.ArtifactsRoot, "runtime", "kernel.log")
			if f2, err2 := openTrunc(fallback); err2 == nil {
				log.Printf("[KernelLog] 使用回退日志文件: %s", fallback)
				logPath = fallback
				f = f2
			} else {
				log.Printf("[KernelLog] 打开回退日志文件失败（%s）: %v", fallback, err2)
			}
		}

		if f != nil {
			logFile = f
			_, _ = fmt.Fprintf(logFile, "----- kernel start %s engine=%s -----\n", time.Now().Format(time.RFC3339Nano), engine)
			procCfg.Stdout = newFanoutWriter(os.Stdout, logFile)
			procCfg.Stderr = newFanoutWriter(os.Stderr, logFile)
		}
	}

	handle, err := adapter.Start(procCfg, configPath)
	if err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return fmt.Errorf("failed to start process: %w", err)
	}
	handle.LogCloser = logFile

	done := make(chan struct{})
	handle.Done = done

	s.mainHandle = handle
	s.mainEngine = engine
	s.kernelLogPath = logPath
	s.kernelLogEngine = engine
	s.kernelLogStartedAt = handle.StartedAt
	s.kernelLogSession++

	go s.monitorProcess(handle, done)

	// 非 TUN：等端口监听就绪，提前失败而不是“启动成功但不可用”。
	if cfg.InboundMode != domain.InboundTUN && cfg.InboundPort > 0 {
		handle.Port = cfg.InboundPort
		if err := adapter.WaitForReady(handle, 5*time.Second); err != nil {
			_ = adapter.Stop(handle)
			s.mainHandle = nil
			s.mainEngine = ""
			return fmt.Errorf("process not ready: %w", err)
		}
	}

	if cfg.InboundMode == domain.InboundTUN {
		interfaceName := "tun0"
		if cfg.TUNSettings != nil && cfg.TUNSettings.InterfaceName != "" {
			interfaceName = cfg.TUNSettings.InterfaceName
		}

		if err := s.waitForTUNReady(interfaceName, 10*time.Second); err != nil {
			// TUN 接口没起来就宣告成功是误导；直接失败并回收进程。
			_ = adapter.Stop(handle)
			s.mainHandle = nil
			s.mainEngine = ""
			return fmt.Errorf("TUN interface not ready: %w", err)
		}
	}

	return nil
}

// waitForTUNReady 等待 TUN 接口就绪
func (s *Service) waitForTUNReady(interfaceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				if iface.Name == interfaceName {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("TUN interface %s not ready after %v", interfaceName, timeout)
}

// waitForTUNAbsent 等待 TUN 接口释放（主要用于快速重启时避免 EBUSY）
func (s *Service) waitForTUNAbsent(interfaceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		found := false
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				if iface.Name == interfaceName {
					found = true
					break
				}
			}
		}
		if !found {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("TUN interface %s still exists after %v", interfaceName, timeout)
}

func (s *Service) killProcessesUsingConfig(configPath string) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return
	}

	pattern := regexp.QuoteMeta(configPath)

	pkillPath, err := exec.LookPath("pkill")
	if err == nil {
		if err := exec.Command(pkillPath, "-f", pattern).Run(); err == nil {
			return
		}
	}

	if runtime.GOOS != "linux" {
		return
	}

	pkexecPath, err := exec.LookPath("pkexec")
	if err != nil {
		return
	}
	if pkillPath == "" {
		if p, err := exec.LookPath("pkill"); err == nil {
			pkillPath = p
		} else {
			return
		}
	}

	if pgrepPath, err := exec.LookPath("pgrep"); err == nil {
		// 如果完全没有匹配的进程，pkexec 也只会白白弹一次授权框。
		if err := exec.Command(pgrepPath, "-f", pattern).Run(); err != nil {
			return
		}
	}

	_ = exec.Command(pkexecPath, pkillPath, "-f", pattern).Run()
}

func (s *Service) monitorProcess(handle *adapters.ProcessHandle, done chan struct{}) {
	if handle == nil || handle.Cmd == nil {
		if done != nil {
			close(done)
		}
		return
	}

	_ = handle.Cmd.Wait()
	if handle.LogCloser != nil {
		_ = handle.LogCloser.Close()
	}

	// 先释放 Done，避免 stopLocked 在持有 s.mu 时等待造成死锁。
	if done != nil {
		close(done)
	}

	s.mu.Lock()
	if s.mainHandle == handle {
		s.mainHandle = nil
		s.mainEngine = ""
	}
	s.mu.Unlock()
}

// ========== 引擎推荐 ==========

// EngineRecommendation 引擎推荐结果
type EngineRecommendation struct {
	RecommendedEngine domain.CoreEngineKind `json:"recommendedEngine"`
	Reason            string                `json:"reason"`
	XrayCompatible    int                   `json:"xrayCompatible"`
	SingBoxOnly       int                   `json:"singBoxOnly"`
	TotalNodes        int                   `json:"totalNodes"`
}

// EngineStatus 引擎状态
type EngineStatus struct {
	XrayInstalled    bool                  `json:"xrayInstalled"`
	SingBoxInstalled bool                  `json:"singboxInstalled"`
	DefaultEngine    domain.CoreEngineKind `json:"defaultEngine"`
	Recommendation   EngineRecommendation  `json:"recommendation"`
}

// RecommendEngine 根据现有节点智能推荐引擎
// 规则：
// 1. 无节点 → 默认 Xray（更成熟稳定）
// 2. 存在 Hysteria2/TUIC 节点 → 推荐 sing-box
// 3. 所有节点支持 Xray → 推荐 Xray
// 4. 混合场景 → 推荐 sing-box（支持更广）
func (s *Service) RecommendEngine(ctx context.Context) EngineRecommendation {
	nodes := []domain.Node(nil)
	if s.nodes != nil {
		nodes, _ = s.nodes.List(ctx)
	}
	return recommendEngineForNodes(nodes, s.adapters)
}

// GetEffectiveDefaultEngine 获取有效的默认引擎
// 优先级：全局设置 > 智能推荐
func (s *Service) GetEffectiveDefaultEngine(ctx context.Context) domain.CoreEngineKind {
	settings, _ := s.settings.GetFrontend(ctx)

	// 从设置中获取默认引擎
	if defaultEngine, ok := settings["engine.defaultEngine"].(string); ok {
		engine := domain.CoreEngineKind(defaultEngine)
		if engine != "" && engine != domain.EngineAuto {
			return engine
		}
	}

	// 使用智能推荐
	return s.RecommendEngine(ctx).RecommendedEngine
}

// GetEngineStatus 获取引擎状态
func (s *Service) GetEngineStatus(ctx context.Context) EngineStatus {
	status := EngineStatus{
		Recommendation: s.RecommendEngine(ctx),
		DefaultEngine:  s.GetEffectiveDefaultEngine(ctx),
	}

	// 检查引擎是否真正安装（有安装目录且有安装时间）
	if comp, err := s.components.GetByKind(ctx, domain.ComponentXray); err == nil {
		if comp.InstallDir != "" && comp.LastInstalledAt.Unix() > 0 {
			status.XrayInstalled = true
		}
	}
	if comp, err := s.components.GetByKind(ctx, domain.ComponentSingBox); err == nil {
		if comp.InstallDir != "" && comp.LastInstalledAt.Unix() > 0 {
			status.SingBoxInstalled = true
		}
	}

	return status
}
