package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	authpkg "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

type stubSwitchService struct {
	lastInput auth.SwitchStoreInput
	result    *auth.SwitchStoreResult
	err       error
}

func (s *stubSwitchService) Switch(ctx context.Context, input auth.SwitchStoreInput) (*auth.SwitchStoreResult, error) {
	s.lastInput = input
	return s.result, s.err
}

func TestAuthSwitchStoreSuccess(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	token, _ := authpkg.MintAccessToken(cfg, time.Now(), authpkg.AccessTokenPayload{
		UserID: uuid.New(),
		Role:   enums.MemberRoleOwner,
		JTI:    session.NewAccessID(),
	})
	storeID := uuid.New()
	service := &stubSwitchService{
		result: &auth.SwitchStoreResult{
			AccessToken:  "new-token",
			RefreshToken: "new-refresh",
			Store: auth.StoreSummary{
				ID:   storeID,
				Name: "Store",
				Type: enums.StoreTypeBuyer,
			},
		},
	}

	body := []byte(`{"store_id":"` + storeID.String() + `","refresh_token":"ref"}`)
	req := httptest.NewRequest(http.MethodPost, "/switch-store", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := AuthSwitchStore(service, cfg, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if rec.Header().Get("X-PF-Token") != "new-token" {
		t.Fatalf("expected header token new-token got %s", rec.Header().Get("X-PF-Token"))
	}
	var envelope struct {
		Data auth.SwitchStoreResult `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected refresh token %s", envelope.Data.RefreshToken)
	}
}

func TestAuthSwitchStoreForbidden(t *testing.T) {
	cfg := config.JWTConfig{Secret: "secret", Issuer: "issuer", ExpirationMinutes: 10}
	token, _ := authpkg.MintAccessToken(cfg, time.Now(), authpkg.AccessTokenPayload{
		UserID: uuid.New(),
		Role:   enums.MemberRoleOwner,
		JTI:    session.NewAccessID(),
	})
	service := &stubSwitchService{err: pkgerrors.New(pkgerrors.CodeForbidden, "no membership")}

	body := []byte(`{"store_id":"` + uuid.NewString() + `","refresh_token":"ref"}`)
	req := httptest.NewRequest(http.MethodPost, "/switch-store", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := AuthSwitchStore(service, cfg, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", rec.Code)
	}
}
