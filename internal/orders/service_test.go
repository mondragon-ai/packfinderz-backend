package orders

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubOrdersRepo struct {
	order         *models.VendorOrder
	updatedStatus enums.VendorOrderStatus
}

func (s *stubOrdersRepo) WithTx(tx *gorm.DB) Repository {
	return s
}

func (s *stubOrdersRepo) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	panic("not implemented")
}

func (s *stubOrdersRepo) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters BuyerOrderFilters) (*BuyerOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters VendorOrderFilters) (*VendorOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*OrderDetail, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	if s.order == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.order, nil
}

func (s *stubOrdersRepo) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	s.updatedStatus = status
	return nil
}

type stubOutboxPublisher struct {
	event  outbox.DomainEvent
	called bool
	err    error
}

func (s *stubOutboxPublisher) Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error {
	if s.err != nil {
		return s.err
	}
	s.called = true
	s.event = event
	return nil
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

func TestVendorDecision(t *testing.T) {
	orderID := uuid.New()
	storeID := uuid.New()
	buyerID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:              orderID,
			VendorStoreID:   storeID,
			BuyerStoreID:    buyerID,
			CheckoutGroupID: uuid.New(),
			Status:          enums.VendorOrderStatusCreatedPending,
		},
	}
	outbox := &stubOutboxPublisher{}
	svc, err := NewService(repo, stubTxRunner{}, outbox)
	if err != nil {
		t.Fatalf("service constructor failed: %v", err)
	}

	err = svc.VendorDecision(context.Background(), VendorDecisionInput{
		OrderID:      orderID,
		Decision:     VendorOrderDecisionAccept,
		ActorUserID:  uuid.New(),
		ActorStoreID: storeID,
		ActorRole:    "owner",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if repo.updatedStatus != enums.VendorOrderStatusAccepted {
		t.Fatalf("expected status accepted got %s", repo.updatedStatus)
	}
	if !outbox.called {
		t.Fatal("expected outbox event")
	}
	if outbox.event.EventType != enums.EventOrderDecided {
		t.Fatalf("unexpected event type %s", outbox.event.EventType)
	}
}

func TestVendorDecisionIdempotent(t *testing.T) {
	orderID := uuid.New()
	storeID := uuid.New()
	order := &models.VendorOrder{
		ID:            orderID,
		VendorStoreID: storeID,
		Status:        enums.VendorOrderStatusAccepted,
	}
	repo := &stubOrdersRepo{order: order}
	outbox := &stubOutboxPublisher{}
	svc, _ := NewService(repo, stubTxRunner{}, outbox)
	err := svc.VendorDecision(context.Background(), VendorDecisionInput{
		OrderID:      orderID,
		Decision:     VendorOrderDecisionAccept,
		ActorUserID:  uuid.New(),
		ActorStoreID: storeID,
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if outbox.called {
		t.Fatalf("unexpected outbox call")
	}
}

func TestVendorDecisionInvalidState(t *testing.T) {
	orderID := uuid.New()
	storeID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:            orderID,
			VendorStoreID: storeID,
			Status:        enums.VendorOrderStatusAccepted,
		},
	}
	outbox := &stubOutboxPublisher{}
	svc, _ := NewService(repo, stubTxRunner{}, outbox)
	err := svc.VendorDecision(context.Background(), VendorDecisionInput{
		OrderID:      orderID,
		Decision:     VendorOrderDecisionReject,
		ActorUserID:  uuid.New(),
		ActorStoreID: storeID,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	typed := pkgerrors.As(err)
	if typed == nil || typed.Code() != pkgerrors.CodeStateConflict {
		t.Fatalf("unexpected error %v", err)
	}
	if outbox.called {
		t.Fatalf("unexpected outbox call")
	}
}
