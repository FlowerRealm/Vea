package service

import (
	"context"
	"errors"
	"testing"

	"vea/backend/domain"
	"vea/backend/persist"
	"vea/backend/repository"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service/component"
	configsvc "vea/backend/service/config"
	"vea/backend/service/frouter"
	"vea/backend/service/geo"
	"vea/backend/service/nodes"
	"vea/backend/service/proxy"
)

func TestFacade_Snapshot_IncludesRuntimeMetrics(t *testing.T) {
	t.Parallel()

	eventBus := events.NewBus()
	memStore := memory.NewStore(eventBus)

	nodeRepo := memory.NewNodeRepo(memStore)
	frouterRepo := memory.NewFRouterRepo(memStore)
	configRepo := memory.NewConfigRepo(memStore)
	geoRepo := memory.NewGeoRepo(memStore)
	componentRepo := memory.NewComponentRepo(memStore)
	settingsRepo := memory.NewSettingsRepo(memStore)

	repos := repository.NewRepositories(memStore, nodeRepo, frouterRepo, configRepo, geoRepo, componentRepo, settingsRepo)

	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	frouterSvc := frouter.NewService(context.Background(), frouterRepo, nodeRepo)
	speedMeasurer := proxy.NewSpeedMeasurer(context.Background(), componentRepo, geoRepo, settingsRepo)
	nodeSvc.SetMeasurer(speedMeasurer)
	frouterSvc.SetMeasurer(speedMeasurer)

	configSvc := configsvc.NewService(context.Background(), configRepo, nodeSvc, frouterRepo)
	proxySvc := proxy.NewService(frouterRepo, nodeRepo, componentRepo, settingsRepo)
	componentSvc := component.NewService(context.Background(), componentRepo)
	geoSvc := geo.NewService(geoRepo)

	facade := NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, nil, repos)

	createdNode, err := nodeRepo.Create(context.Background(), domain.Node{
		Name:     "node-1",
		Protocol: domain.ProtocolVLESS,
		Address:  "example.com",
		Port:     443,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := nodeRepo.UpdateLatency(context.Background(), createdNode.ID, 123, ""); err != nil {
		t.Fatalf("update node latency: %v", err)
	}

	createdFRouter, err := frouterRepo.Create(context.Background(), domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "edge-1", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}
	if err := frouterRepo.UpdateSpeed(context.Background(), createdFRouter.ID, 88.8, ""); err != nil {
		t.Fatalf("update frouter speed: %v", err)
	}

	snapshot, err := facade.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.SchemaVersion != persist.SchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %q", persist.SchemaVersion, snapshot.SchemaVersion)
	}
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected 1 node in snapshot, got %d", len(snapshot.Nodes))
	}
	if snapshot.Nodes[0].LastLatencyMS != 123 {
		t.Fatalf("expected node latency to be preserved, got %d", snapshot.Nodes[0].LastLatencyMS)
	}
	if len(snapshot.FRouters) != 1 {
		t.Fatalf("expected 1 frouter in snapshot, got %d", len(snapshot.FRouters))
	}
	if snapshot.FRouters[0].LastSpeedMbps != 88.8 {
		t.Fatalf("expected frouter speed to be preserved, got %v", snapshot.FRouters[0].LastSpeedMbps)
	}
}

func TestFacade_DeleteFRouter_EnsuresProxyConfigFRouterIDValid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	eventBus := events.NewBus()
	memStore := memory.NewStore(eventBus)

	nodeRepo := memory.NewNodeRepo(memStore)
	frouterRepo := memory.NewFRouterRepo(memStore)
	settingsRepo := memory.NewSettingsRepo(memStore)

	repos := repository.NewRepositories(memStore, nodeRepo, frouterRepo, nil, nil, nil, settingsRepo)
	frouterSvc := frouter.NewService(context.Background(), frouterRepo, nodeRepo)

	facade := NewFacade(nil, frouterSvc, nil, nil, nil, nil, nil, repos)

	created, err := frouterRepo.Create(ctx, domain.FRouter{Name: "fr-1"})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	cfg, err := settingsRepo.GetProxyConfig(ctx)
	if err != nil {
		t.Fatalf("get proxy config: %v", err)
	}
	cfg.FRouterID = created.ID
	if _, err := settingsRepo.UpdateProxyConfig(ctx, cfg); err != nil {
		t.Fatalf("update proxy config: %v", err)
	}

	if err := facade.DeleteFRouter(created.ID); err != nil {
		t.Fatalf("delete frouter: %v", err)
	}

	updated, err := settingsRepo.GetProxyConfig(ctx)
	if err != nil {
		t.Fatalf("get updated proxy config: %v", err)
	}
	if updated.FRouterID == "" {
		t.Fatalf("expected proxy config frouterId to be set")
	}
	if updated.FRouterID == created.ID {
		t.Fatalf("expected proxy config frouterId to change after delete, still %s", updated.FRouterID)
	}
	if _, err := frouterRepo.Get(ctx, updated.FRouterID); err != nil {
		t.Fatalf("expected proxy config frouterId to point to existing frouter, got err=%v", err)
	}
}

type errorNodeRepo struct {
	err error
}

func (r *errorNodeRepo) Get(context.Context, string) (domain.Node, error) {
	return domain.Node{}, r.err
}
func (r *errorNodeRepo) List(context.Context) ([]domain.Node, error) { return nil, r.err }
func (r *errorNodeRepo) Create(context.Context, domain.Node) (domain.Node, error) {
	return domain.Node{}, r.err
}
func (r *errorNodeRepo) Update(context.Context, string, domain.Node) (domain.Node, error) {
	return domain.Node{}, r.err
}
func (r *errorNodeRepo) Delete(context.Context, string) error { return r.err }
func (r *errorNodeRepo) ListByConfigID(context.Context, string) ([]domain.Node, error) {
	return nil, r.err
}
func (r *errorNodeRepo) ReplaceNodesForConfig(context.Context, string, []domain.Node) ([]domain.Node, error) {
	return nil, r.err
}
func (r *errorNodeRepo) UpdateLatency(context.Context, string, int64, string) error { return r.err }
func (r *errorNodeRepo) UpdateSpeed(context.Context, string, float64, string) error { return r.err }

func TestFacade_Snapshot_PropagatesListError(t *testing.T) {
	t.Parallel()

	expected := errors.New("boom")
	nodeSvc := nodes.NewService(context.Background(), &errorNodeRepo{err: expected})

	facade := NewFacade(nodeSvc, nil, nil, nil, nil, nil, nil, nil)
	_, err := facade.Snapshot()
	if !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}
