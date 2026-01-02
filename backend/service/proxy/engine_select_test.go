package proxy

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/memory"
	"vea/backend/service/adapters"
)

func installTestEngines(t *testing.T, store *memory.Store, installXray, installSingBox bool) *memory.ComponentRepo {
	t.Helper()

	repo := memory.NewComponentRepo(store)
	ctx := context.Background()

	if installXray {
		xrayDir := filepath.Join(t.TempDir(), "xray")
		comp, err := repo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentXray, Name: "Xray"})
		if err != nil {
			t.Fatalf("create xray component: %v", err)
		}
		if err := repo.SetInstalled(ctx, comp.ID, xrayDir, "test", ""); err != nil {
			t.Fatalf("set xray installed: %v", err)
		}
	}

	if installSingBox {
		singDir := filepath.Join(t.TempDir(), "singbox")
		comp, err := repo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentSingBox, Name: "sing-box"})
		if err != nil {
			t.Fatalf("create sing-box component: %v", err)
		}
		if err := repo.SetInstalled(ctx, comp.ID, singDir, "test", ""); err != nil {
			t.Fatalf("set sing-box installed: %v", err)
		}
	}

	return repo
}

func TestSelectEngineForFRouter_PrefersPreferredEngineWhenSupported(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, true, true)
	settingsRepo := memory.NewSettingsRepo(store)

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", Protocol: domain.ProtocolVLESS},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: "n1", Enabled: true},
			},
		},
	}

	engine, _, err := selectEngineForFRouter(context.Background(), frouter, nodes, domain.EngineXray, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineXray:    &adapters.XrayAdapter{},
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineXray {
		t.Fatalf("expected engine %q, got %q", domain.EngineXray, engine)
	}
}

func TestSelectEngineForFRouter_NodeRequiresSingBoxOverridesPreferred(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, true, true)
	settingsRepo := memory.NewSettingsRepo(store)

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", Protocol: domain.ProtocolHysteria2},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: "n1", Enabled: true},
			},
		},
	}

	engine, _, err := selectEngineForFRouter(context.Background(), frouter, nodes, domain.EngineXray, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineXray:    &adapters.XrayAdapter{},
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineSingBox {
		t.Fatalf("expected engine %q, got %q", domain.EngineSingBox, engine)
	}
}

func TestSelectEngineForFRouter_SettingsDefaultEngineUsedWhenPreferredAuto(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, false, true) // only sing-box installed
	settingsRepo := memory.NewSettingsRepo(store)

	_, err := settingsRepo.UpdateFrontend(context.Background(), map[string]interface{}{
		"engine.defaultEngine": "xray",
	})
	if err != nil {
		t.Fatalf("UpdateFrontend() error: %v", err)
	}

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", Protocol: domain.ProtocolVLESS},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: "n1", Enabled: true},
			},
		},
	}

	engine, _, err := selectEngineForFRouter(context.Background(), frouter, nodes, domain.EngineAuto, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineXray:    &adapters.XrayAdapter{},
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineSingBox {
		t.Fatalf("expected engine %q, got %q", domain.EngineSingBox, engine)
	}
}

func TestSelectEngineForFRouter_NoInstalledEngineSupportsNodes(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, true, false) // only xray installed
	settingsRepo := memory.NewSettingsRepo(store)

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", Protocol: domain.ProtocolHysteria2},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: "n1", Enabled: true},
			},
		},
	}

	_, _, err := selectEngineForFRouter(context.Background(), frouter, nodes, domain.EngineAuto, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineXray:    &adapters.XrayAdapter{},
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestAnyNodeRequiresSingBox_ShadowSocksPluginRequiresSingBox(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{
			ID:       "n1",
			Name:     "ss",
			Protocol: domain.ProtocolShadowsocks,
			Security: &domain.NodeSecurity{Plugin: "v2ray-plugin"},
		},
	}
	if !anyNodeRequiresSingBox(nodes) {
		t.Fatalf("expected anyNodeRequiresSingBox=true")
	}
}

func TestEngineRecommendation_NoNodesDefaultsToXray(t *testing.T) {
	t.Parallel()

	rec := recommendEngineForNodes(nil, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineXray:    &adapters.XrayAdapter{},
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
	})
	if rec.RecommendedEngine != domain.EngineXray {
		t.Fatalf("expected recommended engine %q, got %q", domain.EngineXray, rec.RecommendedEngine)
	}
	if rec.TotalNodes != 0 {
		t.Fatalf("expected TotalNodes=0, got %d", rec.TotalNodes)
	}
}

func TestInstalledEnginesFromComponents_IgnoresUninstalled(t *testing.T) {
	t.Parallel()

	comps := []domain.CoreComponent{
		{Kind: domain.ComponentXray, InstallDir: "/tmp/x", LastInstalledAt: time.Now()},
		{Kind: domain.ComponentSingBox, InstallDir: "", LastInstalledAt: time.Now()}, // not installed
	}
	installed := installedEnginesFromComponents(comps)
	if _, ok := installed[domain.EngineXray]; !ok {
		t.Fatalf("expected xray to be installed")
	}
	if _, ok := installed[domain.EngineSingBox]; ok {
		t.Fatalf("expected sing-box to be not installed")
	}
}
