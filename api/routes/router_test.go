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
	"github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	ordersrepo "github.com/angelmondragon/packfinderz-backend/internal/orders"
	product "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

type stubNotificationsService struct {
	listFn func(ctx context.Context, params notifications.ListParams) (*notifications.ListResult, error)
}

func (s stubNotificationsService) List(ctx context.Context, params notifications.ListParams) (*notifications.ListResult, error) {
	if s.listFn != nil {
		return s.listFn(ctx, params)
	}
	return &notifications.ListResult{}, nil
}

func (stubNotificationsService) MarkRead(ctx context.Context, storeID, notificationID uuid.UUID) error {
	return nil
}

func (stubNotificationsService) MarkAllRead(ctx context.Context, storeID uuid.UUID) (int64, error) {
	return 0, nil
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

type stubOrdersRepo struct {
	listBuyer     func(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters ordersrepo.BuyerOrderFilters) (*ordersrepo.BuyerOrderList, error)
	listVendor    func(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters ordersrepo.VendorOrderFilters) (*ordersrepo.VendorOrderList, error)
	payoutList    func(ctx context.Context, params pagination.Params) (*ordersrepo.PayoutOrderList, error)
	queue         func(ctx context.Context, params pagination.Params) (*ordersrepo.AgentOrderQueueList, error)
	assignedQueue func(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*ordersrepo.AgentOrderQueueList, error)
	detail        func(ctx context.Context, orderID uuid.UUID) (*ordersrepo.OrderDetail, error)
}

// FindOrderLineItem implements [orders.Repository].
func (s *stubOrdersRepo) FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error) {
	panic("unimplemented")
}

// UpdateOrderLineItemStatus implements [orders.Repository].
func (s *stubOrdersRepo) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	panic("unimplemented")
}

// UpdateVendorOrder implements [orders.Repository].
func (s *stubOrdersRepo) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	panic("unimplemented")
}

func (s *stubOrdersRepo) UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	panic("unimplemented")
}

func (s *stubOrdersRepo) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	panic("unimplemented")
}

func (s *stubOrdersRepo) WithTx(tx *gorm.DB) ordersrepo.Repository {
	return s
}

func (s *stubOrdersRepo) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	panic("unimplemented")
}

func (s *stubOrdersRepo) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters ordersrepo.BuyerOrderFilters) (*ordersrepo.BuyerOrderList, error) {
	if s.listBuyer != nil {
		return s.listBuyer(ctx, buyerStoreID, params, filters)
	}
	return nil, nil
}

func (s *stubOrdersRepo) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters ordersrepo.VendorOrderFilters) (*ordersrepo.VendorOrderList, error) {
	if s.listVendor != nil {
		return s.listVendor(ctx, vendorStoreID, params, filters)
	}
	return nil, nil
}

func (s *stubOrdersRepo) ListPayoutOrders(ctx context.Context, params pagination.Params) (*ordersrepo.PayoutOrderList, error) {
	if s.payoutList != nil {
		return s.payoutList(ctx, params)
	}
	return &ordersrepo.PayoutOrderList{}, nil
}

func (s *stubOrdersRepo) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*ordersrepo.AgentOrderQueueList, error) {
	if s.queue != nil {
		return s.queue(ctx, params)
	}
	return &ordersrepo.AgentOrderQueueList{}, nil
}

func (s *stubOrdersRepo) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*ordersrepo.AgentOrderQueueList, error) {
	if s.assignedQueue != nil {
		return s.assignedQueue(ctx, agentID, params)
	}
	return &ordersrepo.AgentOrderQueueList{}, nil
}

func (s *stubOrdersRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*ordersrepo.OrderDetail, error) {
	if s.detail != nil {
		return s.detail(ctx, orderID)
	}
	return nil, nil
}

func (s *stubOrdersRepo) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubOrdersRepo) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	return nil
}

type stubOrdersService struct {
	decision     func(ctx context.Context, input ordersrepo.VendorDecisionInput) error
	agentPickup  func(ctx context.Context, input ordersrepo.AgentPickupInput) error
	agentDeliver func(ctx context.Context, input ordersrepo.AgentDeliverInput) error
}

// CancelOrder implements [orders.Service].
func (s stubOrdersService) CancelOrder(ctx context.Context, input ordersrepo.BuyerCancelInput) error {
	panic("unimplemented")
}

// NudgeVendor implements [orders.Service].
func (s stubOrdersService) NudgeVendor(ctx context.Context, input ordersrepo.BuyerNudgeInput) error {
	panic("unimplemented")
}

// RetryOrder implements [orders.Service].
func (s stubOrdersService) RetryOrder(ctx context.Context, input ordersrepo.BuyerRetryInput) (*ordersrepo.BuyerRetryResult, error) {
	panic("unimplemented")
}

// LineItemDecision implements [orders.Service].
func (s stubOrdersService) LineItemDecision(ctx context.Context, input ordersrepo.LineItemDecisionInput) error {
	panic("unimplemented")
}

func (s stubOrdersService) VendorDecision(ctx context.Context, input ordersrepo.VendorDecisionInput) error {
	if s.decision != nil {
		return s.decision(ctx, input)
	}
	return nil
}

func (s stubOrdersService) AgentPickup(ctx context.Context, input ordersrepo.AgentPickupInput) error {
	if s.agentPickup != nil {
		return s.agentPickup(ctx, input)
	}
	return nil
}

func (s stubOrdersService) AgentDeliver(ctx context.Context, input ordersrepo.AgentDeliverInput) error {
	if s.agentDeliver != nil {
		return s.agentDeliver(ctx, input)
	}
	return nil
}

func (s stubOrdersService) ConfirmPayout(ctx context.Context, input ordersrepo.ConfirmPayoutInput) error {
	return nil
}

type stubCheckoutService struct{}

// Execute implements [checkout.Service].
func (s stubCheckoutService) Execute(ctx context.Context, buyerStoreID uuid.UUID, cartID uuid.UUID, input checkout.CheckoutInput) (*models.CheckoutGroup, error) {
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
		stubCheckoutService{},
		stubCartService{},
		stubNotificationsService{},
		&stubOrdersRepo{},
		stubOrdersService{},
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

	nonAgent := httptest.NewRequest(http.MethodGet, "/api/v1/agent/ping", nil)
	nonAgent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleViewer))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, nonAgent)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-agent got %d", resp.Code)
	}

	agent := httptest.NewRequest(http.MethodGet, "/api/v1/agent/ping", nil)
	agent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAgent))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, agent)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent got %d", resp.Code)
	}
}

func TestAgentOrderQueueRequiresAgentRole(t *testing.T) {
	cfg := testConfig()
	router := newTestRouter(cfg)

	nonAgent := httptest.NewRequest(http.MethodGet, "/api/v1/agent/orders/queue", nil)
	nonAgent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleViewer))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, nonAgent)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-agent queue got %d", resp.Code)
	}

	agent := httptest.NewRequest(http.MethodGet, "/api/v1/agent/orders/queue", nil)
	agent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAgent))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, agent)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent queue got %d", resp.Code)
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

func TestAgentAssignedOrdersRequiresAgentRole(t *testing.T) {
	cfg := testConfig()
	repo := &stubOrdersRepo{
		assignedQueue: func(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*ordersrepo.AgentOrderQueueList, error) {
			return &ordersrepo.AgentOrderQueueList{
				Orders: []ordersrepo.AgentOrderQueueSummary{
					{OrderID: uuid.New(), OrderNumber: 101},
				},
			}, nil
		},
	}
	logg := logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard})
	router := NewRouter(
		cfg,
		logg,
		stubPinger{},
		(*redis.Client)(nil),
		stubPinger{},
		stubSessionManager{},
		stubAuthService{},
		stubRegisterService{},
		stubSwitchService{},
		stubStoreService{},
		stubMediaService{},
		stubLicensesService{},
		stubProductService{},
		stubCheckoutService{},
		stubCartService{},
		stubNotificationsService{},
		repo,
		stubOrdersService{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/orders", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when missing token got %d", resp.Code)
	}

	nonAgent := httptest.NewRequest(http.MethodGet, "/api/v1/agent/orders", nil)
	nonAgent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleViewer))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, nonAgent)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-agent orders got %d", resp.Code)
	}

	agent := httptest.NewRequest(http.MethodGet, "/api/v1/agent/orders", nil)
	agent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAgent))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, agent)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent assigned orders got %d", resp.Code)
	}
}

func TestAgentAssignedOrderDetailRequiresAgentRole(t *testing.T) {
	cfg := testConfig()
	expectedAgent := uuid.New()
	repo := &stubOrdersRepo{
		detail: func(ctx context.Context, orderID uuid.UUID) (*ordersrepo.OrderDetail, error) {
			return &ordersrepo.OrderDetail{
				ActiveAssignment: &ordersrepo.OrderAssignmentSummary{
					AgentUserID: expectedAgent,
				},
			}, nil
		},
	}
	logg := logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard})
	router := NewRouter(
		cfg,
		logg,
		stubPinger{},
		(*redis.Client)(nil),
		stubPinger{},
		stubSessionManager{},
		stubAuthService{},
		stubRegisterService{},
		stubSwitchService{},
		stubStoreService{},
		stubMediaService{},
		stubLicensesService{},
		stubProductService{},
		stubCheckoutService{},
		stubCartService{},
		stubNotificationsService{},
		repo,
		stubOrdersService{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/orders/"+uuid.NewString(), nil)
	req.Header.Set("Authorization", "Bearer "+buildTokenWithUserID(t, cfg, enums.MemberRoleAgent, expectedAgent))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for assigned order detail got %d", resp.Code)
	}
}

func TestAgentPickupRequiresAgentRole(t *testing.T) {
	cfg := testConfig()
	repo := &stubOrdersRepo{}
	logg := logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard})
	router := NewRouter(
		cfg,
		logg,
		stubPinger{},
		(*redis.Client)(nil),
		stubPinger{},
		stubSessionManager{},
		stubAuthService{},
		stubRegisterService{},
		stubSwitchService{},
		stubStoreService{},
		stubMediaService{},
		stubLicensesService{},
		stubProductService{},
		stubCheckoutService{},
		stubCartService{},
		stubNotificationsService{},
		repo,
		stubOrdersService{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/orders/"+uuid.NewString()+"/pickup", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when missing token got %d", resp.Code)
	}

	nonAgent := httptest.NewRequest(http.MethodPost, "/api/v1/agent/orders/"+uuid.NewString()+"/pickup", nil)
	nonAgent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleViewer))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, nonAgent)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-agent pickup got %d", resp.Code)
	}

	agent := httptest.NewRequest(http.MethodPost, "/api/v1/agent/orders/"+uuid.NewString()+"/pickup", nil)
	agent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAgent))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, agent)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent pickup got %d", resp.Code)
	}
}

func TestAgentDeliverRequiresAgentRole(t *testing.T) {
	cfg := testConfig()
	repo := &stubOrdersRepo{}
	logg := logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard})
	router := NewRouter(
		cfg,
		logg,
		stubPinger{},
		(*redis.Client)(nil),
		stubPinger{},
		stubSessionManager{},
		stubAuthService{},
		stubRegisterService{},
		stubSwitchService{},
		stubStoreService{},
		stubMediaService{},
		stubLicensesService{},
		stubProductService{},
		stubCheckoutService{},
		stubCartService{},
		stubNotificationsService{},
		repo,
		stubOrdersService{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/orders/"+uuid.NewString()+"/deliver", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when missing token got %d", resp.Code)
	}

	nonAgent := httptest.NewRequest(http.MethodPost, "/api/v1/agent/orders/"+uuid.NewString()+"/deliver", nil)
	nonAgent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleViewer))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, nonAgent)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-agent deliver got %d", resp.Code)
	}

	agent := httptest.NewRequest(http.MethodPost, "/api/v1/agent/orders/"+uuid.NewString()+"/deliver", nil)
	agent.Header.Set("Authorization", "Bearer "+buildToken(t, cfg, enums.MemberRoleAgent))
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, agent)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent deliver got %d", resp.Code)
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
	storeType := enums.StoreTypeBuyer
	token, err := pkgAuth.MintAccessToken(cfg.JWT, time.Now(), pkgAuth.AccessTokenPayload{
		UserID:        uuid.New(),
		ActiveStoreID: &storeID,
		Role:          role,
		StoreType:     &storeType,
		JTI:           accessID,
	})
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}

func buildTokenWithUserID(t *testing.T, cfg *config.Config, role enums.MemberRole, userID uuid.UUID) string {
	t.Helper()
	storeID := uuid.New()
	accessID := session.NewAccessID()
	storeType := enums.StoreTypeBuyer
	token, err := pkgAuth.MintAccessToken(cfg.JWT, time.Now(), pkgAuth.AccessTokenPayload{
		UserID:        userID,
		ActiveStoreID: &storeID,
		Role:          role,
		StoreType:     &storeType,
		JTI:           accessID,
	})
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}
