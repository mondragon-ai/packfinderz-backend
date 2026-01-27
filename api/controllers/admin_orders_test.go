package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	internalorders "github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
)

type stubPayoutRepo struct {
	listFn   func(ctx context.Context, params pagination.Params) (*internalorders.PayoutOrderList, error)
	detailFn func(ctx context.Context, orderID uuid.UUID) (*internalorders.OrderDetail, error)
}

func (s stubPayoutRepo) ListPayoutOrders(ctx context.Context, params pagination.Params) (*internalorders.PayoutOrderList, error) {
	if s.listFn != nil {
		return s.listFn(ctx, params)
	}
	return &internalorders.PayoutOrderList{}, nil
}

func (s stubPayoutRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*internalorders.OrderDetail, error) {
	if s.detailFn != nil {
		return s.detailFn(ctx, orderID)
	}
	return nil, nil
}

type stubConfirmService struct {
	input  internalorders.ConfirmPayoutInput
	err    error
	called bool
}

func (s *stubConfirmService) ConfirmPayout(ctx context.Context, input internalorders.ConfirmPayoutInput) error {
	s.called = true
	s.input = input
	return s.err
}

func TestAdminPayoutOrdersList(t *testing.T) {
	orderID := uuid.New()
	now := time.Now().UTC()
	expected := &internalorders.PayoutOrderList{
		Orders: []internalorders.PayoutOrderSummary{{
			OrderID:       orderID,
			VendorStoreID: uuid.New(),
			OrderNumber:   1,
			AmountCents:   5000,
			DeliveredAt:   now,
		}},
	}

	repo := stubPayoutRepo{
		listFn: func(ctx context.Context, params pagination.Params) (*internalorders.PayoutOrderList, error) {
			if params.Limit != 5 {
				t.Fatalf("unexpected limit %d", params.Limit)
			}
			return expected, nil
		},
	}

	handler := AdminPayoutOrders(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/?limit=5", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data internalorders.PayoutOrderList `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Orders) != 1 || envelope.Data.Orders[0].OrderID != orderID {
		t.Fatalf("unexpected payload %v", envelope.Data)
	}
}

func TestAdminPayoutOrderDetail(t *testing.T) {
	orderID := uuid.New()
	detail := &internalorders.OrderDetail{
		Order: &internalorders.VendorOrderSummary{
			Status: enums.VendorOrderStatusDelivered,
		},
		PaymentIntent: &internalorders.PaymentIntentDetail{
			Status: string(enums.PaymentStatusSettled),
		},
	}
	repo := stubPayoutRepo{
		detailFn: func(ctx context.Context, id uuid.UUID) (*internalorders.OrderDetail, error) {
			if id != orderID {
				t.Fatalf("unexpected id %s", id)
			}
			return detail, nil
		},
	}

	handler := AdminPayoutOrderDetail(repo, nil)
	req := withOrderID(httptest.NewRequest(http.MethodGet, "/", nil), orderID)
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
	if envelope.Data.Order == nil || envelope.Data.Order.Status != enums.VendorOrderStatusDelivered {
		t.Fatalf("unexpected detail %v", envelope.Data)
	}
}

func TestAdminPayoutOrderDetail_IneligibleState(t *testing.T) {
	orderID := uuid.New()
	repo := stubPayoutRepo{
		detailFn: func(ctx context.Context, id uuid.UUID) (*internalorders.OrderDetail, error) {
			return &internalorders.OrderDetail{
				Order: &internalorders.VendorOrderSummary{
					Status: enums.VendorOrderStatusAccepted,
				},
				PaymentIntent: &internalorders.PaymentIntentDetail{
					Status: string(enums.PaymentStatusSettled),
				},
			}, nil
		},
	}

	handler := AdminPayoutOrderDetail(repo, nil)
	req := withOrderID(httptest.NewRequest(http.MethodGet, "/", nil), orderID)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 got %d", resp.Code)
	}
}

func TestAdminPayoutOrderDetail_PaymentNotSettled(t *testing.T) {
	orderID := uuid.New()
	repo := stubPayoutRepo{
		detailFn: func(ctx context.Context, id uuid.UUID) (*internalorders.OrderDetail, error) {
			return &internalorders.OrderDetail{
				Order: &internalorders.VendorOrderSummary{
					Status: enums.VendorOrderStatusDelivered,
				},
				PaymentIntent: &internalorders.PaymentIntentDetail{
					Status: string(enums.PaymentStatusUnpaid),
				},
			}, nil
		},
	}

	handler := AdminPayoutOrderDetail(repo, nil)
	req := withOrderID(httptest.NewRequest(http.MethodGet, "/", nil), orderID)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 got %d", resp.Code)
	}
}

func TestAdminConfirmPayoutSuccess(t *testing.T) {
	orderID := uuid.New()
	userID := uuid.New()
	storeID := uuid.New()
	service := &stubConfirmService{}

	handler := AdminConfirmPayout(service, nil)
	req := withOrderID(httptest.NewRequest(http.MethodPost, "/", nil), orderID)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID.String()))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}
	if !service.called {
		t.Fatal("expected service called")
	}
	if service.input.OrderID != orderID {
		t.Fatalf("unexpected order id %v", service.input.OrderID)
	}
	if service.input.ActorUserID != userID {
		t.Fatalf("unexpected user id %v", service.input.ActorUserID)
	}
	if service.input.ActorStoreID != storeID {
		t.Fatalf("unexpected store id %v", service.input.ActorStoreID)
	}
}

func TestAdminConfirmPayoutInvalidOrderID(t *testing.T) {
	service := &stubConfirmService{}

	handler := AdminConfirmPayout(service, nil)
	req := withOrderParamValue(httptest.NewRequest(http.MethodPost, "/", nil), "invalid-uuid")
	req = req.WithContext(middleware.WithUserID(req.Context(), uuid.New().String()))
	req = req.WithContext(middleware.WithStoreID(req.Context(), uuid.New().String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
	if service.called {
		t.Fatal("expected service not called")
	}
}

func TestAdminConfirmPayoutUnauthorized(t *testing.T) {
	orderID := uuid.New()
	service := &stubConfirmService{}

	handler := AdminConfirmPayout(service, nil)
	req := withOrderID(httptest.NewRequest(http.MethodPost, "/", nil), orderID)
	req = req.WithContext(middleware.WithStoreID(req.Context(), uuid.New().String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.Code)
	}
	if service.called {
		t.Fatal("expected service not called")
	}
}

func withOrderID(req *http.Request, orderID uuid.UUID) *http.Request {
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}

func withOrderParamValue(req *http.Request, value string) *http.Request {
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}
