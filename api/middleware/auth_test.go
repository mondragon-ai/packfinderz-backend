package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

func TestAuthRejectsTokenErrors(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	handler := Auth(cfg, stubSessionVerifier{ok: true}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	expiredToken := mintExpiredTestToken(t, cfg, enums.MemberRoleOwner, uuid.New())

	tests := []struct {
		name            string
		setup           func(*http.Request)
		expectedMessage string
	}{
		{
			name:            "missing token",
			expectedMessage: "missing credentials",
		},
		{
			name: "invalid token",
			setup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer invalid")
			},
			expectedMessage: "invalid token",
		},
		{
			name: "expired token",
			setup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+expiredToken)
			},
			expectedMessage: "invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			assertErrorResponse(t, resp, http.StatusUnauthorized, pkgerrors.CodeUnauthorized, tt.expectedMessage)
		})
	}
}

func TestAuthRejectsRevokedSession(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 60}
	storeID := uuid.New()
	token := mintTestToken(t, cfg, enums.MemberRoleOwner, storeID)

	handler := Auth(cfg, stubSessionVerifier{ok: false}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	assertErrorResponse(t, resp, http.StatusUnauthorized, pkgerrors.CodeUnauthorized, "session unavailable")
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
	return mintTestTokenAt(t, cfg, role, storeID, time.Now())
}

func mintExpiredTestToken(t *testing.T, cfg config.JWTConfig, role enums.MemberRole, storeID uuid.UUID) string {
	issuedAt := time.Now().Add(-time.Duration(cfg.ExpirationMinutes+1) * time.Minute)
	return mintTestTokenAt(t, cfg, role, storeID, issuedAt)
}

func mintTestTokenAt(t *testing.T, cfg config.JWTConfig, role enums.MemberRole, storeID uuid.UUID, issuedAt time.Time) string {
	t.Helper()
	accessID := session.NewAccessID()
	store := storeID
	payload := auth.AccessTokenPayload{
		UserID:        uuid.New(),
		ActiveStoreID: &store,
		Role:          role,
		JTI:           accessID,
	}
	token, err := auth.MintAccessToken(cfg, issuedAt, payload)
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

func assertErrorResponse(t *testing.T, resp *httptest.ResponseRecorder, expectStatus int, expectCode pkgerrors.Code, expectMessage string) {
	t.Helper()
	if resp.Code != expectStatus {
		t.Fatalf("expected status %d got %d", expectStatus, resp.Code)
	}
	var payload types.ErrorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != string(expectCode) {
		t.Fatalf("expected code %s got %s", expectCode, payload.Error.Code)
	}
	if payload.Error.Message != expectMessage {
		t.Fatalf("expected message %q got %q", expectMessage, payload.Error.Message)
	}
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
