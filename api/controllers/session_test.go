package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

type stubSessionTokenManager struct {
	lastRevoked    string
	lastRotateOld  string
	lastRotateBody string
	rotateRespID   string
	rotateRespTok  string
	rotateErr      error
	revokeErr      error
}

func (s *stubSessionTokenManager) Rotate(ctx context.Context, oldAccessID, provided string) (string, string, error) {
	s.lastRotateOld = oldAccessID
	s.lastRotateBody = provided
	return s.rotateRespID, s.rotateRespTok, s.rotateErr
}

func (s *stubSessionTokenManager) Revoke(ctx context.Context, accessID string) error {
	s.lastRevoked = accessID
	return s.revokeErr
}

func mintTestToken(t *testing.T, cfg config.JWTConfig, role enums.MemberRole) (string, string) {
	t.Helper()
	accessID := session.NewAccessID()
	token, err := auth.MintAccessToken(cfg, time.Now(), auth.AccessTokenPayload{
		UserID:        uuid.New(),
		ActiveStoreID: nil,
		Role:          role,
		JTI:           accessID,
	})
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}
	return token, accessID
}

func TestAuthLogout(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	manager := &stubSessionTokenManager{}
	handler := AuthLogout(manager, cfg, nil)

	token, jti := mintTestToken(t, cfg, enums.MemberRoleOwner)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if manager.lastRevoked != jti {
		t.Fatalf("expected revoked %s got %s", jti, manager.lastRevoked)
	}
}

func TestAuthRefresh(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	manager := &stubSessionTokenManager{
		rotateRespID:  "new-jti",
		rotateRespTok: "new-refresh",
	}
	handler := AuthRefresh(manager, cfg, nil)

	token, jti := mintTestToken(t, cfg, enums.MemberRoleOwner)
	body := `{"refresh_token":"old-refresh"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if manager.lastRotateOld != jti {
		t.Fatalf("expected rotate old %s got %s", jti, manager.lastRotateOld)
	}
	var envelope struct {
		Data refreshResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.RefreshToken != "new-refresh" {
		t.Fatalf("expected refresh token new-refresh got %s", envelope.Data.RefreshToken)
	}
	if envelope.Data.AccessToken == "" {
		t.Fatalf("expected access token in body")
	}
	if rec.Header().Get("X-PF-Token") != envelope.Data.AccessToken {
		t.Fatalf("expected header token match body token")
	}
}

func TestAuthRefreshInvalidToken(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	manager := &stubSessionTokenManager{
		rotateErr: session.ErrInvalidRefreshToken,
	}
	handler := AuthRefresh(manager, cfg, nil)

	token, _ := mintTestToken(t, cfg, enums.MemberRoleOwner)
	body := `{"refresh_token":"old-refresh"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}
