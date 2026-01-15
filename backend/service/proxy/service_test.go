package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/repository/memory"
	coreadapters "vea/backend/service/adapters"
	"vea/backend/service/nodegroup"
	"vea/backend/service/shared"
)

type fakeCoreAdapter struct {
	kind domain.CoreEngineKind

	binaryNames []string
	buildConfig func(plan nodegroup.RuntimePlan, geo coreadapters.GeoFiles) ([]byte, error)

	startCalls         int
	lastStartCfg       coreadapters.ProcessConfig
	lastStartConfig    string
	startErrs          []error
	startErr           error
	stopCalls          int
	waitForReadyCalls  int
	waitForReadyResult error
}

func (a *fakeCoreAdapter) Kind() domain.CoreEngineKind { return a.kind }
func (a *fakeCoreAdapter) BinaryNames() []string       { return append([]string(nil), a.binaryNames...) }
func (a *fakeCoreAdapter) SupportedProtocols() []domain.NodeProtocol {
	return nil
}
func (a *fakeCoreAdapter) SupportsProtocol(domain.NodeProtocol) bool { return true }
func (a *fakeCoreAdapter) SupportsInbound(domain.InboundMode) bool   { return true }
func (a *fakeCoreAdapter) BuildConfig(plan nodegroup.RuntimePlan, geo coreadapters.GeoFiles) ([]byte, error) {
	if a.buildConfig != nil {
		return a.buildConfig(plan, geo)
	}
	return []byte(`{"ok":true}`), nil
}
func (a *fakeCoreAdapter) RequiresPrivileges(domain.ProxyConfig) bool { return false }
func (a *fakeCoreAdapter) GetCommandArgs(string) []string             { return nil }

func (a *fakeCoreAdapter) Start(cfg coreadapters.ProcessConfig, configPath string) (*coreadapters.ProcessHandle, error) {
	a.startCalls++
	a.lastStartCfg = cfg
	a.lastStartConfig = configPath
	if idx := a.startCalls - 1; idx >= 0 && idx < len(a.startErrs) && a.startErrs[idx] != nil {
		return nil, a.startErrs[idx]
	}
	if a.startErr != nil {
		return nil, a.startErr
	}
	return &coreadapters.ProcessHandle{
		Cmd:        nil,
		ConfigPath: configPath,
		BinaryPath: cfg.BinaryPath,
		StartedAt:  time.Now(),
	}, nil
}

func (a *fakeCoreAdapter) Stop(handle *coreadapters.ProcessHandle) error {
	a.stopCalls++
	return nil
}

func (a *fakeCoreAdapter) WaitForReady(handle *coreadapters.ProcessHandle, timeout time.Duration) error {
	a.waitForReadyCalls++
	return a.waitForReadyResult
}

func seedSingBoxRuleSets(t *testing.T) {
	t.Helper()

	ruleSetDir := filepath.Join(shared.ArtifactsRoot, "core", "sing-box", "rule-set")
	if err := os.MkdirAll(ruleSetDir, 0o755); err != nil {
		t.Fatalf("mkdir rule-set dir: %v", err)
	}
	for _, tag := range shared.DefaultSingBoxRuleSetTags {
		if err := os.WriteFile(filepath.Join(ruleSetDir, tag+".srs"), []byte("dummy"), 0o644); err != nil {
			t.Fatalf("write rule-set %s: %v", tag, err)
		}
	}
}

func TestService_Start_WritesConfigAndSavesProxyConfig(t *testing.T) {
	ctx := context.Background()

	oldRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = t.TempDir()
	t.Cleanup(func() { shared.ArtifactsRoot = oldRoot })
	seedSingBoxRuleSets(t)

	store := memory.NewStore(nil)
	frouterRepo := memory.NewFRouterRepo(store)
	componentRepo := memory.NewComponentRepo(store)
	settingsRepo := memory.NewSettingsRepo(store)

	frouter, err := frouterRepo.Create(ctx, domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	comp, err := componentRepo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentSingBox, Name: "sing-box"})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	installDir := filepath.Join(t.TempDir(), "sing-box")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	binPath := filepath.Join(installDir, "sing-box")
	if err := os.WriteFile(binPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write dummy binary: %v", err)
	}
	if err := componentRepo.SetInstalled(ctx, comp.ID, installDir, "test", ""); err != nil {
		t.Fatalf("set installed: %v", err)
	}

	adapter := &fakeCoreAdapter{
		kind:        domain.EngineSingBox,
		binaryNames: []string{"sing-box"},
	}
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)
	svc.adapters = map[domain.CoreEngineKind]coreadapters.CoreAdapter{
		domain.EngineSingBox: adapter,
	}

	cfg := domain.ProxyConfig{
		FRouterID:       frouter.ID,
		InboundMode:     domain.InboundSOCKS,
		InboundPort:     1081,
		PreferredEngine: domain.EngineSingBox,
	}
	if err := svc.Start(ctx, cfg); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	configDir := engineConfigDir(domain.EngineSingBox)
	configPath := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.explain.txt")); err != nil {
		t.Fatalf("config.explain.txt not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "kernel.log")); err != nil {
		t.Fatalf("kernel.log not written: %v", err)
	}

	if adapter.startCalls != 1 {
		t.Fatalf("expected adapter.Start to be called once, got %d", adapter.startCalls)
	}
	if adapter.lastStartConfig != configPath {
		t.Fatalf("adapter.Start configPath mismatch: got %q want %q", adapter.lastStartConfig, configPath)
	}
	if adapter.lastStartCfg.BinaryPath != binPath {
		t.Fatalf("adapter.Start binaryPath mismatch: got %q want %q", adapter.lastStartCfg.BinaryPath, binPath)
	}
	if adapter.lastStartCfg.ConfigDir != configDir {
		t.Fatalf("adapter.Start configDir mismatch: got %q want %q", adapter.lastStartCfg.ConfigDir, configDir)
	}

	stored, err := settingsRepo.GetProxyConfig(ctx)
	if err != nil {
		t.Fatalf("GetProxyConfig() error: %v", err)
	}
	if stored.FRouterID != frouter.ID {
		t.Fatalf("stored proxyConfig.frouterId mismatch: got %q want %q", stored.FRouterID, frouter.ID)
	}
	if stored.InboundPort != 1081 {
		t.Fatalf("stored proxyConfig.inboundPort mismatch: got %d want %d", stored.InboundPort, 1081)
	}

	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
}

func TestService_Start_MissingFRouterIDIsInvalidData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.NewStore(nil)

	frouterRepo := memory.NewFRouterRepo(store)
	componentRepo := memory.NewComponentRepo(store)
	settingsRepo := memory.NewSettingsRepo(store)
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)

	err := svc.Start(ctx, domain.ProxyConfig{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrInvalidData) {
		t.Fatalf("expected errors.Is(..., ErrInvalidData)=true, got err=%v", err)
	}
}

func TestService_Start_InboundPortInUseReturnsInvalidData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	oldRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = t.TempDir()
	t.Cleanup(func() { shared.ArtifactsRoot = oldRoot })
	seedSingBoxRuleSets(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	port := ln.Addr().(*net.TCPAddr).Port

	store := memory.NewStore(nil)
	frouterRepo := memory.NewFRouterRepo(store)
	componentRepo := memory.NewComponentRepo(store)
	settingsRepo := memory.NewSettingsRepo(store)

	frouter, err := frouterRepo.Create(ctx, domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	comp, err := componentRepo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentSingBox, Name: "sing-box"})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	installDir := filepath.Join(t.TempDir(), "sing-box")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	binPath := filepath.Join(installDir, "sing-box")
	if err := os.WriteFile(binPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write dummy binary: %v", err)
	}
	if err := componentRepo.SetInstalled(ctx, comp.ID, installDir, "test", ""); err != nil {
		t.Fatalf("set installed: %v", err)
	}

	adapter := &fakeCoreAdapter{
		kind:        domain.EngineSingBox,
		binaryNames: []string{"sing-box"},
	}
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)
	svc.adapters = map[domain.CoreEngineKind]coreadapters.CoreAdapter{
		domain.EngineSingBox: adapter,
	}

	err = svc.Start(ctx, domain.ProxyConfig{
		FRouterID:       frouter.ID,
		InboundMode:     domain.InboundMixed,
		InboundPort:     port,
		PreferredEngine: domain.EngineSingBox,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrInvalidData) {
		t.Fatalf("expected errors.Is(..., ErrInvalidData)=true, got err=%v", err)
	}
	if adapter.startCalls != 0 {
		t.Fatalf("expected adapter.Start not to be called, got %d", adapter.startCalls)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("127.0.0.1:%d", port)) {
		t.Fatalf("expected error to contain %q, got %v", fmt.Sprintf("127.0.0.1:%d", port), err)
	}
}

func TestTuneTUNSettingsForEngine_ClashLinux_AdjustsDefaultMTU(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only: MTU default fix is for Linux + mihomo")
	}

	cfg := domain.ProxyConfig{
		InboundMode: domain.InboundTUN,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "vea",
			MTU:           9000,
			Address:       []string{"172.19.0.1/30"},
			AutoRoute:     true,
			StrictRoute:   true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	got, changed := tuneTUNSettingsForEngine(domain.EngineClash, cfg)
	if !changed {
		t.Fatalf("expected config to be tuned, got changed=false")
	}
	if got.TUNSettings == nil {
		t.Fatalf("expected tun settings to be present")
	}
	if got.TUNSettings.MTU != 1500 {
		t.Fatalf("expected MTU=1500 after tuning, got %d", got.TUNSettings.MTU)
	}
}

func TestTuneTUNSettingsForEngine_ClashLinux_DoesNotOverrideCustomMTU(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only: MTU default fix is for Linux + mihomo")
	}

	cfg := domain.ProxyConfig{
		InboundMode: domain.InboundTUN,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "vea",
			MTU:           1500,
			Address:       []string{"172.19.0.1/30"},
			AutoRoute:     true,
			StrictRoute:   true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	got, changed := tuneTUNSettingsForEngine(domain.EngineClash, cfg)
	if changed {
		t.Fatalf("expected config not to be tuned, got changed=true (mtu=%d)", got.TUNSettings.MTU)
	}
}

func TestService_Start_DoesNotStopPreviousProxyOnCompileError(t *testing.T) {
	ctx := context.Background()

	oldRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = t.TempDir()
	t.Cleanup(func() { shared.ArtifactsRoot = oldRoot })
	seedSingBoxRuleSets(t)

	store := memory.NewStore(nil)
	frouterRepo := memory.NewFRouterRepo(store)
	componentRepo := memory.NewComponentRepo(store)
	settingsRepo := memory.NewSettingsRepo(store)

	frouter, err := frouterRepo.Create(ctx, domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	comp, err := componentRepo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentSingBox, Name: "sing-box"})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	installDir := filepath.Join(t.TempDir(), "sing-box")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	binPath := filepath.Join(installDir, "sing-box")
	if err := os.WriteFile(binPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write dummy binary: %v", err)
	}
	if err := componentRepo.SetInstalled(ctx, comp.ID, installDir, "test", ""); err != nil {
		t.Fatalf("set installed: %v", err)
	}

	adapter := &fakeCoreAdapter{
		kind:        domain.EngineSingBox,
		binaryNames: []string{"sing-box"},
	}
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)
	svc.adapters = map[domain.CoreEngineKind]coreadapters.CoreAdapter{
		domain.EngineSingBox: adapter,
	}

	cfg := domain.ProxyConfig{
		FRouterID:       frouter.ID,
		InboundMode:     domain.InboundSOCKS,
		InboundPort:     1081,
		PreferredEngine: domain.EngineSingBox,
	}
	if err := svc.Start(ctx, cfg); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	beforeHandle := svc.mainHandle
	beforeEngine := svc.mainEngine

	current, err := frouterRepo.Get(ctx, frouter.ID)
	if err != nil {
		t.Fatalf("get frouter: %v", err)
	}
	current.ChainProxy.Edges = []domain.ProxyEdge{}
	if _, err := frouterRepo.Update(ctx, frouter.ID, current); err != nil {
		t.Fatalf("update frouter: %v", err)
	}

	err = svc.Start(ctx, cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if adapter.stopCalls != 0 {
		t.Fatalf("expected adapter.Stop not to be called, got %d", adapter.stopCalls)
	}
	if adapter.startCalls != 1 {
		t.Fatalf("expected adapter.Start not to be called again, got %d", adapter.startCalls)
	}
	if svc.mainHandle != beforeHandle {
		t.Fatalf("expected mainHandle to remain unchanged on compile error")
	}
	if svc.mainEngine != beforeEngine {
		t.Fatalf("expected mainEngine to remain unchanged on compile error")
	}
	if svc.mainHandle == nil {
		t.Fatalf("expected proxy to still be running (mainHandle != nil)")
	}
}

func TestService_Start_InvalidFRouterIsCompileErrorAndInvalidData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.NewStore(nil)

	frouterRepo := memory.NewFRouterRepo(store)
	frouter, err := frouterRepo.Create(ctx, domain.FRouter{
		Name:       "fr-invalid",
		ChainProxy: domain.ChainProxySettings{Edges: []domain.ProxyEdge{}},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	componentRepo := memory.NewComponentRepo(store)
	settingsRepo := memory.NewSettingsRepo(store)
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)

	err = svc.Start(ctx, domain.ProxyConfig{
		FRouterID:       frouter.ID,
		InboundMode:     domain.InboundSOCKS,
		InboundPort:     1080,
		PreferredEngine: domain.EngineSingBox,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var ce *nodegroup.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("expected errors.As to match *CompileError, got %T: %v", err, err)
	}
	if !errors.Is(err, repository.ErrInvalidData) {
		t.Fatalf("expected errors.Is(..., ErrInvalidData)=true, got err=%v", err)
	}
}

func TestService_Start_RollbackToPreviousProxyOnStartFailure(t *testing.T) {
	ctx := context.Background()

	oldRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = t.TempDir()
	t.Cleanup(func() { shared.ArtifactsRoot = oldRoot })
	seedSingBoxRuleSets(t)

	store := memory.NewStore(nil)
	frouterRepo := memory.NewFRouterRepo(store)
	componentRepo := memory.NewComponentRepo(store)
	settingsRepo := memory.NewSettingsRepo(store)

	frouter, err := frouterRepo.Create(ctx, domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	comp, err := componentRepo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentSingBox, Name: "sing-box"})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	installDir := filepath.Join(t.TempDir(), "sing-box")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	binPath := filepath.Join(installDir, "sing-box")
	if err := os.WriteFile(binPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write dummy binary: %v", err)
	}
	if err := componentRepo.SetInstalled(ctx, comp.ID, installDir, "test", ""); err != nil {
		t.Fatalf("set installed: %v", err)
	}

	adapter := &fakeCoreAdapter{
		kind:        domain.EngineSingBox,
		binaryNames: []string{"sing-box"},
		startErrs:   []error{nil, errors.New("boom"), nil},
	}
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)
	svc.adapters = map[domain.CoreEngineKind]coreadapters.CoreAdapter{
		domain.EngineSingBox: adapter,
	}

	cfg := domain.ProxyConfig{
		FRouterID:       frouter.ID,
		InboundMode:     domain.InboundSOCKS,
		InboundPort:     1081,
		PreferredEngine: domain.EngineSingBox,
	}
	if err := svc.Start(ctx, cfg); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	beforeCfg := svc.activeCfg

	err = svc.Start(ctx, cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if adapter.startCalls != 3 {
		t.Fatalf("expected adapter.Start to be called 3 times (start + fail + rollback), got %d", adapter.startCalls)
	}
	if adapter.stopCalls != 1 {
		t.Fatalf("expected adapter.Stop to be called once (stop previous), got %d", adapter.stopCalls)
	}
	if svc.mainHandle == nil {
		t.Fatalf("expected proxy to be running after rollback (mainHandle != nil)")
	}
	if svc.activeCfg.FRouterID != beforeCfg.FRouterID {
		t.Fatalf("expected activeCfg to be rolled back to previous config")
	}
}

func TestService_Start_BinaryMissingReturnsErrEngineNotInstalled(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(nil)

	frouterRepo := memory.NewFRouterRepo(store)
	frouter, err := frouterRepo.Create(ctx, domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	componentRepo := memory.NewComponentRepo(store)
	comp, err := componentRepo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentSingBox, Name: "sing-box"})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	oldRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = t.TempDir()
	t.Cleanup(func() { shared.ArtifactsRoot = oldRoot })
	seedSingBoxRuleSets(t)

	installDir := filepath.Join(t.TempDir(), "sing-box")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	if err := componentRepo.SetInstalled(ctx, comp.ID, installDir, "test", ""); err != nil {
		t.Fatalf("set installed: %v", err)
	}

	settingsRepo := memory.NewSettingsRepo(store)

	adapter := &fakeCoreAdapter{
		kind:        domain.EngineSingBox,
		binaryNames: []string{"sing-box"},
	}
	svc := NewService(frouterRepo, nil, componentRepo, settingsRepo)
	svc.adapters = map[domain.CoreEngineKind]coreadapters.CoreAdapter{
		domain.EngineSingBox: adapter,
	}

	err = svc.Start(ctx, domain.ProxyConfig{
		FRouterID:       frouter.ID,
		InboundMode:     domain.InboundSOCKS,
		InboundPort:     1080,
		PreferredEngine: domain.EngineSingBox,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("expected errors.Is(..., ErrEngineNotInstalled)=true, got err=%v", err)
	}
}
