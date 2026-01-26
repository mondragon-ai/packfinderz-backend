package orders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	internalorders "github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
)

type stubControllerOrdersRepo struct {
	listBuyer  func(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters internalorders.BuyerOrderFilters) (*internalorders.BuyerOrderList, error)
	listVendor func(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters internalorders.VendorOrderFilters) (*internalorders.VendorOrderList, error)
	detail     func(ctx context.Context, orderID uuid.UUID) (*internalorders.OrderDetail, error)
}

func (s *stubControllerOrdersRepo) WithTx(tx *gorm.DB) internalorders.Repository {
	return s
}

func (s *stubControllerOrdersRepo) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	panic("not implemented")
}

func (s *stubControllerOrdersRepo) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters internalorders.BuyerOrderFilters) (*internalorders.BuyerOrderList, error) {
	if s.listBuyer != nil {
		return s.listBuyer(ctx, buyerStoreID, params, filters)
	}
	return nil, nil
}

func (s *stubControllerOrdersRepo) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters internalorders.VendorOrderFilters) (*internalorders.VendorOrderList, error) {
	if s.listVendor != nil {
		return s.listVendor(ctx, vendorStoreID, params, filters)
	}
	return nil, nil
}

func (s *stubControllerOrdersRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*internalorders.OrderDetail, error) {
	if s.detail != nil {
		return s.detail(ctx, orderID)
	}
	return nil, nil
}

func (s *stubControllerOrdersRepo) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubControllerOrdersRepo) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	return nil
}

type stubControllerOrdersService struct {
	decision func(ctx context.Context, input internalorders.VendorDecisionInput) error
}

func (s *stubControllerOrdersService) VendorDecision(ctx context.Context, input internalorders.VendorDecisionInput) error {
	if s.decision != nil {
		return s.decision(ctx, input)
	}
	return nil
}

func TestListBuyerPerspective(t *testing.T) {
	storeID := uuid.New()
	expected := &internalorders.BuyerOrderList{
		Orders: []internalorders.BuyerOrderSummary{
			{OrderNumber: 42},
		},
	}
	repo := &stubControllerOrdersRepo{
		listBuyer: func(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters internalorders.BuyerOrderFilters) (*internalorders.BuyerOrderList, error) {
			if buyerStoreID != storeID {
				t.Fatalf("unexpected buyer store id %s", buyerStoreID)
			}
			if params.Limit != 5 {
				t.Fatalf("unexpected limit %d", params.Limit)
			}
			if filters.Query != "tap" {
				t.Fatalf("unexpected query %q", filters.Query)
			}
			if filters.OrderStatus == nil || *filters.OrderStatus != enums.VendorOrderStatusCreatedPending {
				t.Fatalf("order status not parsed")
			}
			return expected, nil
		},
	}

	handler := List(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?limit=5&q=tap&order_status=created_pending", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeBuyer))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data internalorders.BuyerOrderList `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Orders) != 1 || envelope.Data.Orders[0].OrderNumber != 42 {
		t.Fatalf("unexpected orders in response")
	}
}

func TestListVendorPerspectiveActionable(t *testing.T) {
	storeID := uuid.New()
	expected := &internalorders.VendorOrderList{
		Orders: []internalorders.VendorOrderSummary{
			{OrderNumber: 100},
		},
	}
	repo := &stubControllerOrdersRepo{
		listVendor: func(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters internalorders.VendorOrderFilters) (*internalorders.VendorOrderList, error) {
			if vendorStoreID != storeID {
				t.Fatalf("unexpected vendor store id %s", vendorStoreID)
			}
			if len(filters.ActionableStatuses) != 2 {
				t.Fatalf("expected actionable statuses")
			}
			return expected, nil
		},
	}

	handler := List(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?actionable_statuses=created_pending,accepted", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeVendor))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data internalorders.VendorOrderList `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Orders) != 1 || envelope.Data.Orders[0].OrderNumber != 100 {
		t.Fatalf("unexpected vendor orders in response")
	}
}

func TestDetailUnauthorized(t *testing.T) {
	storeID := uuid.New()
	orderID := uuid.New()
	repo := &stubControllerOrdersRepo{
		detail: func(ctx context.Context, incoming uuid.UUID) (*internalorders.OrderDetail, error) {
			return &internalorders.OrderDetail{
				BuyerStore:  internalorders.OrderStoreSummary{ID: uuid.New()},
				VendorStore: internalorders.OrderStoreSummary{ID: uuid.New()},
			}, nil
		},
	}

	handler := Detail(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String(), nil)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeBuyer))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}

func TestDetailSuccess(t *testing.T) {
	storeID := uuid.New()
	orderID := uuid.New()
	repo := &stubControllerOrdersRepo{
		detail: func(ctx context.Context, incoming uuid.UUID) (*internalorders.OrderDetail, error) {
			return &internalorders.OrderDetail{
				Order:       &internalorders.VendorOrderSummary{OrderNumber: 7},
				BuyerStore:  internalorders.OrderStoreSummary{ID: uuid.New()},
				VendorStore: internalorders.OrderStoreSummary{ID: storeID},
			}, nil
		},
	}

	handler := Detail(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String(), nil)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeVendor))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data internalorders.OrderDetail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Order == nil || envelope.Data.Order.OrderNumber != 7 {
		t.Fatalf("unexpected order detail")
	}
}

func TestVendorOrderDecisionSuccess(t *testing.T) {
	storeID := uuid.New()
	orderID := uuid.New()
	called := false
	svc := &stubControllerOrdersService{
		decision: func(ctx context.Context, input internalorders.VendorDecisionInput) error {
			if input.OrderID != orderID {
				t.Fatalf("unexpected order id %s", input.OrderID)
			}
			if input.Decision != internalorders.VendorOrderDecisionAccept {
				t.Fatalf("unexpected decision %s", input.Decision)
			}
			if input.ActorStoreID != storeID {
				t.Fatalf("unexpected store id %s", input.ActorStoreID)
			}
			called = true
			return nil
		},
	}

	handler := VendorOrderDecision(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vendor/orders/"+orderID.String()+"/decision", strings.NewReader(`{"decision":"accept"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeVendor))
	req = req.WithContext(middleware.WithUserID(req.Context(), uuid.New().String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}
	if !called {
		t.Fatalf("service not invoked")
	}
}

func TestVendorOrderDecisionStoreMismatch(t *testing.T) {
	orderID := uuid.New()
	handler := VendorOrderDecision(&stubControllerOrdersService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vendor/orders/"+orderID.String()+"/decision", strings.NewReader(`{"decision":"accept"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeBuyer))
	req = req.WithContext(middleware.WithUserID(req.Context(), uuid.New().String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}

func TestVendorOrderDecisionInvalidDecision(t *testing.T) {
	storeID := uuid.New()
	orderID := uuid.New()
	handler := VendorOrderDecision(&stubControllerOrdersService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vendor/orders/"+orderID.String()+"/decision", strings.NewReader(`{"decision":"maybe"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = req.WithContext(middleware.WithStoreType(req.Context(), enums.StoreTypeVendor))
	req = req.WithContext(middleware.WithUserID(req.Context(), uuid.New().String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
}
