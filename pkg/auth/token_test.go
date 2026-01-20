package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

func TestMintAndParseAccessToken(t *testing.T) {
	cfg := config.JWTConfig{
		Secret:            "secret",
		Issuer:            "packfinderz",
		ExpirationMinutes: 30,
	}
	now := time.Now().UTC()
	userID := uuid.New()
	storeID := uuid.New()
	storeType := enums.StoreTypeVendor
	kyc := enums.KYCStatusVerified

	payload := AccessTokenPayload{
		UserID:        userID,
		ActiveStoreID: &storeID,
		Role:          enums.MemberRoleOwner,
		StoreType:     &storeType,
		KYCStatus:     &kyc,
	}

	token, err := MintAccessToken(cfg, now, payload)
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}

	claims, err := ParseAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected user_id %s, got %s", userID, claims.UserID)
	}
	if claims.ActiveStoreID == nil || *claims.ActiveStoreID != storeID {
		t.Fatalf("active store id not preserved")
	}
	if claims.Role != enums.MemberRoleOwner {
		t.Fatalf("unexpected role %s", claims.Role)
	}
	if claims.StoreType == nil || *claims.StoreType != storeType {
		t.Fatalf("store type mismatch")
	}
	if claims.KYCStatus == nil || *claims.KYCStatus != kyc {
		t.Fatalf("kyc status mismatch")
	}

	// RegisteredClaims is embedded, so access fields directly.
	if claims.Issuer != cfg.Issuer {
		t.Fatalf("expected issuer %s, got %s", cfg.Issuer, claims.Issuer)
	}

	exp := now.Add(time.Duration(cfg.ExpirationMinutes) * time.Minute)
	diff := claims.ExpiresAt.Sub(exp) // embedded time.Time (no .Time needed)
	if diff < 0 {
		diff = -diff
	}
	if diff >= time.Second {
		t.Fatalf("expected exp roughly %v, got %v (diff %v)", exp.UTC(), claims.ExpiresAt.UTC(), diff) // embedded time.Time
	}
}

func TestParseAccessTokenInvalidSignature(t *testing.T) {
	cfg := config.JWTConfig{
		Secret:            "secret",
		Issuer:            "packfinderz",
		ExpirationMinutes: 10,
	}
	now := time.Now()
	payload := AccessTokenPayload{
		UserID: uuid.New(),
		Role:   enums.MemberRoleManager,
	}

	token, err := MintAccessToken(cfg, now, payload)
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}

	_, err = ParseAccessToken(cfg, token+"x")
	if err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestParseAccessTokenExpired(t *testing.T) {
	cfg := config.JWTConfig{
		Secret:            "secret",
		Issuer:            "packfinderz",
		ExpirationMinutes: 15,
	}
	now := time.Now().Add(-time.Hour)
	payload := AccessTokenPayload{
		UserID: uuid.New(),
		Role:   enums.MemberRoleStaff,
	}

	token, err := MintAccessToken(cfg, now, payload)
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}

	_, err = ParseAccessToken(cfg, token)
	if err == nil {
		t.Fatal("expected expiration error")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMintAccessTokenInvalidRole(t *testing.T) {
	cfg := config.JWTConfig{
		Secret:            "secret",
		Issuer:            "packfinderz",
		ExpirationMinutes: 5,
	}
	now := time.Now()
	payload := AccessTokenPayload{
		UserID: uuid.New(),
		Role:   "",
	}

	if _, err := MintAccessToken(cfg, now, payload); err == nil {
		t.Fatal("expected invalid role error")
	}
}
