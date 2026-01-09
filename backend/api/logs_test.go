package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"vea/backend/service"
)

func TestGETAppLogs_InvalidSince_ReturnsBadRequest(t *testing.T) {
	t.Parallel()

	// 这里仅验证 query 参数校验；请求会在进入 Facade 调用前返回，因此允许注入全 nil Facade。
	facade := service.NewFacade(nil, nil, nil, nil, nil, nil, nil)
	router := NewRouter(facade)

	req := httptest.NewRequest(http.MethodGet, "/app/logs?since=not-a-number", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestGETAppLogs_NegativeSince_ReturnsBadRequest(t *testing.T) {
	t.Parallel()

	// 这里仅验证 query 参数校验；请求会在进入 Facade 调用前返回，因此允许注入全 nil Facade。
	facade := service.NewFacade(nil, nil, nil, nil, nil, nil, nil)
	router := NewRouter(facade)

	req := httptest.NewRequest(http.MethodGet, "/app/logs?since=-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
