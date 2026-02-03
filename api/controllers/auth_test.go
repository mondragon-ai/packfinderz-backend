package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
	"github.com/google/uuid"
)

type stubAdminRegisterService struct {
	user *users.UserDTO
	err  error
}

func (s stubAdminRegisterService) Register(ctx context.Context, req auth.AdminRegisterRequest) (*users.UserDTO, error) {
	return s.user, s.err
}

type stubAdminAuthService struct {
	resp *auth.AdminLoginResponse
	err  error
}

func (s stubAdminAuthService) Login(ctx context.Context, req auth.LoginRequest) (*auth.LoginResponse, error) {
	return nil, s.err
}

func (s stubAdminAuthService) AdminLogin(ctx context.Context, req auth.LoginRequest) (*auth.AdminLoginResponse, error) {
	return s.resp, s.err
}

func TestAdminAuthRegisterSuccess(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Env: "dev", Port: "0"}}
	user := &models.User{
		ID:         uuid.New(),
		Email:      "admin@example.com",
		FirstName:  "Admin",
		LastName:   "User",
		IsActive:   true,
		SystemRole: strPtr("admin"),
	}
	user.PasswordHash = mustHashPassword(t)

	handler := AdminAuthRegister(
		stubAdminRegisterService{user: users.FromModel(user)},
		stubAdminAuthService{resp: &auth.AdminLoginResponse{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			User:         users.FromModel(user),
		}},
		cfg,
		nil,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/auth/register", bytes.NewReader([]byte(`{"first_names":"Admin","last_name":"User","email":"admin@example.com","password":"Secret#1"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", resp.Code)
	}
	if got := resp.Header().Get("X-PF-Token"); got != "access-token" {
		t.Fatalf("expected x-pf-token header set to access-token got %s", got)
	}

	var envelope struct {
		Data struct {
			AccessToken  string         `json:"access_token"`
			RefreshToken string         `json:"refresh_token"`
			User         *users.UserDTO `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.User == nil || envelope.Data.User.Email != user.Email {
		t.Fatalf("expected user in payload got %+v", envelope.Data.User)
	}
}

func TestAdminAuthRegisterInvalidPayload(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Env: "dev", Port: "0"}}
	handler := AdminAuthRegister(stubAdminRegisterService{}, stubAuthService{}, cfg, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/auth/register", bytes.NewReader([]byte(`{"password":"Secret#1"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 422 got %d", resp.Code)
	}
}

func TestAdminAuthRegisterDisabledInProd(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Env: "prod", Port: "0"}}
	handler := AdminAuthRegister(nil, nil, cfg, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/auth/register", bytes.NewReader([]byte(`{"email":"admin@example.com","password":"Secret#1"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}

func mustHashPassword(t *testing.T) string {
	t.Helper()
	hash, err := security.HashPassword("Secret#1", config.PasswordConfig{})
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return hash
}

func strPtr(value string) *string {
	return &value
}
