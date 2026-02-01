package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

type stubCheckoutService struct {
	group *models.CheckoutGroup
	err   error
}

func (s stubCheckoutService) Execute(ctx context.Context, buyerStoreID, cartID uuid.UUID, input checkoutsvc.CheckoutInput) (*models.CheckoutGroup, error) {
	return s.group, s.err
}

type checkoutStubStoreService struct {
	store *stores.StoreDTO
	err   error
}

func (s checkoutStubStoreService) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	return s.store, s.err
}

func (checkoutStubStoreService) Update(ctx context.Context, userID, storeID uuid.UUID, input stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	return nil, pkgerrors.New(pkgerrors.CodeInternal, "not implemented")
}

func (checkoutStubStoreService) ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	return nil, pkgerrors.New(pkgerrors.CodeInternal, "not implemented")
}

func (checkoutStubStoreService) InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	return nil, "", pkgerrors.New(pkgerrors.CodeInternal, "not implemented")
}

func (checkoutStubStoreService) RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error {
	return pkgerrors.New(pkgerrors.CodeInternal, "not implemented")
}

func TestCheckoutSuccess(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	vendorID := uuid.New()
	productID := uuid.New()
	lineRejected := uuid.New()

	group := &models.CheckoutGroup{
		ID: uuid.New(),
		VendorOrders: []models.VendorOrder{
			{
				ID:            uuid.New(),
				VendorStoreID: vendorID,
				Status:        enums.VendorOrderStatusCreatedPending,
				ShippingAddress: &types.Address{
					Line1:      "123 Market St",
					City:       "Tulsa",
					State:      "OK",
					PostalCode: "74104",
					Country:    "US",
					Lat:        36.15,
					Lng:        -95.99,
				},
				PaymentMethod: enums.PaymentMethodCash,
				ShippingLine: &types.ShippingLine{
					Code:       "express",
					Title:      "Express",
					PriceCents: 500,
				},
				SubtotalCents:     3000,
				DiscountsCents:    200,
				TaxCents:          0,
				TransportFeeCents: 0,
				TotalCents:        2800,
				BalanceDueCents:   2800,
				Items: []models.OrderLineItem{
					{
						ID:             uuid.New(),
						ProductID:      &productID,
						Name:           "Accepted",
						Unit:           enums.ProductUnitUnit,
						Qty:            2,
						UnitPriceCents: 1000,
						DiscountCents:  100,
						TotalCents:     1900,
						Status:         enums.LineItemStatusPending,
					},
					{
						ID:             lineRejected,
						ProductID:      &productID,
						Name:           "Rejected",
						Unit:           enums.ProductUnitUnit,
						Qty:            1,
						UnitPriceCents: 1000,
						DiscountCents:  0,
						TotalCents:     1000,
						Status:         enums.LineItemStatusRejected,
						Notes:          ptrString("insufficient_inventory"),
					},
				},
			},
		},
	}

	handler := Checkout(
		stubCheckoutService{group: group},
		checkoutStubStoreService{store: &stores.StoreDTO{ID: storeID, Type: enums.StoreTypeBuyer}},
		nil,
	)

	payloadCartID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", strings.NewReader(validCheckoutRequest(payloadCartID)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-key")
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", resp.Code)
	}

	var envelope struct {
		Data checkoutResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.CheckoutGroupID != group.ID {
		t.Fatalf("unexpected checkout group id: %s", envelope.Data.CheckoutGroupID)
	}
	if len(envelope.Data.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(envelope.Data.VendorOrders))
	}
	if len(envelope.Data.RejectedVendors) != 1 {
		t.Fatalf("expected rejected vendor, got %d", len(envelope.Data.RejectedVendors))
	}
	if envelope.Data.RejectedVendors[0].VendorStoreID != vendorID {
		t.Fatalf("unexpected rejected vendor: %s", envelope.Data.RejectedVendors[0].VendorStoreID)
	}
	if len(envelope.Data.RejectedVendors[0].LineItems) != 1 {
		t.Fatalf("expected 1 rejected line item, got %d", len(envelope.Data.RejectedVendors[0].LineItems))
	}
	if envelope.Data.RejectedVendors[0].LineItems[0].LineItemID != lineRejected {
		t.Fatalf("unexpected rejected line item id: %s", envelope.Data.RejectedVendors[0].LineItems[0].LineItemID)
	}
	if envelope.Data.ShippingAddress == nil || envelope.Data.ShippingAddress.Line1 != "123 Market St" {
		t.Fatalf("unexpected shipping address: %+v", envelope.Data.ShippingAddress)
	}
	if envelope.Data.PaymentMethod != enums.PaymentMethodCash {
		t.Fatalf("unexpected payment method: %s", envelope.Data.PaymentMethod)
	}
	if envelope.Data.ShippingLine == nil || envelope.Data.ShippingLine.Code != "express" {
		t.Fatalf("unexpected shipping line: %+v", envelope.Data.ShippingLine)
	}
}

func TestCheckoutRequiresBuyerStore(t *testing.T) {
	storeID := uuid.New()
	handler := Checkout(
		stubCheckoutService{},
		checkoutStubStoreService{store: &stores.StoreDTO{ID: storeID, Type: enums.StoreTypeVendor}},
		nil,
	)

	payloadCartID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", strings.NewReader(validCheckoutRequest(payloadCartID)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-key")
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}

func TestCheckoutValidationError(t *testing.T) {
	storeID := uuid.New()
	handler := Checkout(
		stubCheckoutService{},
		checkoutStubStoreService{store: &stores.StoreDTO{ID: storeID, Type: enums.StoreTypeBuyer}},
		nil,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-key")
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
}

func ptrString(value string) *string {
	return &value
}

func validCheckoutRequest(cartID uuid.UUID) string {
	return fmt.Sprintf(`{
		"cart_id":"%s",
		"shipping_address":{
			"line1":"123 Market St",
			"city":"Tulsa",
			"state":"OK",
			"postal_code":"74104",
			"country":"US",
			"lat":36.154,
			"lng":-95.992
		},
		"payment_method":"cash",
		"shipping_line":{
			"code":"express",
			"title":"Express",
			"price_cents":500
		}
	}`, cartID.String())
}
