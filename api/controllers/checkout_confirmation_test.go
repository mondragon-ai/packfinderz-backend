package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestCheckoutConfirmationReturnsData(t *testing.T) {
	buyerStoreID := uuid.New()
	groupID := uuid.New()
	order := models.VendorOrder{
		ID:            uuid.New(),
		VendorStoreID: uuid.New(),
		BuyerStoreID:  buyerStoreID,
		Status:        enums.VendorOrderStatusCreatedPending,
		PaymentMethod: enums.PaymentMethodCash,
		PaymentIntent: &models.PaymentIntent{
			ID:          uuid.New(),
			Method:      enums.PaymentMethodCash,
			Status:      enums.PaymentStatusUnpaid,
			AmountCents: 2500,
		},
		Assignments: []models.OrderAssignment{
			{
				ID:          uuid.New(),
				Active:      true,
				AgentUserID: uuid.New(),
				AssignedAt:  time.Now(),
			},
		},
	}
	checkoutGroup := &models.CheckoutGroup{
		ID:           groupID,
		BuyerStoreID: buyerStoreID,
		CartID:       ptrUUID(uuid.New()),
		VendorOrders: []models.VendorOrder{order},
		CartVendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: order.VendorStoreID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 2500,
				TotalCents:    2500,
			},
		},
	}
	handler := CheckoutConfirmation(
		stubCheckoutConfirmationRepo{group: checkoutGroup},
		stubCheckoutStoreService{store: &stores.StoreDTO{ID: buyerStoreID, Type: enums.StoreTypeBuyer}},
		logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/"+groupID.String()+"/confirmation", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), buyerStoreID.String()))
	req = withIdentifier(req, groupID.String())
	req = withIdentifier(req, groupID.String())
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var envelope struct {
		Data checkoutConfirmationResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.CheckoutGroupID != groupID {
		t.Fatalf("unexpected checkout id %s", envelope.Data.CheckoutGroupID)
	}
	if len(envelope.Data.VendorOrders) != 1 {
		t.Fatalf("expected vendor order, got %d", len(envelope.Data.VendorOrders))
	}
}

func TestCheckoutConfirmationReturnsForbiddenForOtherStore(t *testing.T) {
	buyerStoreID := uuid.New()
	groupID := uuid.New()
	otherStoreID := uuid.New()
	handler := CheckoutConfirmation(
		stubCheckoutConfirmationRepo{group: &models.CheckoutGroup{ID: groupID, BuyerStoreID: buyerStoreID}},
		stubCheckoutStoreService{store: &stores.StoreDTO{ID: otherStoreID, Type: enums.StoreTypeBuyer}},
		logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/"+groupID.String()+"/confirmation", nil)
	ctx := middleware.WithStoreID(req.Context(), otherStoreID.String())
	req = withIdentifier(req.WithContext(ctx), groupID.String())
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when store mismatch, got %d", resp.Code)
	}
}

func TestCheckoutConfirmationReturnsNotFoundWhenMissing(t *testing.T) {
	groupID := uuid.New()
	handler := CheckoutConfirmation(
		stubCheckoutConfirmationRepo{},
		stubCheckoutStoreService{store: &stores.StoreDTO{ID: uuid.New(), Type: enums.StoreTypeBuyer}},
		logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/"+groupID.String()+"/confirmation", nil)
	ctx := middleware.WithStoreID(req.Context(), uuid.New().String())
	req = withIdentifier(req.WithContext(ctx), groupID.String())
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when checkout missing, got %d", resp.Code)
	}
}

func TestCheckoutConfirmationRejectsInvalidIdentifier(t *testing.T) {
	handler := CheckoutConfirmation(
		stubCheckoutConfirmationRepo{},
		stubCheckoutStoreService{store: &stores.StoreDTO{ID: uuid.New(), Type: enums.StoreTypeBuyer}},
		logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/not-a-uuid/confirmation", nil)
	ctx := middleware.WithStoreID(req.Context(), uuid.New().String())
	req = withIdentifier(req.WithContext(ctx), "not-a-uuid")
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", resp.Code)
	}
}

type stubCheckoutConfirmationRepo struct {
	group *models.CheckoutGroup
	err   error
}

func (s stubCheckoutConfirmationRepo) WithTx(tx *gorm.DB) checkout.Repository {
	return s
}

func (s stubCheckoutConfirmationRepo) FindByCheckoutGroupID(ctx context.Context, checkoutGroupID uuid.UUID) (*models.CheckoutGroup, error) {
	return s.group, s.err
}

func (s stubCheckoutConfirmationRepo) FindByCartID(ctx context.Context, cartID uuid.UUID) (*models.CheckoutGroup, error) {
	return s.group, s.err
}

type stubCheckoutStoreService struct {
	store *stores.StoreDTO
	err   error
}

func (s stubCheckoutStoreService) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.store, nil
}

func (s stubCheckoutStoreService) GetManagerView(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	return s.GetByID(ctx, id)
}

func (s stubCheckoutStoreService) Update(ctx context.Context, userID, storeID uuid.UUID, input stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	panic("not implemented")
}

func (s stubCheckoutStoreService) ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	panic("not implemented")
}

func (s stubCheckoutStoreService) InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	panic("not implemented")
}

func (s stubCheckoutStoreService) RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error {
	panic("not implemented")
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}

func withIdentifier(req *http.Request, identifier string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add("identifier", identifier)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))
}
