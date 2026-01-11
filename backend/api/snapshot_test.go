package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vea/backend/domain"
	"vea/backend/persist"
	"vea/backend/repository"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service"
	"vea/backend/service/component"
	configsvc "vea/backend/service/config"
	"vea/backend/service/frouter"
	"vea/backend/service/geo"
	"vea/backend/service/nodes"
	"vea/backend/service/proxy"
)

func TestGETSnapshot_IncludesRuntimeMetrics(t *testing.T) {
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

	facade := service.NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, nil, repos)
	router := NewRouter(facade)

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

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var snapshot domain.ServiceState
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
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
