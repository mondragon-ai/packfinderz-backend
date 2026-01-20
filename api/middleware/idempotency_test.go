package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

type fakeStore struct {
	data map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: make(map[string]string)}
}

func (f *fakeStore) Get(_ context.Context, key string) (string, error) {
	if v, ok := f.data[key]; ok {
		return v, nil
	}
	return "", redis.Nil
}

func (f *fakeStore) SetNX(_ context.Context, key string, value any, _ time.Duration) (bool, error) {
	if _, ok := f.data[key]; ok {
		return false, nil
	}
	str, _ := value.(string)
	f.data[key] = str
	return true, nil
}

func (f *fakeStore) IdempotencyKey(scope, id string) string {
	return fmt.Sprintf("fake:%s:%s", scope, id)
}

func requestWithPattern(method, url, pattern string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	rc := chi.NewRouteContext()
	rc.RoutePatterns = []string{pattern}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))
}

func TestRouteTTLSelection(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		pattern string
		want    time.Duration
		ok      bool
	}{
		{"checkout", http.MethodPost, "/api/v1/checkout", criticalIdempotencyTTL, true},
		{"vendor decision", http.MethodPost, "/api/v1/vendor/orders/123/decision", criticalIdempotencyTTL, true},
		{"order cancel", http.MethodPost, "/api/v1/orders/456/cancel", criticalIdempotencyTTL, true},
		{"vendor subscriptions", http.MethodPost, "/api/v1/vendor/subscriptions", defaultIdempotencyTTL, true},
		{"admin order action", http.MethodPost, "/api/v1/admin/orders/abc/assign-agent", defaultIdempotencyTTL, true},
		{"non idempotent", http.MethodPost, "/api/v1/auth/login", 0, false},
	}

	for _, tt := range tests {
		ttl, ok := routeTTL(tt.method, tt.pattern)
		if ok != tt.ok {
			t.Fatalf("%s: expected ok=%v got %v", tt.name, tt.ok, ok)
		}
		if ok && ttl != tt.want {
			t.Fatalf("%s: expected ttl=%v got %v", tt.name, tt.want, ttl)
		}
	}
}

func TestIdempotencyMiddlewareRequiresHeader(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store, nil)
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	})

	req := requestWithPattern(http.MethodPost, "/api/v1/auth/register", "/api/v1/auth/register", strings.NewReader(`{"foo":"bar"}`))
	resp := httptest.NewRecorder()
	mw(handler).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
	if handlerCalled {
		t.Fatalf("handler should not run without idempotency key")
	}
}

func TestIdempotencyMiddlewareReplaysStoredResponse(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store, nil)
	var calls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := requestWithPattern(http.MethodPost, "/api/v1/auth/register", "/api/v1/auth/register", strings.NewReader(`{"foo":"bar"}`))
	req.Header.Set("Idempotency-Key", "abc")
	resp := httptest.NewRecorder()
	mw(handler).ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected first response 202 got %d", resp.Code)
	}

	replay := requestWithPattern(http.MethodPost, "/api/v1/auth/register", "/api/v1/auth/register", strings.NewReader(`{"foo":"bar"}`))
	replay.Header.Set("Idempotency-Key", "abc")
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, replay)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected replay status 202 got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected content-type header preserved")
	}
	if strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatalf("expected stored body got %s", rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("handler executed %d times, expected 1", calls)
	}
}

func TestIdempotencyMiddlewareDetectsBodyChange(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := requestWithPattern(http.MethodPost, "/api/v1/auth/register", "/api/v1/auth/register", strings.NewReader(`{"foo":"bar"}`))
	req.Header.Set("Idempotency-Key", "xyz")
	mw(handler).ServeHTTP(httptest.NewRecorder(), req)

	replay := requestWithPattern(http.MethodPost, "/api/v1/auth/register", "/api/v1/auth/register", strings.NewReader(`{"foo":"diff"}`))
	replay.Header.Set("Idempotency-Key", "xyz")
	resp := httptest.NewRecorder()
	mw(handler).ServeHTTP(resp, replay)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", resp.Code)
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("parse error response: %v", err)
	}
	if payload.Error.Code != string(pkgerrors.CodeIdempotency) {
		t.Fatalf("expected error code %s got %s", pkgerrors.CodeIdempotency, payload.Error.Code)
	}
}
