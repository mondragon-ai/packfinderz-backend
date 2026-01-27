package auth

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
	"github.com/google/uuid"
)

func TestServiceLoginAgentSystemRole(t *testing.T) {
	password := "agent-secret"
	hashed := mustHashPassword(t, password)
	user := &models.User{
		ID:           uuid.New(),
		Email:        "agent@example.com",
		PasswordHash: hashed,
		FirstName:    "Agent",
		LastName:     "Runner",
		IsActive:     true,
		SystemRole:   strPtr("agent"),
	}
	cfg := config.JWTConfig{
		Secret:            "secret",
		Issuer:            "packfinderz",
		ExpirationMinutes: 30,
	}

	svc, _, err := buildTestService(user, nil, cfg)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	resp, err := svc.Login(context.Background(), LoginRequest{
		Email:    user.Email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	claims, err := pkgAuth.ParseAccessToken(cfg, resp.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if claims.Role != enums.MemberRoleAgent {
		t.Fatalf("expected agent role claim, got %s", claims.Role)
	}
	if len(resp.Stores) != 0 {
		t.Fatalf("expected no stores for agent, got %d", len(resp.Stores))
	}
	if resp.RefreshToken == "" {
		t.Fatalf("expected refresh token to be set")
	}
}

func TestServiceLoginRequiresMembershipWithoutSystemRole(t *testing.T) {
	password := "no-role"
	hashed := mustHashPassword(t, password)
	user := &models.User{
		ID:           uuid.New(),
		Email:        "no-role@example.com",
		PasswordHash: hashed,
		FirstName:    "NoRole",
		LastName:     "User",
		IsActive:     true,
	}
	cfg := config.JWTConfig{
		Secret:            "secret",
		Issuer:            "packfinderz",
		ExpirationMinutes: 30,
	}

	svc, _, err := buildTestService(user, nil, cfg)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	_, err = svc.Login(context.Background(), LoginRequest{
		Email:    user.Email,
		Password: password,
	})
	if err == nil {
		t.Fatalf("expected unauthorized without membership")
	}
	typed := pkgerrors.As(err)
	if typed == nil || typed.Code() != pkgerrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func buildTestService(user *models.User, stores []memberships.MembershipWithStore, jwtCfg config.JWTConfig) (Service, *stubSessionManager, error) {
	userRepo := stubUserRepo{user: user}
	membershipRepo := stubMembershipsRepo{stores: stores}
	sessionMgr := &stubSessionManager{refreshToken: "refresh-token"}
	svc, err := NewService(ServiceParams{
		UserRepo:        userRepo,
		MembershipsRepo: membershipRepo,
		SessionManager:  sessionMgr,
		JWTConfig:       jwtCfg,
	})
	return svc, sessionMgr, err
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := security.HashPassword(password, config.PasswordConfig{})
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return hash
}

func strPtr(value string) *string {
	return &value
}

type stubUserRepo struct {
	user *models.User
	err  error
}

func (s stubUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

func (s stubUserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	if s.user != nil && s.user.ID == id {
		s.user.LastLoginAt = &at
	}
	return nil
}

type stubMembershipsRepo struct {
	stores []memberships.MembershipWithStore
	err    error
}

func (s stubMembershipsRepo) ListUserStores(ctx context.Context, userID uuid.UUID) ([]memberships.MembershipWithStore, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.stores, nil
}

type stubSessionManager struct {
	refreshToken string
}

func (s *stubSessionManager) Generate(ctx context.Context, accessID string) (string, error) {
	return s.refreshToken, nil
}
