package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"vea/backend/domain"
)

func TestPUTFRouterMeta_RenamesAndKeepsTags(t *testing.T) {
	t.Parallel()

	_, frouterRepo, handler := newTestRouterWithRepos(t)

	created, err := frouterRepo.Create(context.Background(), domain.FRouter{
		Name: "old-name",
		Tags: []string{"tag-a"},
		ChainProxy: domain.ChainProxySettings{
			// Intentionally invalid graph (missing default edge) to ensure
			// /frouters/:id/meta does not trigger compile validation.
			Edges: []domain.ProxyEdge{
				{
					ID:        "e1",
					From:      domain.EdgeNodeLocal,
					To:        domain.EdgeNodeDirect,
					Priority:  10,
					Enabled:   true,
					RuleType:  domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{Domains: []string{"domain:example.com"}},
				},
			},
			Slots: []domain.SlotNode{{ID: "slot-1", Name: "slot"}},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	body := []byte(`{"name":"new-name"}`)
	req := httptest.NewRequest(http.MethodPut, "/frouters/"+created.ID+"/meta", bytes.NewReader(body))
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
	if updated.Name != "new-name" {
		t.Fatalf("expected name=%q, got %q", "new-name", updated.Name)
	}
	if !reflect.DeepEqual(updated.Tags, []string{"tag-a"}) {
		t.Fatalf("expected tags to be kept, got %+v", updated.Tags)
	}
}

func TestPUTFRouterMeta_EmptyBodyReturns400(t *testing.T) {
	t.Parallel()

	_, frouterRepo, handler := newTestRouterWithRepos(t)

	created, err := frouterRepo.Create(context.Background(), domain.FRouter{Name: "fr-1"})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPut, "/frouters/"+created.ID+"/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestPUTFRouter_WhenTagsOmitted_DoesNotClearTags(t *testing.T) {
	t.Parallel()

	_, frouterRepo, handler := newTestRouterWithRepos(t)

	created, err := frouterRepo.Create(context.Background(), domain.FRouter{
		Name: "old-name",
		Tags: []string{"tag-a"},
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
			},
			Slots: []domain.SlotNode{{ID: "slot-1", Name: "slot"}},
		},
	})
	if err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	body := []byte(`{"name":"new-name"}`)
	req := httptest.NewRequest(http.MethodPut, "/frouters/"+created.ID, bytes.NewReader(body))
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
	if updated.Name != "new-name" {
		t.Fatalf("expected name=%q, got %q", "new-name", updated.Name)
	}
	if !reflect.DeepEqual(updated.Tags, []string{"tag-a"}) {
		t.Fatalf("expected tags to be kept, got %+v", updated.Tags)
	}
}
