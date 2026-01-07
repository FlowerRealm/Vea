package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestGETComponents_SeedsDefaultComponents(t *testing.T) {
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

	nodeSvc := nodes.NewService(nodeRepo)
	frouterSvc := frouter.NewService(frouterRepo, nodeRepo)
	speedMeasurer := proxy.NewSpeedMeasurer(componentRepo, geoRepo, settingsRepo)
	nodeSvc.SetMeasurer(speedMeasurer)
	frouterSvc.SetMeasurer(speedMeasurer)

	configSvc := configsvc.NewService(configRepo, nodeSvc, nil)
	proxySvc := proxy.NewService(frouterRepo, nodeRepo, componentRepo, settingsRepo)
	componentSvc := component.NewService(componentRepo)
	geoSvc := geo.NewService(geoRepo)

	facade := service.NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, repos)
	router := NewRouter(facade)

	req := httptest.NewRequest(http.MethodGet, "/components", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var components []domain.CoreComponent
	if err := json.Unmarshal(rec.Body.Bytes(), &components); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
	}

	if countKind(components, domain.ComponentXray) != 1 {
		t.Fatalf("expected exactly 1 xray component, got %d", countKind(components, domain.ComponentXray))
	}
	if countKind(components, domain.ComponentSingBox) != 1 {
		t.Fatalf("expected exactly 1 singbox component, got %d", countKind(components, domain.ComponentSingBox))
	}
	if countKind(components, domain.ComponentClash) != 1 {
		t.Fatalf("expected exactly 1 clash component, got %d", countKind(components, domain.ComponentClash))
	}
}

func countKind(components []domain.CoreComponent, kind domain.CoreComponentKind) int {
	count := 0
	for _, comp := range components {
		if comp.Kind == kind {
			count++
		}
	}
	return count
}
