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

func withOrderID(req *http.Request, orderID uuid.UUID) *http.Request {
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("orderId", orderID.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}
