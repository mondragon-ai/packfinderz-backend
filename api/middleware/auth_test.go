package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

func TestAuthRejectsMissingToken(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	handler := Auth(cfg, stubSessionVerifier{ok: true}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.Code)
	}
}

func TestAuthRejectsInvalidToken(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	handler := Auth(cfg, stubSessionVerifier{ok: true}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.Code)
	}
}

func TestAuthAllowsValidToken(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 60}
	storeID := uuid.New()
	token := mintTestToken(t, cfg, enums.MemberRoleOwner, storeID)

	var captured struct {
		user  string
		role  string
		store string
	}
	handler := Auth(cfg, stubSessionVerifier{ok: true}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.user = UserIDFromContext(r.Context())
		captured.role = RoleFromContext(r.Context())
		captured.store = StoreIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}
	if captured.user == "" {
		t.Fatal("expected user id in context")
	}
	if captured.role != string(enums.MemberRoleOwner) {
		t.Fatalf("expected role owner got %s", captured.role)
	}
	if captured.store != storeID.String() {
		t.Fatalf("expected store %s got %s", storeID, captured.store)
	}
}

func TestAuthAllowsTokenWithoutStore(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 60}
	token := mintTestTokenWithoutStore(t, cfg, enums.MemberRoleAdmin)

	var captured struct {
		user  string
		role  string
		store string
	}
	handler := Auth(cfg, stubSessionVerifier{ok: true}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.user = UserIDFromContext(r.Context())
		captured.role = RoleFromContext(r.Context())
		captured.store = StoreIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}
	if captured.user == "" {
		t.Fatal("expected user id in context")
	}
	if captured.role != string(enums.MemberRoleAdmin) {
		t.Fatalf("expected role admin got %s", captured.role)
	}
	if captured.store != "" {
		t.Fatalf("expected empty store got %s", captured.store)
	}
}

func mintTestToken(t *testing.T, cfg config.JWTConfig, role enums.MemberRole, storeID uuid.UUID) string {
	t.Helper()
	accessID := session.NewAccessID()
	payload := auth.AccessTokenPayload{
		UserID:        uuid.New(),
		ActiveStoreID: &storeID,
		Role:          role,
		JTI:           accessID,
	}
	token, err := auth.MintAccessToken(cfg, time.Now(), payload)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}

func mintTestTokenWithoutStore(t *testing.T, cfg config.JWTConfig, role enums.MemberRole) string {
	t.Helper()
	accessID := session.NewAccessID()
	payload := auth.AccessTokenPayload{
		UserID: uuid.New(),
		Role:   role,
		JTI:    accessID,
	}
	token, err := auth.MintAccessToken(cfg, time.Now(), payload)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}

type stubSessionVerifier struct {
	ok  bool
	err error
}

func (s stubSessionVerifier) HasSession(ctx context.Context, accessID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.ok, nil
}
