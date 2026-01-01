package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vea/backend/domain"
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

func newTestRouterWithRepos(t *testing.T) (*memory.NodeRepo, *memory.FRouterRepo, http.Handler) {
	t.Helper()

	eventBus := events.NewBus()
	memStore := memory.NewStore(eventBus)

	nodeRepo := memory.NewNodeRepo(memStore)
	frouterRepo := memory.NewFRouterRepo(memStore)
	configRepo := memory.NewConfigRepo(memStore)
	geoRepo := memory.NewGeoRepo(memStore)
	componentRepo := memory.NewComponentRepo(memStore)
	settingsRepo := memory.NewSettingsRepo(memStore)

	repos := &repository.RepositoriesImpl{
		Store: memStore,

		NodeRepo:      nodeRepo,
		FRouterRepo:   frouterRepo,
		ConfigRepo:    configRepo,
		GeoRepo:       geoRepo,
		ComponentRepo: componentRepo,
		SettingsRepo:  settingsRepo,
	}

	nodeSvc := nodes.NewService(nodeRepo)
	frouterSvc := frouter.NewService(frouterRepo, nodeRepo)
	speedMeasurer := proxy.NewSpeedMeasurer(componentRepo, geoRepo, settingsRepo)
	nodeSvc.SetMeasurer(speedMeasurer)
	frouterSvc.SetMeasurer(speedMeasurer)

	configSvc := configsvc.NewService(configRepo, nodeSvc)
	proxySvc := proxy.NewService(frouterRepo, nodeRepo, componentRepo, settingsRepo)
	componentSvc := component.NewService(componentRepo)
	geoSvc := geo.NewService(geoRepo)

	facade := service.NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, repos)
	router := NewRouter(facade)

	// NOTE: gin.Engine already implements http.Handler; we keep the signature compatible with existing tests.
	return nodeRepo, frouterRepo, router
}

func TestGETFRouterGraph_ReturnsGraphData(t *testing.T) {
	t.Parallel()

	nodeRepo, frouterRepo, handler := newTestRouterWithRepos(t)

	_, err := nodeRepo.Create(t.Context(), domain.Node{
		ID:       "n1",
		Name:     "node-1",
		Protocol: domain.ProtocolVLESS,
		Address:  "example.com",
		Port:     443,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	now := time.Now()
	created, err := frouterRepo.Create(t.Context(), domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "n1", Enabled: true},
			},
			Positions: map[string]domain.GraphPosition{
				"n1": {X: 1, Y: 2},
			},
			Slots:     []domain.SlotNode{{ID: "slot-1", Name: "slot"}},
			UpdatedAt: now,
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/frouters/"+created.ID+"/graph", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp frouterGraphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
	}
	if len(resp.Edges) != 1 || resp.Edges[0].ID != "e1" {
		t.Fatalf("expected edges returned, got %+v", resp.Edges)
	}
	if pos, ok := resp.Positions["n1"]; !ok || pos.X != 1 || pos.Y != 2 {
		t.Fatalf("expected positions to be returned, got %+v", resp.Positions)
	}
	if len(resp.Slots) != 1 || resp.Slots[0].ID != "slot-1" {
		t.Fatalf("expected slots returned, got %+v", resp.Slots)
	}
	if resp.UpdatedAt.IsZero() {
		t.Fatalf("expected updatedAt to be set")
	}
}

func TestPUTFRouterGraph_NormalizesDefaultPriority(t *testing.T) {
	t.Parallel()

	_, frouterRepo, handler := newTestRouterWithRepos(t)

	created, err := frouterRepo.Create(t.Context(), domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true}},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	body := []byte(`{
  "edges": [
    {"id":"e1","from":"local","to":"direct","priority":123,"enabled":true}
  ],
  "positions": {},
  "slots": []
}`)
	req := httptest.NewRequest(http.MethodPut, "/frouters/"+created.ID+"/graph", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var updated domain.FRouter
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
	}
	if len(updated.ChainProxy.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(updated.ChainProxy.Edges))
	}
	if got := updated.ChainProxy.Edges[0].Priority; got != 0 {
		t.Fatalf("expected default priority to be normalized to 0, got %d", got)
	}
	if updated.ChainProxy.UpdatedAt.IsZero() {
		t.Fatalf("expected chainProxy.updatedAt to be set")
	}
}

func TestPUTFRouterGraph_InvalidGraphReturnsProblems(t *testing.T) {
	t.Parallel()

	nodeRepo, frouterRepo, handler := newTestRouterWithRepos(t)

	_, err := nodeRepo.Create(t.Context(), domain.Node{
		ID:       "n1",
		Name:     "node-1",
		Protocol: domain.ProtocolVLESS,
		Address:  "example.com",
		Port:     443,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	created, err := frouterRepo.Create(t.Context(), domain.FRouter{Name: "fr-1"})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	// Missing default edge: only a route edge exists.
	body := []byte(`{
  "edges": [
    {"id":"e1","from":"local","to":"n1","priority":10,"enabled":true,"ruleType":"route","routeRule":{"domains":["domain:example.com"]}}
  ],
  "positions": {},
  "slots": []
}`)
	req := httptest.NewRequest(http.MethodPut, "/frouters/"+created.ID+"/graph", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	var resp struct {
		Error    string   `json:"error"`
		Problems []string `json:"problems"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
	}
	if resp.Error != "invalid frouter graph" {
		t.Fatalf("expected error %q, got %q", "invalid frouter graph", resp.Error)
	}
	found := false
	for _, p := range resp.Problems {
		if strings.Contains(p, "missing default edge") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected problems to include missing default edge, got %+v", resp.Problems)
	}
}

func TestPOSTFRouterGraphValidate_ReturnsValidFalseOnCompileError(t *testing.T) {
	t.Parallel()

	nodeRepo, frouterRepo, handler := newTestRouterWithRepos(t)

	_, err := nodeRepo.Create(t.Context(), domain.Node{
		ID:       "n1",
		Name:     "node-1",
		Protocol: domain.ProtocolVLESS,
		Address:  "example.com",
		Port:     443,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	created, err := frouterRepo.Create(t.Context(), domain.FRouter{Name: "fr-1"})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	body := []byte(`{
  "edges": [
    {"id":"e1","from":"local","to":"n1","priority":10,"enabled":true,"ruleType":"route","routeRule":{"domains":["domain:example.com"]}}
  ],
  "positions": {},
  "slots": []
}`)
	req := httptest.NewRequest(http.MethodPost, "/frouters/"+created.ID+"/graph/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp validateGraphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
	}
	if resp.Valid {
		t.Fatalf("expected valid=false")
	}
	found := false
	for _, e := range resp.Errors {
		if strings.Contains(e, "missing default edge") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected errors to include missing default edge, got %+v", resp.Errors)
	}
}
