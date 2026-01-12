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

const (
	defaultTunInterfaceName = "tun0"
	defaultTunMTU           = 9000
	defaultTunAddress       = "172.19.0.1/30"
	defaultTunStack         = "mixed"
)

// 错误定义
var (
	ErrEngineNotInstalled = errors.New("engine not installed")
	ErrProxyNotRunning    = errors.New("proxy not running")
)

type EngineNotInstalledError struct {
	Engine domain.CoreEngineKind
	Cause  error
}

func (e *EngineNotInstalledError) Error() string {
	if e == nil {
		return ErrEngineNotInstalled.Error()
	}
	if e.Engine == "" {
		if e.Cause != nil {
			return fmt.Sprintf("%s: %v", ErrEngineNotInstalled.Error(), e.Cause)
		}
		return ErrEngineNotInstalled.Error()
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s (%s): %v", ErrEngineNotInstalled.Error(), e.Engine, e.Cause)
	}
	return fmt.Sprintf("%s (%s)", ErrEngineNotInstalled.Error(), e.Engine)
}

func (e *EngineNotInstalledError) Unwrap() error { return ErrEngineNotInstalled }

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
	tunIface   string

	lastRestartAt    time.Time
	lastRestartError string
	userStopped      bool
	userStoppedAt    time.Time

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
			domain.EngineSingBox: &adapters.SingBoxAdapter{},
			domain.EngineClash:   &adapters.ClashAdapter{},
		},
	}
}

// ========== 代理控制 ==========

// Start 启动代理
func (s *Service) Start(ctx context.Context, cfg domain.ProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.userStopped = false
	s.userStoppedAt = time.Time{}

	hadPrevious := s.mainHandle != nil
	previousCfg := s.activeCfg
	previousEngine := s.mainEngine
	previousAdapter := s.adapters[previousEngine]
	previousConfigPath := ""
	previousBinaryPath := ""
	if s.mainHandle != nil {
		previousConfigPath = strings.TrimSpace(s.mainHandle.ConfigPath)
		previousBinaryPath = strings.TrimSpace(s.mainHandle.BinaryPath)
	}
	previousConfigBytes := []byte(nil)
	if previousConfigPath != "" {
		if b, err := os.ReadFile(previousConfigPath); err == nil {
			previousConfigBytes = b
		}
	}
	previousTunInterface := ""
	if previousCfg.InboundMode == domain.InboundTUN {
		previousTunInterface = strings.TrimSpace(s.tunIface)
		if previousTunInterface == "" {
			previousTunInterface = defaultTunInterfaceName
			if previousCfg.TUNSettings != nil && previousCfg.TUNSettings.InterfaceName != "" {
				previousTunInterface = previousCfg.TUNSettings.InterfaceName
			}
		}
	}

	rollback := func(cause error) error {
		if !hadPrevious {
			return cause
		}
		if previousEngine == "" {
			return errors.Join(cause, errors.New("rollback: previous engine is empty"))
		}
		if previousAdapter == nil {
			return errors.Join(cause, fmt.Errorf("rollback: adapter not found for engine: %s", previousEngine))
		}
		if previousConfigPath == "" {
			return errors.Join(cause, errors.New("rollback: previous config path is empty"))
		}

		if len(previousConfigBytes) > 0 {
			_ = os.MkdirAll(filepath.Dir(previousConfigPath), 0o755)
			if err := os.WriteFile(previousConfigPath, previousConfigBytes, 0o600); err != nil {
				return errors.Join(cause, fmt.Errorf("rollback: restore previous config: %w", err))
			}
		}

		binaryPath := previousBinaryPath
		if binaryPath == "" {
			p, err := s.getEngineBinary(ctx, previousEngine, previousAdapter)
			if err != nil {
				return errors.Join(cause, fmt.Errorf("rollback: resolve previous binary: %w", err))
			}
			binaryPath = p
		}

		if err := s.startProcess(previousAdapter, previousEngine, binaryPath, previousConfigPath, previousCfg); err != nil {
			return errors.Join(cause, fmt.Errorf("rollback: restart previous proxy: %w", err))
		}

		s.activeCfg = previousCfg
		log.Printf("[Proxy] 启动失败，已回滚到上一次可用配置: %v", cause)
		return cause
	}

	cfg = s.applyConfigDefaults(ctx, cfg)

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

	// 基于“实际选中引擎”修正配置默认值（避免不同内核的默认假设相互污染）。
	//
	// 典型问题：前端/后端默认 MTU=9000（偏 sing-box），但 Linux + mihomo 的 TUN 在部分网络环境下
	// 会因为 PMTU/分片兼容性表现为“看起来全网断开”。主流 mihomo GUI 在 Linux 上更偏向默认 1500。
	if tunedCfg, changed := tuneTUNSettingsForEngine(engine, cfg); changed {
		cfg = tunedCfg
		log.Printf("[TUN] 已按引擎(%s)调整默认 TUN 配置（如需自定义请在设置中修改）", engine)
	}

	// 直到这里都不应该停掉现有代理；避免“新配置编译失败就把用户网络断掉”这种灾难。

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

	// 停止现有代理（现在新配置已经准备好，失败概率大幅降低）。
	s.stopLocked()
	if runtime.GOOS == "linux" && hadPrevious && previousTunInterface != "" {
		// 避免重启过快导致 tun 设备尚未释放就再次创建，触发 "device or resource busy"。
		if err := s.waitForTUNAbsent(previousTunInterface, 10*time.Second); err != nil {
			// 处理历史遗留：曾经用 pkexec 启动过 sing-box 时，后端无法以普通用户态 stop root 进程，
			// 可能留下孤儿 sing-box 占用 tun 设备。这里做一次 best-effort 清理。
			s.killProcessesUsingConfig(previousConfigPath)
			if err2 := s.waitForTUNAbsent(previousTunInterface, 10*time.Second); err2 != nil {
				return rollback(fmt.Errorf("previous TUN is still busy: %w", err2))
			}
		}
	}

	// 写入配置文件
	configDir := engineConfigDir(engine)
	configName := "config.json"
	if engine == domain.EngineClash {
		configName = "config.yaml"
	}
	configPath := filepath.Join(configDir, configName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return rollback(fmt.Errorf("failed to create config dir: %w", err))
	}
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		return rollback(fmt.Errorf("failed to write config: %w", err))
	}

	// 写入可读诊断信息（不影响启动流程）
	explainPath := filepath.Join(configDir, "config.explain.txt")
	_ = os.WriteFile(explainPath, []byte(plan.Explain()), 0600)

	// mihomo/clash 使用 GeoSite.dat/GeoIP.dat（大小写敏感）；这里把已有的 geo 资源同步一份过去，
	// 避免每次启动都触发“找不到 GeoSite.dat -> 在线下载”的行为。
	if engine == domain.EngineClash {
		if err := ensureClashGeoData(configDir); err != nil {
			log.Printf("[Clash] ensure geo data failed: %v", err)
		}
	}

	// 启动进程
	if err := s.startProcess(adapter, engine, binaryPath, configPath, cfg); err != nil {
		return rollback(err)
	}

	if s.settings != nil {
		if stored, err := s.settings.UpdateProxyConfig(ctx, cfg); err == nil {
			cfg = stored
		} else {
			return fmt.Errorf("save proxy config: %w", err)
		}
	}

	s.activeCfg = cfg
	s.lastRestartError = ""
	return nil
}

// Stop 停止代理
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopLocked()
	return nil
}

// StopUser 停止代理（用户显式触发），用于让 keepalive 尊重“用户已停止”的状态。
func (s *Service) StopUser(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.userStopped = true
	s.userStoppedAt = time.Now()
	s.stopLocked()
	return nil
}

// MarkRestartScheduled 记录一次“重启已触发/计划中”（用于前端轮询提示）。
func (s *Service) MarkRestartScheduled() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastRestartAt = time.Now()
	s.lastRestartError = ""
}

// MarkRestartFailed 记录一次“重启失败”。
func (s *Service) MarkRestartFailed(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastRestartAt.IsZero() {
		s.lastRestartAt = time.Now()
	}
	s.lastRestartError = err.Error()
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
	if !s.lastRestartAt.IsZero() {
		status["lastRestartAt"] = s.lastRestartAt
	}
	if s.lastRestartError != "" {
		status["lastRestartError"] = s.lastRestartError
	}
	if s.userStopped {
		status["userStopped"] = true
		status["userStoppedAt"] = s.userStoppedAt
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
	s.tunIface = ""
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

func (s *Service) applyConfigDefaults(ctx context.Context, cfg domain.ProxyConfig) domain.ProxyConfig {
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

	// TUN 模式：补齐默认 TUNSettings；默认引擎保持 auto（优先选择支持 TUN 的已安装内核）。
	if cfg.InboundMode == domain.InboundTUN {
		if cfg.PreferredEngine == "" {
			cfg.PreferredEngine = domain.EngineAuto
		}
		if cfg.TUNSettings == nil {
			cfg.TUNSettings = &domain.TUNConfiguration{
				InterfaceName: defaultTunInterfaceName,
				MTU:           defaultTunMTU,
				Address:       []string{defaultTunAddress},
				AutoRoute:     true,
				StrictRoute:   true,
				Stack:         defaultTunStack,
				DNSHijack:     true,
			}
		}
		if cfg.TUNSettings.InterfaceName == "" {
			cfg.TUNSettings.InterfaceName = defaultTunInterfaceName
		}
		if cfg.TUNSettings.MTU <= 0 {
			cfg.TUNSettings.MTU = defaultTunMTU
		}
		if len(cfg.TUNSettings.Address) == 0 {
			cfg.TUNSettings.Address = []string{defaultTunAddress}
		}
		if cfg.TUNSettings.Stack == "" {
			cfg.TUNSettings.Stack = defaultTunStack
		}
	}

	// 设置智能默认引擎
	if cfg.PreferredEngine == "" {
		cfg.PreferredEngine = s.GetEffectiveDefaultEngine(ctx)
	}

	return cfg
}

func tuneTUNSettingsForEngine(engine domain.CoreEngineKind, cfg domain.ProxyConfig) (domain.ProxyConfig, bool) {
	if cfg.InboundMode != domain.InboundTUN || cfg.TUNSettings == nil {
		return cfg, false
	}

	// Linux + mihomo: 默认 MTU 使用 1500 更稳（避免某些网络/防火墙下 jumbo frame 兼容性问题）。
	// 注意：仅对“明显是 Vea 默认值”的情况做修正，避免意外覆盖用户显式配置。
	if runtime.GOOS == "linux" && engine == domain.EngineClash {
		if isLikelyDefaultTUNSettings(cfg.TUNSettings) {
			cfg.TUNSettings.MTU = 1500
			return cfg, true
		}
	}

	return cfg, false
}

func isLikelyDefaultTUNSettings(cfg *domain.TUNConfiguration) bool {
	if cfg == nil {
		return false
	}

	// 前端 schema 与后端 applyConfigDefaults 的默认值（偏 sing-box）：
	// - MTU=9000, Address=172.19.0.1/30, AutoRoute=true, StrictRoute=true, Stack=mixed, DNSHijack=true
	// 这里用“完整匹配默认组合”的方式判断是否可安全修正。
	if cfg.MTU != defaultTunMTU {
		return false
	}
	if cfg.InterfaceName != "" && cfg.InterfaceName != defaultTunInterfaceName {
		return false
	}
	if len(cfg.Address) != 1 || strings.TrimSpace(cfg.Address[0]) != defaultTunAddress {
		return false
	}
	if !cfg.AutoRoute || !cfg.StrictRoute || !cfg.DNSHijack {
		return false
	}
	if strings.TrimSpace(cfg.Stack) != "" && strings.TrimSpace(cfg.Stack) != defaultTunStack {
		return false
	}
	if cfg.AutoRedirect || cfg.EndpointIndependentNat || cfg.UDPTimeout != 0 {
		return false
	}
	if len(cfg.RouteAddress) != 0 || len(cfg.RouteExcludeAddress) != 0 {
		return false
	}

	return true
}

func (s *Service) selectEngine(ctx context.Context, cfg domain.ProxyConfig, frouter domain.FRouter, nodes []domain.Node) (domain.CoreEngineKind, error) {
	engine, _, err := selectEngineForFRouter(ctx, cfg.InboundMode, frouter, nodes, cfg.PreferredEngine, s.components, s.settings, s.adapters)
	return engine, err
}

func (s *Service) getEngineBinary(ctx context.Context, engine domain.CoreEngineKind, adapter adapters.CoreAdapter) (string, error) {
	var kind domain.CoreComponentKind

	switch engine {
	case domain.EngineSingBox:
		kind = domain.ComponentSingBox
	case domain.EngineClash:
		kind = domain.ComponentClash
	default:
		return "", fmt.Errorf("unknown engine: %s", engine)
	}

	comp, err := s.components.GetByKind(ctx, kind)
	if err != nil {
		return "", &EngineNotInstalledError{Engine: engine, Cause: err}
	}

	if comp.InstallDir == "" {
		return "", &EngineNotInstalledError{Engine: engine}
	}

	candidates := []string(nil)
	if adapter != nil {
		candidates = adapter.BinaryNames()
	}
	if len(candidates) == 0 {
		switch engine {
		case domain.EngineSingBox:
			candidates = []string{"sing-box", "sing-box.exe"}
		case domain.EngineClash:
			candidates = []string{"mihomo", "mihomo.exe", "clash", "clash.exe"}
		}
	}

	path, err := shared.FindBinaryInDir(comp.InstallDir, candidates)
	if err != nil {
		return "", &EngineNotInstalledError{Engine: engine, Cause: err}
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
		configured, err := shared.CheckTUNCapabilitiesForBinary(binaryPath)
		if err != nil {
			return fmt.Errorf("check TUN capabilities: %w", err)
		}
		if !configured && runtime.GOOS == "linux" {
			_, err := shared.EnsureTUNCapabilitiesForBinary(binaryPath)
			if err != nil {
				return fmt.Errorf("configure TUN capabilities: %w", err)
			}
			configured, err = shared.CheckTUNCapabilitiesForBinary(binaryPath)
			if err != nil {
				return fmt.Errorf("check TUN capabilities: %w", err)
			}
		}
		if !configured {
			switch runtime.GOOS {
			case "linux":
				return fmt.Errorf("TUN 模式未配置：请运行 `sudo ./vea setup-tun`（默认 sing-box）或 `sudo ./vea setup-tun --binary <内核路径>`")
			case "darwin":
				return fmt.Errorf("TUN 模式需要 root 权限：请使用 sudo 运行 vea")
			case "windows":
				return fmt.Errorf("TUN 模式需要管理员权限：请以管理员身份运行 Vea")
			default:
				return fmt.Errorf("TUN 模式需要特权权限，但当前未配置")
			}
		}
	}

	// Linux + TUN(sing-box/mihomo): 内核可能会调用 resolvectl 配置 systemd-resolved，触发多次 polkit 授权。
	// 做法：用 PATH shim 拦截 resolvectl，转发到同一生命周期内的 root helper（只需授权一次）。
	if runtime.GOOS == "linux" && cfg.InboundMode == domain.InboundTUN && (engine == domain.EngineSingBox || engine == domain.EngineClash) {
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

	existingIfaces := map[string]int(nil)
	if cfg.InboundMode == domain.InboundTUN {
		existingIfaces = snapshotInterfaceIndices()
		// 清理可能与 TUN 冲突的遗留 iptables 规则（例如 XRAY/XRAY_SELF 链）。
		shared.CleanConflictingIPTablesRules()
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
	if cfg.InboundMode != domain.InboundTUN {
		if err := s.waitForInboundPortReady(adapter, handle, cfg.InboundPort, 5*time.Second, false); err != nil {
			return err
		}
	}

	if cfg.InboundMode == domain.InboundTUN {
		if engine == domain.EngineSingBox || engine == domain.EngineClash {
			interfaceName := defaultTunInterfaceName
			if cfg.TUNSettings != nil && cfg.TUNSettings.InterfaceName != "" {
				interfaceName = cfg.TUNSettings.InterfaceName
			}

			prevIndex := 0
			if existingIfaces != nil {
				if idx, ok := existingIfaces[interfaceName]; ok {
					prevIndex = idx
				}
			}

			tunIface := ""
			if runtime.GOOS == "linux" {
				if err := s.waitForTUNReadyWithIndex(interfaceName, prevIndex, 10*time.Second, done); err != nil {
					// TUN 接口没起来就宣告成功是误导；直接失败并回收进程。
					_ = adapter.Stop(handle)
					s.mainHandle = nil
					s.mainEngine = ""
					return fmt.Errorf("TUN interface not ready: %w", err)
				}
				tunIface = interfaceName
			} else {
				desiredName := strings.TrimSpace(interfaceName)
				// Windows/macOS 下默认不强制依赖 "tun0"；由内核自动选择实际名称。
				if desiredName == defaultTunInterfaceName {
					desiredName = ""
				}
				expectedAddrs := expectedTUNAddressesForEngine(engine, cfg)
				ifaceName, err := s.waitForTUNReadyByAddress(existingIfaces, desiredName, expectedAddrs, 10*time.Second, done)
				if err != nil {
					// TUN 接口没起来就宣告成功是误导；直接失败并回收进程。
					_ = adapter.Stop(handle)
					s.mainHandle = nil
					s.mainEngine = ""
					return fmt.Errorf("TUN interface not ready: %w", err)
				}
				tunIface = ifaceName
			}
			s.tunIface = tunIface

			// TUN 也可能同时开启本地代理端口（HTTP/SOCKS），这里同样做 readiness probe。
			if err := s.waitForInboundPortReady(adapter, handle, cfg.InboundPort, 5*time.Second, true); err != nil {
				return err
			}
			return nil
		}

		ifaceName, err := s.waitForNewTUNReady(existingIfaces, 10*time.Second, done)
		if err != nil {
			// TUN 接口没起来就宣告成功是误导；直接失败并回收进程。
			_ = adapter.Stop(handle)
			s.mainHandle = nil
			s.mainEngine = ""
			return fmt.Errorf("TUN interface not ready: %w", err)
		}
		s.tunIface = ifaceName

		// TUN 也可能同时开启本地代理端口（HTTP/SOCKS），这里同样做 readiness probe。
		if err := s.waitForInboundPortReady(adapter, handle, cfg.InboundPort, 5*time.Second, true); err != nil {
			return err
		}
		return nil
	}

	return nil
}

func (s *Service) waitForInboundPortReady(adapter adapters.CoreAdapter, handle *adapters.ProcessHandle, port int, timeout time.Duration, clearTunIface bool) error {
	if port <= 0 {
		return nil
	}

	handle.Port = port
	if err := adapter.WaitForReady(handle, timeout); err != nil {
		_ = adapter.Stop(handle)
		s.mainHandle = nil
		s.mainEngine = ""
		if clearTunIface {
			s.tunIface = ""
		}
		return fmt.Errorf("process not ready: %w", err)
	}
	return nil
}

// waitForTUNReady 等待 TUN 接口就绪
func (s *Service) waitForTUNReady(interfaceName string, timeout time.Duration, done <-chan struct{}) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if done != nil {
			select {
			case <-done:
				return errors.New("kernel exited before TUN interface ready")
			default:
			}
		}
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

func (s *Service) waitForTUNReadyWithIndex(interfaceName string, prevIndex int, timeout time.Duration, done <-chan struct{}) error {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return errors.New("interface name is empty")
	}
	if prevIndex <= 0 {
		return s.waitForTUNReady(interfaceName, timeout, done)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if done != nil {
			select {
			case <-done:
				return errors.New("kernel exited before TUN interface ready")
			default:
			}
		}
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				if strings.TrimSpace(iface.Name) != interfaceName {
					continue
				}
				if iface.Index != prevIndex {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("TUN interface %s not recreated after %v (previous index %d)", interfaceName, timeout, prevIndex)
}

func snapshotInterfaceIndices() map[string]int {
	out := make(map[string]int)
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		name := strings.TrimSpace(iface.Name)
		if name == "" {
			continue
		}
		out[name] = iface.Index
	}
	return out
}

func isTunInterfaceName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}

	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "tun") || strings.HasPrefix(lower, "utun") {
		return true
	}

	// Linux: TUN 设备名不一定以 tun*/utun* 开头（例如某些内核会用自定义名称）。
	// 通过 sysfs 判断是否是 TUN 设备更可靠。
	if runtime.GOOS == "linux" {
		if _, err := os.Stat(filepath.Join("/sys/class/net", name, "tun_flags")); err == nil {
			return true
		}
	}
	return false
}

func expectedTUNAddressesForEngine(engine domain.CoreEngineKind, cfg domain.ProxyConfig) []string {
	if engine == domain.EngineClash {
		// mihomo/clash 的 TUN 默认地址/网段就是 198.18.0.1/30（参见 adapters/clash.go）。
		return []string{"198.18.0.1/30"}
	}
	if cfg.TUNSettings != nil && len(cfg.TUNSettings.Address) > 0 {
		return cfg.TUNSettings.Address
	}
	return nil
}

func parseTUNAddressCIDRs(addrs []string) ([]*net.IPNet, error) {
	nets := []*net.IPNet{}
	for _, raw := range addrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(raw)
		if err != nil || ipNet == nil {
			continue
		}
		nets = append(nets, ipNet)
	}
	if len(nets) == 0 {
		return nil, errors.New("tun address is empty or invalid")
	}
	return nets, nil
}

func interfaceHasAnyCIDRAddr(iface net.Interface, nets []*net.IPNet) bool {
	if len(nets) == 0 {
		return false
	}
	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}
		if ip == nil {
			continue
		}
		for _, n := range nets {
			if n != nil && n.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func isNewInterface(existing map[string]int, iface net.Interface) bool {
	if existing == nil {
		return true
	}
	name := strings.TrimSpace(iface.Name)
	if name == "" {
		return false
	}
	idx, ok := existing[name]
	return !ok || idx != iface.Index
}

func looksLikeTunInterface(name string, flags net.Flags) bool {
	if flags&net.FlagPointToPoint != 0 {
		return true
	}
	return isTunInterfaceName(name)
}

// waitForTUNReadyByAddress 在非 Linux 平台用 “地址匹配 + 新网卡” 识别 TUN。
// 返回实际创建的网卡名（用于诊断/展示；Linux 仍使用固定 interface_name 逻辑）。
func (s *Service) waitForTUNReadyByAddress(existing map[string]int, desiredName string, addresses []string, timeout time.Duration, done <-chan struct{}) (string, error) {
	desiredName = strings.TrimSpace(desiredName)

	nets, err := parseTUNAddressCIDRs(addresses)
	if err != nil {
		return "", err
	}

	startedAt := time.Now()
	deadline := startedAt.Add(timeout)

	fallback := ""
	for time.Now().Before(deadline) {
		if done != nil {
			select {
			case <-done:
				return "", errors.New("kernel exited before TUN interface ready")
			default:
			}
		}

		ifaces, err := net.Interfaces()
		if err == nil {
			// 1) 若用户显式指定 interfaceName，则优先按名称匹配（兼容非默认名称场景）。
			if desiredName != "" {
				for _, iface := range ifaces {
					if strings.TrimSpace(iface.Name) != desiredName {
						continue
					}
					if interfaceHasAnyCIDRAddr(iface, nets) {
						return desiredName, nil
					}
				}
			}

			// 2) 优先匹配“新出现的”网卡，避免误命中历史接口（如 Docker/虚拟网卡）。
			for _, iface := range ifaces {
				name := strings.TrimSpace(iface.Name)
				if name == "" {
					continue
				}
				if !interfaceHasAnyCIDRAddr(iface, nets) {
					continue
				}
				if isNewInterface(existing, iface) {
					return name, nil
				}
				// 旧接口仅作为兜底候选：要求更像 TUN，且等待一小段时间优先给“新网卡”机会。
				if fallback == "" && looksLikeTunInterface(name, iface.Flags) {
					fallback = name
				}
			}

			if fallback != "" && time.Since(startedAt) > 2*time.Second {
				return fallback, nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("no TUN interface detected after %v", timeout)
}

// waitForNewTUNReady 等待一个“新出现的”TUN 接口（用于无法指定/无法预知接口名的内核）
func (s *Service) waitForNewTUNReady(existing map[string]int, timeout time.Duration, done <-chan struct{}) (string, error) {
	if existing == nil {
		existing = map[string]int{}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if done != nil {
			select {
			case <-done:
				return "", errors.New("kernel exited before TUN interface ready")
			default:
			}
		}
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				name := strings.TrimSpace(iface.Name)
				if name == "" {
					continue
				}
				if _, ok := existing[name]; ok {
					continue
				}
				if isTunInterfaceName(name) {
					return name, nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("no new TUN interface detected after %v", timeout)
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
		if handle != nil && handle.LogCloser != nil {
			_ = handle.LogCloser.Close()
		}
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
		s.tunIface = ""
	}
	s.mu.Unlock()
}

// ========== 引擎推荐 ==========

// EngineRecommendation 引擎推荐结果
type EngineRecommendation struct {
	RecommendedEngine domain.CoreEngineKind `json:"recommendedEngine"`
	Reason            string                `json:"reason"`
	TotalNodes        int                   `json:"totalNodes"`
}

// EngineStatus 引擎状态
type EngineStatus struct {
	SingBoxInstalled bool                  `json:"singboxInstalled"`
	ClashInstalled   bool                  `json:"clashInstalled"`
	DefaultEngine    domain.CoreEngineKind `json:"defaultEngine"`
	Recommendation   EngineRecommendation  `json:"recommendation"`
}

// RecommendEngine 根据现有节点智能推荐引擎
// 规则：
// 1. 优先 sing-box（协议覆盖更广）
// 2. sing-box 不可用时回退到 clash(mihomo)
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
	if comp, err := s.components.GetByKind(ctx, domain.ComponentSingBox); err == nil {
		if comp.InstallDir != "" && comp.LastInstalledAt.Unix() > 0 {
			status.SingBoxInstalled = true
		}
	}
	if comp, err := s.components.GetByKind(ctx, domain.ComponentClash); err == nil {
		if comp.InstallDir != "" && comp.LastInstalledAt.Unix() > 0 {
			status.ClashInstalled = true
		}
	}

	return status
}
