package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func newTestRouter() http.Handler {
	cfg := &config.Config{App: config.AppConfig{Env: "test", Port: "0"}}
	logg := logger.New(logger.Options{ServiceName: "test-routing", Level: logger.ParseLevel("debug"), Output: io.Discard})
	return NewRouter(cfg, logg)
}

func TestHealthGroupAccessible(t *testing.T) {
	router := newTestRouter()
	for _, path := range []string{"/health/live", "/health/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s got %d", path, resp.Code)
		}
	}
}

func TestPrivateGroupRejectsMissingJWT(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/private/ping", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token got %d", resp.Code)
	}
}

func TestAdminGroupRequiresAdminRole(t *testing.T) {
	router := newTestRouter()
	nonAdmin := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	nonAdmin.Header.Set("Authorization", "Bearer alice|buyer|store-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, nonAdmin)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin got %d", resp.Code)
	}

	admin := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	admin.Header.Set("Authorization", "Bearer bob|admin|store-1")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, admin)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin got %d", resp.Code)
	}
}

func TestAgentGroupRequiresAgentRole(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/ping", nil)
	req.Header.Set("Authorization", "Bearer matt|agent|store-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent got %d", resp.Code)
	}
}

func TestPrivateGroupSucceedsWithJWT(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/private/ping", nil)
	req.Header.Set("Authorization", "Bearer patti|buyer|store-xyz")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for private ping got %d", resp.Code)
	}
}
