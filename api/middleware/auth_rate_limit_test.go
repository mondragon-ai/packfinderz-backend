package middleware

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

func TestAuthRateLimit_AllowsUnderLimit(t *testing.T) {
	store := newFakeRateStore()
	policy := NewAuthRateLimitPolicy("login", time.Minute, 2, 2)
	handler := AuthRateLimit(policy, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), `"email":"tester@example.com"`) {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"tester@example.com","password":"secret"}`))
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthRateLimit_EmailLimitTriggers(t *testing.T) {
	store := newFakeRateStore()
	policy := NewAuthRateLimitPolicy("login", time.Minute, 0, 2)
	handler := AuthRateLimit(policy, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"blocked@example.com","password":"secret"}`))
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		switch {
		case i < 2 && rec.Code != http.StatusOK:
			t.Fatalf("expected success before limit, got %d", rec.Code)
		case i >= 2:
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429, got %d", rec.Code)
			}
			var payload struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if payload.Error.Code != string(pkgerrors.CodeRateLimit) {
				t.Fatalf("unexpected code: %s", payload.Error.Code)
			}
		}
	}
}

func TestAuthRateLimit_IPLimitTriggers(t *testing.T) {
	store := newFakeRateStore()
	policy := NewAuthRateLimitPolicy("register", time.Minute, 1, 0)
	handler := AuthRateLimit(policy, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"foo@example.com","password":"secret"}`))
		req.RemoteAddr = "5.6.7.8:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("expected success, got %d", rec.Code)
		}
		if i == 1 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429, got %d", rec.Code)
			}
		}
	}
}

type fakeRateStore struct {
	mu     sync.Mutex
	counts map[string]int64
}

func newFakeRateStore() *fakeRateStore {
	return &fakeRateStore{counts: map[string]int64{}}
}

func (f *fakeRateStore) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[key]++
	return f.counts[key], nil
}
