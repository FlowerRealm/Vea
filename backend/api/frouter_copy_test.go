package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vea/backend/domain"
)

func TestPOSTFRouterCopy_CopiesWithoutMutatingOriginal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	nodeRepo, frouterRepo, handler := newTestRouterWithRepos(t)

	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:       "n1",
		Name:     "node-1",
		Protocol: domain.ProtocolVLESS,
		Address:  "example.com",
		Port:     443,
		Security: &domain.NodeSecurity{
			UUID: "11111111-1111-1111-1111-111111111111",
		},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	created, err := frouterRepo.Create(ctx, domain.FRouter{
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e-default",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 123,
					Enabled:  true,
				},
				{
					ID:       "e-1",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"domain:example.com"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/frouters/"+created.ID+"/copy", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var copied domain.FRouter
	if err := json.Unmarshal(rec.Body.Bytes(), &copied); err != nil {
		t.Fatalf("unmarshal response: %v; body=%q", err, rec.Body.String())
	}
	if copied.ID == "" || copied.ID == created.ID {
		t.Fatalf("expected new frouter id, got %q", copied.ID)
	}

	found := false
	for _, e := range copied.ChainProxy.Edges {
		if e.From == domain.EdgeNodeLocal && e.To == domain.EdgeNodeDirect {
			found = true
			if e.Priority != 0 {
				t.Fatalf("expected copied default edge priority normalized to 0, got %d", e.Priority)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected copied frouter to have default edge, got edges=%v", copied.ChainProxy.Edges)
	}

	originalAfter, err := frouterRepo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get original frouter: %v", err)
	}
	gotPriority := -1
	for _, e := range originalAfter.ChainProxy.Edges {
		if e.From == domain.EdgeNodeLocal && e.To == domain.EdgeNodeDirect {
			gotPriority = e.Priority
			break
		}
	}
	if gotPriority != 123 {
		t.Fatalf("expected original default edge priority preserved=123, got %d", gotPriority)
	}
}
