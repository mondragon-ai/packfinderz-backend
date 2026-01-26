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
	lineItems     map[uuid.UUID]*models.OrderLineItem
	orderUpdates  map[string]any
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
	items := make([]models.OrderLineItem, 0, len(s.lineItems))
	for _, item := range s.lineItems {
		if item.OrderID == orderID {
			items = append(items, *item)
		}
	}
	return items, nil
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

func (s *stubOrdersRepo) FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error) {
	if s.lineItems == nil {
		return nil, gorm.ErrRecordNotFound
	}
	item, ok := s.lineItems[lineItemID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return item, nil
}

func (s *stubOrdersRepo) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	if s.lineItems == nil {
		return gorm.ErrRecordNotFound
	}
	item, ok := s.lineItems[lineItemID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	item.Status = status
	item.Notes = notes
	return nil
}

func (s *stubOrdersRepo) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	s.orderUpdates = updates
	if s.order == nil || s.order.ID != orderID {
		return gorm.ErrRecordNotFound
	}
	for key, value := range updates {
		switch key {
		case "subtotal_cents":
			if v, ok := value.(int); ok {
				s.order.SubtotalCents = v
			}
		case "total_cents":
			if v, ok := value.(int); ok {
				s.order.TotalCents = v
			}
		case "balance_due_cents":
			if v, ok := value.(int); ok {
				s.order.BalanceDueCents = v
			}
		case "fulfillment_status":
			if v, ok := value.(enums.VendorOrderFulfillmentStatus); ok {
				s.order.FulfillmentStatus = v
			}
		case "status":
			if v, ok := value.(enums.VendorOrderStatus); ok {
				s.order.Status = v
			}
		}
	}
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

type inventoryReleaseCall struct {
	productID uuid.UUID
	qty       int
}

type stubInventoryReleaser struct {
	calls []inventoryReleaseCall
	err   error
}

func (s *stubInventoryReleaser) Release(ctx context.Context, tx *gorm.DB, productID uuid.UUID, qty int) error {
	if s.err != nil {
		return s.err
	}
	s.calls = append(s.calls, inventoryReleaseCall{productID: productID, qty: qty})
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
	svc, err := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{})
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
	svc, _ := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{})
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
	svc, _ := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{})
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

func TestLineItemDecisionFulfillEmitsEvent(t *testing.T) {
	orderID := uuid.New()
	storeID := uuid.New()
	buyerID := uuid.New()
	lineID := uuid.New()
	productID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:                orderID,
			VendorStoreID:     storeID,
			BuyerStoreID:      buyerID,
			CheckoutGroupID:   uuid.New(),
			Status:            enums.VendorOrderStatusAccepted,
			FulfillmentStatus: enums.VendorOrderFulfillmentStatusPending,
			ShippingStatus:    enums.VendorOrderShippingStatusPending,
			SubtotalCents:     1200,
			TotalCents:        1200,
			BalanceDueCents:   1200,
		},
		lineItems: map[uuid.UUID]*models.OrderLineItem{
			lineID: {
				ID:         lineID,
				OrderID:    orderID,
				ProductID:  &productID,
				Qty:        2,
				TotalCents: 1200,
				Status:     enums.LineItemStatusPending,
			},
		},
	}
	outbox := &stubOutboxPublisher{}
	inventory := &stubInventoryReleaser{}
	svc, err := NewService(repo, stubTxRunner{}, outbox, inventory)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	err = svc.LineItemDecision(context.Background(), LineItemDecisionInput{
		OrderID:      orderID,
		LineItemID:   lineID,
		Decision:     LineItemDecisionFulfill,
		ActorUserID:  uuid.New(),
		ActorStoreID: storeID,
		ActorRole:    "owner",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}

	if len(inventory.calls) != 0 {
		t.Fatalf("unexpected inventory release call")
	}
	if !outbox.called {
		t.Fatal("expected outbox event")
	}
	event, ok := outbox.event.Data.(OrderFulfilledEvent)
	if !ok {
		t.Fatalf("unexpected event payload %T", outbox.event.Data)
	}
	if event.RejectedItemCount != 0 {
		t.Fatalf("unexpected rejected item count %d", event.RejectedItemCount)
	}
	if event.ResolvedLineItemID != lineID {
		t.Fatalf("unexpected resolved line item %s", event.ResolvedLineItemID)
	}
	if repo.order.Status != enums.VendorOrderStatusHold {
		t.Fatalf("unexpected order status %s", repo.order.Status)
	}
	if repo.order.FulfillmentStatus != enums.VendorOrderFulfillmentStatusFulfilled {
		t.Fatalf("unexpected fulfillment status %s", repo.order.FulfillmentStatus)
	}
	if repo.order.BalanceDueCents != 1200 {
		t.Fatalf("unexpected balance %d", repo.order.BalanceDueCents)
	}
	if repo.lineItems[lineID].Status != enums.LineItemStatusFulfilled {
		t.Fatalf("unexpected line item status %s", repo.lineItems[lineID].Status)
	}
}

func TestLineItemDecisionRejectReleasesInventory(t *testing.T) {
	orderID := uuid.New()
	storeID := uuid.New()
	buyerID := uuid.New()
	lineID := uuid.New()
	productID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:                orderID,
			VendorStoreID:     storeID,
			BuyerStoreID:      buyerID,
			CheckoutGroupID:   uuid.New(),
			Status:            enums.VendorOrderStatusAccepted,
			FulfillmentStatus: enums.VendorOrderFulfillmentStatusPending,
			ShippingStatus:    enums.VendorOrderShippingStatusPending,
			SubtotalCents:     2000,
			TotalCents:        2000,
			BalanceDueCents:   2000,
		},
		lineItems: map[uuid.UUID]*models.OrderLineItem{
			lineID: {
				ID:         lineID,
				OrderID:    orderID,
				ProductID:  &productID,
				Qty:        3,
				TotalCents: 2000,
				Status:     enums.LineItemStatusPending,
			},
		},
	}
	outbox := &stubOutboxPublisher{}
	inventory := &stubInventoryReleaser{}
	svc, _ := NewService(repo, stubTxRunner{}, outbox, inventory)
	notes := "damaged"
	err := svc.LineItemDecision(context.Background(), LineItemDecisionInput{
		OrderID:      orderID,
		LineItemID:   lineID,
		Decision:     LineItemDecisionReject,
		Notes:        &notes,
		ActorUserID:  uuid.New(),
		ActorStoreID: storeID,
		ActorRole:    "owner",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if len(inventory.calls) != 1 {
		t.Fatalf("expected inventory release")
	}
	call := inventory.calls[0]
	if call.productID != productID || call.qty != 3 {
		t.Fatalf("unexpected release call %+v", call)
	}
	if !outbox.called {
		t.Fatal("expected outbox event")
	}
	event, ok := outbox.event.Data.(OrderFulfilledEvent)
	if !ok {
		t.Fatalf("unexpected event payload %T", outbox.event.Data)
	}
	if event.RejectedItemCount != 1 {
		t.Fatalf("unexpected rejected count %d", event.RejectedItemCount)
	}
	if event.ResolvedLineItemID != lineID {
		t.Fatalf("unexpected resolved line item id %s", event.ResolvedLineItemID)
	}
	if repo.order.SubtotalCents != 0 || repo.order.TotalCents != 0 || repo.order.BalanceDueCents != 0 {
		t.Fatalf("unexpected order totals %+v", repo.order)
	}
	if repo.lineItems[lineID].Status != enums.LineItemStatusRejected {
		t.Fatalf("unexpected line item status %s", repo.lineItems[lineID].Status)
	}
	if repo.lineItems[lineID].Notes == nil || *repo.lineItems[lineID].Notes != notes {
		t.Fatalf("unexpected line item notes %v", repo.lineItems[lineID].Notes)
	}
	if repo.order.FulfillmentStatus != enums.VendorOrderFulfillmentStatusPartial {
		t.Fatalf("unexpected fulfillment status %s", repo.order.FulfillmentStatus)
	}
}
