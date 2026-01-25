package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	product "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/google/uuid"
)

type stubPinger struct{}

func (stubPinger) Ping(context.Context) error {
	return nil
}

type stubAuthService struct{}

func (stubAuthService) Login(ctx context.Context, req auth.LoginRequest) (*auth.LoginResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

type stubRegisterService struct{}

func (stubRegisterService) Register(ctx context.Context, req auth.RegisterRequest) error {
	return nil
}

type stubSessionManager struct{}

func (stubSessionManager) HasSession(ctx context.Context, accessID string) (bool, error) {
	return true, nil
}

func (stubSessionManager) Rotate(ctx context.Context, oldAccessID, provided string) (string, string, error) {
	return "", "", nil
}

func (stubSessionManager) Revoke(ctx context.Context, accessID string) error {
	return nil
}

type stubSwitchService struct{}

func (stubSwitchService) Switch(ctx context.Context, input auth.SwitchStoreInput) (*auth.SwitchStoreResult, error) {
	return nil, nil
}

type stubMediaService struct{}

// DeleteMedia implements [media.Service].
func (s stubMediaService) DeleteMedia(ctx context.Context, params media.DeleteMediaParams) error {
	panic("unimplemented")
}

// GenerateReadURL implements [media.Service].
func (s stubMediaService) GenerateReadURL(ctx context.Context, params media.ReadURLParams) (*media.ReadURLOutput, error) {
	panic("unimplemented")
}

// ListMedia implements [media.Service].
func (s stubMediaService) ListMedia(ctx context.Context, params media.ListParams) (*media.ListResult, error) {
	panic("unimplemented")
}

func (stubMediaService) PresignUpload(ctx context.Context, userID, storeID uuid.UUID, input media.PresignInput) (*media.PresignOutput, error) {
	return &media.PresignOutput{}, nil
}

type stubStoreService struct{}

// GetByID implements [stores.Service].
func (s stubStoreService) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	panic("unimplemented")
}

// InviteUser implements [stores.Service].
func (s stubStoreService) InviteUser(ctx context.Context, inviterID uuid.UUID, storeID uuid.UUID, input stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	panic("unimplemented")
}

// ListUsers implements [stores.Service].
func (s stubStoreService) ListUsers(ctx context.Context, userID uuid.UUID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	panic("unimplemented")
}

// RemoveUser implements [stores.Service].
func (s stubStoreService) RemoveUser(ctx context.Context, actorID uuid.UUID, storeID uuid.UUID, targetUserID uuid.UUID) error {
	panic("unimplemented")
}

// Update implements [stores.Service].
func (s stubStoreService) Update(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, input stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	panic("unimplemented")
}

type stubLicensesService struct{}

// ListLicenses implements [licenses.Service].
func (s stubLicensesService) ListLicenses(ctx context.Context, params licenses.ListParams) (*licenses.ListResult, error) {
	panic("unimplemented")
}

// CreateLicense implements [licenses.Service].
func (s stubLicensesService) CreateLicense(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, input licenses.CreateLicenseInput) (*models.License, error) {
	panic("unimplemented")
}

// DeleteLicense implements [licenses.Service].
func (s stubLicensesService) DeleteLicense(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, licenseID uuid.UUID) error {
	panic("unimplemented")
}

// VerifyLicense implements [licenses.Service].
func (s stubLicensesService) VerifyLicense(ctx context.Context, licenseID uuid.UUID, decision enums.LicenseStatus, reason string) (*models.License, error) {
	panic("unimplemented")
}

type stubProductService struct{}

// CreateProduct implements [product.Service].
func (s stubProductService) CreateProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, input product.CreateProductInput) (*product.ProductDTO, error) {
	panic("unimplemented")
}

// UpdateProduct implements [product.Service].
func (s stubProductService) UpdateProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, productID uuid.UUID, input product.UpdateProductInput) (*product.ProductDTO, error) {
	panic("unimplemented")
}

// DeleteProduct implements [product.Service].
func (s stubProductService) DeleteProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, productID uuid.UUID) error {
	panic("unimplemented")
}

type stubCartService struct{}

// UpsertCart implements [cart.Service].
func (s stubCartService) UpsertCart(ctx context.Context, buyerStoreID uuid.UUID, input cart.UpsertCartInput) (*models.CartRecord, error) {
	panic("unimplemented")
}

// GetActiveCart implements [cart.Service].
func (s stubCartService) GetActiveCart(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	panic("unimplemented")
}

func testConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{Env: "test", Port: "0"},
		JWT: config.JWTConfig{
			Secret:                 "secret",
			Issuer:                 "issuer",
			ExpirationMinutes:      60,
			RefreshTokenTTLMinutes: 120,
		},
	}
}

func newTestRouter(cfg *config.Config) http.Handler {
	logg := logger.New(logger.Options{ServiceName: "test-routing", Level: logger.ParseLevel("debug"), Output: io.Discard})
	return NewRouter(
		cfg,
		logg,
		stubPinger{},          // db.Pinger
		(*redis.Client)(nil),  // *redis.Client
		stubPinger{},          // gcs.Pinger
		stubSessionManager{},  // sessionManager
		stubAuthService{},     // auth.Service
		stubRegisterService{}, // auth.RegisterService
		stubSwitchService{},   // auth.SwitchStoreService
		stubStoreService{},
		stubMediaService{},
		stubLicensesService{},
		stubProductService{},
		stubCartService{},
	)
}

func TestPrivateGroupRejectsMissingJWT(t *testing.T) {
	router := newTestRouter(testConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token got %d", resp.Code)
	}
}

func TestAdminGroupRequiresAdminRole(t *testing.T) {
	cfg := testConfig()
	router := newTestRouter(cfg)

	nonAdmin := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	nonAdmin.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleViewer))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, nonAdmin)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin got %d", resp.Code)
	}

	admin := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	admin.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAdmin))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, admin)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin got %d", resp.Code)
	}
}

func TestAgentGroupRequiresAgentRole(t *testing.T) {
	cfg := testConfig()
	router := newTestRouter(cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/agent/ping", nil)
	req.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAgent))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent got %d", resp.Code)
	}
}

func TestPrivateGroupSucceedsWithJWT(t *testing.T) {
	cfg := testConfig()
	router := newTestRouter(cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleOwner))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for private ping got %d", resp.Code)
	}
}

func TestPublicValidateRejectsBadJSON(t *testing.T) {
	router := newTestRouter(testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/public/validate", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid payload got %d", resp.Code)
	}
}

func TestPublicValidateAcceptsGoodJSON(t *testing.T) {
	router := newTestRouter(testConfig())
	body := `{"name":"Zed","email":"zed@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid payload got %d", resp.Code)
	}
}

func buildToken(t *testing.T, cfg *config.Config, role enums.MemberRole) string {
	t.Helper()
	storeID := uuid.New()
	accessID := session.NewAccessID()
	token, err := pkgAuth.MintAccessToken(cfg.JWT, time.Now(), pkgAuth.AccessTokenPayload{
		UserID:        uuid.New(),
		ActiveStoreID: &storeID,
		Role:          role,
		JTI:           accessID,
	})
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}
