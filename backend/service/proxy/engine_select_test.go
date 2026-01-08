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

func installTestEngines(t *testing.T, store *memory.Store, installSingBox, installClash bool) *memory.ComponentRepo {
	t.Helper()

	repo := memory.NewComponentRepo(store)
	ctx := context.Background()

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

	if installClash {
		clashDir := filepath.Join(t.TempDir(), "clash")
		comp, err := repo.Create(ctx, domain.CoreComponent{Kind: domain.ComponentClash, Name: "Clash"})
		if err != nil {
			t.Fatalf("create clash component: %v", err)
		}
		if err := repo.SetInstalled(ctx, comp.ID, clashDir, "test", ""); err != nil {
			t.Fatalf("set clash installed: %v", err)
		}
	}

	return repo
}

func TestSelectEngineForFRouter_PrefersPreferredEngineWhenInstalled(t *testing.T) {
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

	engine, _, err := selectEngineForFRouter(context.Background(), domain.InboundMixed, frouter, nodes, domain.EngineSingBox, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
		domain.EngineClash:   &adapters.ClashAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineSingBox {
		t.Fatalf("expected engine %q, got %q", domain.EngineSingBox, engine)
	}
}

func TestSelectEngineForFRouter_UnknownPreferredEngineReturnsError(t *testing.T) {
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

	_, _, err := selectEngineForFRouter(context.Background(), domain.InboundMixed, frouter, nodes, domain.CoreEngineKind("unknown"), componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
		domain.EngineClash:   &adapters.ClashAdapter{},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestSelectEngineForFRouter_SettingsDefaultEngineUsedWhenPreferredAuto(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, true, false) // only sing-box installed
	settingsRepo := memory.NewSettingsRepo(store)

	_, err := settingsRepo.UpdateFrontend(context.Background(), map[string]interface{}{
		// unknown engine should be ignored and fallback to installed sing-box.
		"engine.defaultEngine": "unknown",
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

	engine, _, err := selectEngineForFRouter(context.Background(), domain.InboundMixed, frouter, nodes, domain.EngineAuto, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
		domain.EngineClash:   &adapters.ClashAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineSingBox {
		t.Fatalf("expected engine %q, got %q", domain.EngineSingBox, engine)
	}
}

func TestSelectEngineForFRouter_NoEngineInstalled_ReturnsFallback(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, false, false) // no engine installed
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

	engine, comp, err := selectEngineForFRouter(context.Background(), domain.InboundMixed, frouter, nodes, domain.EngineAuto, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
		domain.EngineClash:   &adapters.ClashAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineSingBox {
		t.Fatalf("expected engine %q, got %q", domain.EngineSingBox, engine)
	}
	if comp.ID != "" {
		t.Fatalf("expected empty component when engine is not installed, got id=%q", comp.ID)
	}
}

func TestSelectEngineForFRouter_PrefersClashWhenPreferred(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	componentRepo := installTestEngines(t, store, true, true)
	settingsRepo := memory.NewSettingsRepo(store)

	nodes := []domain.Node{
		{
			ID:       "n1",
			Name:     "ss",
			Protocol: domain.ProtocolShadowsocks,
			Security: &domain.NodeSecurity{Method: "aes-128-gcm", Password: "p", Plugin: "obfs-local", PluginOpts: "obfs=http;obfs-host=example.com"},
		},
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

	engine, _, err := selectEngineForFRouter(context.Background(), domain.InboundMixed, frouter, nodes, domain.EngineClash, componentRepo, settingsRepo, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
		domain.EngineClash:   &adapters.ClashAdapter{},
	})
	if err != nil {
		t.Fatalf("selectEngineForFRouter() error: %v", err)
	}
	if engine != domain.EngineClash {
		t.Fatalf("expected engine %q, got %q", domain.EngineClash, engine)
	}
}

func TestEngineRecommendation_NoNodesDefaultsToSingBox(t *testing.T) {
	t.Parallel()

	rec := recommendEngineForNodes(nil, map[domain.CoreEngineKind]adapters.CoreAdapter{
		domain.EngineSingBox: &adapters.SingBoxAdapter{},
		domain.EngineClash:   &adapters.ClashAdapter{},
	})
	if rec.RecommendedEngine != domain.EngineSingBox {
		t.Fatalf("expected recommended engine %q, got %q", domain.EngineSingBox, rec.RecommendedEngine)
	}
	if rec.TotalNodes != 0 {
		t.Fatalf("expected TotalNodes=0, got %d", rec.TotalNodes)
	}
}

func TestInstalledEnginesFromComponents_IgnoresUninstalled(t *testing.T) {
	t.Parallel()

	comps := []domain.CoreComponent{
		{Kind: domain.ComponentSingBox, InstallDir: "/tmp/sing", LastInstalledAt: time.Now()},
		{Kind: domain.ComponentClash, InstallDir: "", LastInstalledAt: time.Now()}, // not installed
	}
	installed := installedEnginesFromComponents(comps)
	if _, ok := installed[domain.EngineSingBox]; !ok {
		t.Fatalf("expected sing-box to be installed")
	}
	if _, ok := installed[domain.EngineClash]; ok {
		t.Fatalf("expected clash to be not installed")
	}
}
