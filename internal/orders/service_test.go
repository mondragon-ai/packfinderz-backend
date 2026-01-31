package orders

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
	"github.com/angelmondragon/packfinderz-backend/internal/ledger"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubOrdersRepo struct {
	order                *models.VendorOrder
	updatedStatus        enums.VendorOrderStatus
	lineItems            map[uuid.UUID]*models.OrderLineItem
	orderUpdates         map[string]any
	paymentUpdates       map[string]any
	createCheckoutGroup  func(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error)
	createVendorOrder    func(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error)
	createOrderLineItems func(ctx context.Context, items []models.OrderLineItem) error
	createPaymentIntent  func(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error)
	findPaymentIntent    func(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error)
	findOrderDetail      func(ctx context.Context, orderID uuid.UUID) (*OrderDetail, error)
	updateAssignment     func(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error
	updatePaymentIntent  func(ctx context.Context, orderID uuid.UUID, updates map[string]any) error
}

// FindPendingOrdersBefore implements [Repository].
func (s *stubOrdersRepo) FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error) {
	panic("unimplemented")
}

func (s *stubOrdersRepo) WithTx(tx *gorm.DB) Repository {
	return s
}

func (s *stubOrdersRepo) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	if s.createCheckoutGroup != nil {
		return s.createCheckoutGroup(ctx, group)
	}
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	return group, nil
}

func (s *stubOrdersRepo) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	if s.createVendorOrder != nil {
		return s.createVendorOrder(ctx, order)
	}
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	return order, nil
}

func (s *stubOrdersRepo) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	if s.createOrderLineItems != nil {
		return s.createOrderLineItems(ctx, items)
	}
	if s.lineItems == nil {
		s.lineItems = make(map[uuid.UUID]*models.OrderLineItem)
	}
	for i := range items {
		item := items[i]
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		s.lineItems[item.ID] = &item
	}
	return nil
}

func (s *stubOrdersRepo) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	if s.createPaymentIntent != nil {
		return s.createPaymentIntent(ctx, intent)
	}
	if intent.ID == uuid.Nil {
		intent.ID = uuid.New()
	}
	return intent, nil
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
	if s.findPaymentIntent != nil {
		return s.findPaymentIntent(ctx, orderID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubOrdersRepo) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters BuyerOrderFilters) (*BuyerOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters VendorOrderFilters) (*VendorOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListPayoutOrders(ctx context.Context, params pagination.Params) (*PayoutOrderList, error) {
	return &PayoutOrderList{}, nil
}

func (s *stubOrdersRepo) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*AgentOrderQueueList, error) {
	return &AgentOrderQueueList{}, nil
}

func (s *stubOrdersRepo) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*AgentOrderQueueList, error) {
	return &AgentOrderQueueList{}, nil
}

func (s *stubOrdersRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*OrderDetail, error) {
	if s.findOrderDetail != nil {
		return s.findOrderDetail(ctx, orderID)
	}
	return nil, gorm.ErrRecordNotFound
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

func (s *stubOrdersRepo) UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	if s.updatePaymentIntent != nil {
		return s.updatePaymentIntent(ctx, orderID, updates)
	}
	s.paymentUpdates = updates
	return nil
}

func (s *stubOrdersRepo) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	if s.updateAssignment != nil {
		return s.updateAssignment(ctx, assignmentID, updates)
	}
	return nil
}

type stubLedgerService struct {
	recordFn func(ctx context.Context, input ledger.RecordLedgerEventInput) (*models.LedgerEvent, error)
}

func (s *stubLedgerService) RecordEvent(ctx context.Context, input ledger.RecordLedgerEventInput) (*models.LedgerEvent, error) {
	if s.recordFn != nil {
		return s.recordFn(ctx, input)
	}
	return &models.LedgerEvent{ID: uuid.New()}, nil
}

func newStubLedgerService(recordFn func(ctx context.Context, input ledger.RecordLedgerEventInput) (*models.LedgerEvent, error)) *stubLedgerService {
	return &stubLedgerService{recordFn: recordFn}
}

func newTestOrdersService(repo Repository, tx txRunner, outbox outboxPublisher, inventory InventoryReleaser, reserver inventoryReserver) (Service, error) {
	return NewService(repo, tx, outbox, inventory, reserver, newStubLedgerService(nil))
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

type stubInventoryReserver struct {
	calls []reservation.InventoryReservationRequest
	err   error
}

func (s *stubInventoryReserver) Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.calls = append(s.calls, requests...)
	results := make([]reservation.InventoryReservationResult, len(requests))
	for i, req := range requests {
		results[i] = reservation.InventoryReservationResult{
			CartItemID: req.CartItemID,
			ProductID:  req.ProductID,
			Qty:        req.Qty,
			Reserved:   true,
		}
	}
	return results, nil
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
	inventory := &stubInventoryReleaser{}
	reserver := &stubInventoryReserver{}
	svc, err := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	reserver := &stubInventoryReserver{}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{}, reserver)
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
	reserver := &stubInventoryReserver{}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{}, reserver)
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

func TestCancelOrderReleasesInventory(t *testing.T) {
	orderID := uuid.New()
	buyerStore := uuid.New()
	vendorStore := uuid.New()
	productID := uuid.New()
	lineItemID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:              orderID,
			BuyerStoreID:    buyerStore,
			VendorStoreID:   vendorStore,
			CheckoutGroupID: uuid.New(),
			Status:          enums.VendorOrderStatusAccepted,
		},
		lineItems: map[uuid.UUID]*models.OrderLineItem{
			lineItemID: {
				ID:        lineItemID,
				OrderID:   orderID,
				ProductID: &productID,
				Qty:       3,
				Status:    enums.LineItemStatusPending,
			},
		},
	}
	outbox := &stubOutboxPublisher{}
	inventory := &stubInventoryReleaser{}
	reserver := &stubInventoryReserver{}
	svc, err := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}

	err = svc.CancelOrder(context.Background(), BuyerCancelInput{
		OrderID:      orderID,
		ActorUserID:  uuid.New(),
		ActorStoreID: buyerStore,
		ActorRole:    "owner",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if len(inventory.calls) != 1 {
		t.Fatalf("expected inventory release called got %d", len(inventory.calls))
	}
	if repo.lineItems[lineItemID].Status != enums.LineItemStatusRejected {
		t.Fatalf("expected line item rejected got %s", repo.lineItems[lineItemID].Status)
	}
	if repo.orderUpdates == nil || repo.orderUpdates["status"] != enums.VendorOrderStatusCanceled {
		t.Fatalf("unexpected order status updates %+v", repo.orderUpdates)
	}
	if !outbox.called || outbox.event.EventType != enums.EventOrderCanceled {
		t.Fatalf("expected canceled event got %v", outbox.event.EventType)
	}
}

func TestNudgeVendorEmitsNotificationEvent(t *testing.T) {
	orderID := uuid.New()
	vendorStore := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:              orderID,
			BuyerStoreID:    vendorStore,
			VendorStoreID:   uuid.New(),
			CheckoutGroupID: uuid.New(),
			Status:          enums.VendorOrderStatusAccepted,
		},
	}
	outbox := &stubOutboxPublisher{}
	inventory := &stubInventoryReleaser{}
	reserver := &stubInventoryReserver{}
	svc, err := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}

	err = svc.NudgeVendor(context.Background(), BuyerNudgeInput{
		OrderID:      orderID,
		ActorUserID:  uuid.New(),
		ActorStoreID: vendorStore,
		ActorRole:    "owner",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if !outbox.called || outbox.event.EventType != enums.EventNotificationRequested {
		t.Fatalf("expected notification event got %v", outbox.event.EventType)
	}
}

func TestRetryOrderCreatesNewOrder(t *testing.T) {
	orderID := uuid.New()
	buyerStore := uuid.New()
	vendorStore := uuid.New()
	lineItemID := uuid.New()
	productID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{
			ID:                orderID,
			BuyerStoreID:      buyerStore,
			VendorStoreID:     vendorStore,
			SubtotalCents:     2000,
			DiscountCents:     0,
			TaxCents:          0,
			TransportFeeCents: 0,
			TotalCents:        2000,
			BalanceDueCents:   2000,
			Status:            enums.VendorOrderStatusExpired,
			CheckoutGroupID:   uuid.New(),
			FulfillmentStatus: enums.VendorOrderFulfillmentStatusPending,
			ShippingStatus:    enums.VendorOrderShippingStatusPending,
		},
		lineItems: map[uuid.UUID]*models.OrderLineItem{
			lineItemID: {
				ID:         lineItemID,
				OrderID:    orderID,
				ProductID:  &productID,
				Qty:        2,
				TotalCents: 2000,
				Status:     enums.LineItemStatusPending,
			},
		},
		findPaymentIntent: func(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
			return &models.PaymentIntent{
				Method: enums.PaymentMethodCash,
				Status: enums.PaymentStatusSettled,
			}, nil
		},
	}
	var createdOrder *models.VendorOrder
	repo.createVendorOrder = func(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
		order.ID = uuid.New()
		createdOrder = order
		return order, nil
	}
	groupID := uuid.New()
	repo.createCheckoutGroup = func(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
		group.ID = groupID
		return group, nil
	}
	capturedItems := make([]models.OrderLineItem, 0)
	repo.createOrderLineItems = func(ctx context.Context, items []models.OrderLineItem) error {
		capturedItems = append(capturedItems, items...)
		return nil
	}

	outbox := &stubOutboxPublisher{}
	inventory := &stubInventoryReleaser{}
	reserver := &stubInventoryReserver{}
	svc, err := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}

	result, err := svc.RetryOrder(context.Background(), BuyerRetryInput{
		OrderID:      orderID,
		ActorUserID:  uuid.New(),
		ActorStoreID: buyerStore,
		ActorRole:    "owner",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if result == nil || result.OrderID != createdOrder.ID {
		t.Fatalf("unexpected retry result %v", result)
	}
	if len(capturedItems) != 1 {
		t.Fatalf("expected line items created")
	}
	if capturedItems[0].OrderID != createdOrder.ID {
		t.Fatalf("line item not linked to new order")
	}
	if len(reserver.calls) == 0 {
		t.Fatalf("expected inventory reservation")
	}
	if !outbox.called || outbox.event.EventType != enums.EventOrderRetried {
		t.Fatalf("expected retry event got %v", outbox.event.EventType)
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
	reserver := &stubInventoryReserver{}
	svc, err := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	reserver := &stubInventoryReserver{}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
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

func TestAgentPickupSuccess(t *testing.T) {
	orderID := uuid.New()
	agentID := uuid.New()
	assignID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status:         enums.VendorOrderStatusHold,
			ShippingStatus: enums.VendorOrderShippingStatusPending,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          assignID,
			AgentUserID: agentID,
			AssignedAt:  time.Now().UTC(),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			if id != orderID {
				t.Fatalf("unexpected order id %s", id)
			}
			return detail, nil
		},
		updateAssignment: func(ctx context.Context, id uuid.UUID, updates map[string]any) error {
			if id != assignID {
				t.Fatalf("unexpected assignment id %s", id)
			}
			if _, ok := updates["pickup_time"]; !ok {
				t.Fatalf("expected pickup_time update")
			}
			return nil
		},
	}
	outbox := &stubOutboxPublisher{}
	inventory := &stubInventoryReleaser{}
	reserver := &stubInventoryReserver{}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, outbox, inventory, reserver)
	err := svc.AgentPickup(context.Background(), AgentPickupInput{OrderID: orderID, AgentUserID: agentID})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if repo.orderUpdates["status"] != enums.VendorOrderStatusInTransit {
		t.Fatalf("expected status in_transit got %v", repo.orderUpdates["status"])
	}
	if repo.orderUpdates["shipping_status"] != enums.VendorOrderShippingStatusInTransit {
		t.Fatalf("expected shipping_status in_transit got %v", repo.orderUpdates["shipping_status"])
	}
}

func TestAgentPickupForbiddenWhenNotAssigned(t *testing.T) {
	orderID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status: enums.VendorOrderStatusHold,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          uuid.New(),
			AgentUserID: uuid.New(),
			AssignedAt:  time.Now().UTC(),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentPickup(context.Background(), AgentPickupInput{OrderID: orderID, AgentUserID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
	if pkgerrors.As(err).Code() != pkgerrors.CodeForbidden {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestAgentPickupStateConflict(t *testing.T) {
	orderID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status: enums.VendorOrderStatusAccepted,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          uuid.New(),
			AgentUserID: uuid.New(),
			AssignedAt:  time.Now().UTC(),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentPickup(context.Background(), AgentPickupInput{OrderID: orderID, AgentUserID: detail.ActiveAssignment.AgentUserID})
	if err == nil {
		t.Fatal("expected error")
	}
	if pkgerrors.As(err).Code() != pkgerrors.CodeStateConflict {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestAgentPickupIdempotent(t *testing.T) {
	orderID := uuid.New()
	agentID := uuid.New()
	now := time.Now().UTC()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status:         enums.VendorOrderStatusInTransit,
			ShippingStatus: enums.VendorOrderShippingStatusInTransit,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          uuid.New(),
			AgentUserID: agentID,
			AssignedAt:  now,
			PickupTime:  &now,
		},
	}
	updatesCalled := false
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
		updateAssignment: func(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
			updatesCalled = true
			return nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentPickup(context.Background(), AgentPickupInput{OrderID: orderID, AgentUserID: agentID})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if updatesCalled {
		t.Fatalf("expected assignment update skipped")
	}
	if repo.orderUpdates != nil {
		t.Fatalf("expected no order updates, got %v", repo.orderUpdates)
	}
}

func TestAgentDeliverSuccess(t *testing.T) {
	orderID := uuid.New()
	agentID := uuid.New()
	assignID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status:         enums.VendorOrderStatusInTransit,
			ShippingStatus: enums.VendorOrderShippingStatusInTransit,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          assignID,
			AgentUserID: agentID,
			AssignedAt:  time.Now().UTC(),
		},
	}
	assignmentUpdated := false
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
		updateAssignment: func(ctx context.Context, id uuid.UUID, updates map[string]any) error {
			assignmentUpdated = true
			if id != assignID {
				t.Fatalf("unexpected assignment id %s", id)
			}
			if _, ok := updates["delivery_time"]; !ok {
				t.Fatalf("expected delivery_time update")
			}
			return nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentDeliver(context.Background(), AgentDeliverInput{OrderID: orderID, AgentUserID: agentID})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if repo.orderUpdates["status"] != enums.VendorOrderStatusDelivered {
		t.Fatalf("expected status delivered got %v", repo.orderUpdates["status"])
	}
	if repo.orderUpdates["shipping_status"] != enums.VendorOrderShippingStatusDelivered {
		t.Fatalf("expected shipping_status delivered got %v", repo.orderUpdates["shipping_status"])
	}
	if _, ok := repo.orderUpdates["delivered_at"]; !ok {
		t.Fatal("expected delivered_at timestamp")
	}
	if !assignmentUpdated {
		t.Fatal("expected assignment update")
	}
}

func TestAgentDeliverForbiddenWhenNotAssigned(t *testing.T) {
	orderID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status: enums.VendorOrderStatusInTransit,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          uuid.New(),
			AgentUserID: uuid.New(),
			AssignedAt:  time.Now().UTC(),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentDeliver(context.Background(), AgentDeliverInput{OrderID: orderID, AgentUserID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
	if pkgerrors.As(err).Code() != pkgerrors.CodeForbidden {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestAgentDeliverStateConflict(t *testing.T) {
	orderID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status: enums.VendorOrderStatusHold,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:          uuid.New(),
			AgentUserID: uuid.New(),
			AssignedAt:  time.Now().UTC(),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentDeliver(context.Background(), AgentDeliverInput{OrderID: orderID, AgentUserID: detail.ActiveAssignment.AgentUserID})
	if err == nil {
		t.Fatal("expected error")
	}
	if pkgerrors.As(err).Code() != pkgerrors.CodeStateConflict {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestAgentDeliverIdempotent(t *testing.T) {
	orderID := uuid.New()
	agentID := uuid.New()
	now := time.Now().UTC()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status:         enums.VendorOrderStatusDelivered,
			ShippingStatus: enums.VendorOrderShippingStatusDelivered,
			DeliveredAt:    &now,
		},
		ActiveAssignment: &OrderAssignmentSummary{
			ID:           uuid.New(),
			AgentUserID:  agentID,
			AssignedAt:   now,
			DeliveryTime: &now,
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
		updateAssignment: func(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
			return nil
		},
	}
	svc, _ := newTestOrdersService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{})
	err := svc.AgentDeliver(context.Background(), AgentDeliverInput{OrderID: orderID, AgentUserID: agentID})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if repo.orderUpdates != nil {
		t.Fatalf("expected no order updates, got %v", repo.orderUpdates)
	}
}

func TestService_ConfirmPayoutFinalizesOrder(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	vendorID := uuid.New()
	actorID := uuid.New()
	actorStoreID := uuid.New()

	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status: enums.VendorOrderStatusDelivered,
		},
		BuyerStore:  OrderStoreSummary{ID: buyerID},
		VendorStore: OrderStoreSummary{ID: vendorID},
		PaymentIntent: &PaymentIntentDetail{
			ID:          uuid.New(),
			AmountCents: 12345,
			Status:      string(enums.PaymentStatusSettled),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			if id != orderID {
				t.Fatalf("unexpected order id %v", id)
			}
			return detail, nil
		},
	}

	var recorded ledger.RecordLedgerEventInput
	ledgerSvc := newStubLedgerService(func(ctx context.Context, input ledger.RecordLedgerEventInput) (*models.LedgerEvent, error) {
		recorded = input
		return &models.LedgerEvent{ID: uuid.New()}, nil
	})

	outbox := &stubOutboxPublisher{}
	svc, err := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{}, &stubInventoryReserver{}, ledgerSvc)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	if err := svc.ConfirmPayout(context.Background(), ConfirmPayoutInput{
		OrderID:      orderID,
		ActorUserID:  actorID,
		ActorStoreID: actorStoreID,
		ActorRole:    "admin",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.orderUpdates == nil || repo.orderUpdates["status"] != enums.VendorOrderStatusClosed {
		t.Fatalf("order not closed, updates %v", repo.orderUpdates)
	}
	if repo.paymentUpdates == nil {
		t.Fatal("payment intent not updated")
	}
	if repo.paymentUpdates["status"] != enums.PaymentStatusPaid {
		t.Fatalf("unexpected payment status %v", repo.paymentUpdates["status"])
	}
	vendorPaidAt, ok := repo.paymentUpdates["vendor_paid_at"].(time.Time)
	if !ok || vendorPaidAt.IsZero() {
		t.Fatalf("vendor_paid_at not set %v", repo.paymentUpdates["vendor_paid_at"])
	}

	if recorded.OrderID != orderID {
		t.Fatalf("ledger recorded wrong order %v", recorded.OrderID)
	}
	if recorded.BuyerStoreID != buyerID || recorded.VendorStoreID != vendorID {
		t.Fatalf("ledger recorded wrong stores %+v", recorded)
	}
	if recorded.ActorUserID != actorID {
		t.Fatalf("ledger recorded wrong actor %v", recorded.ActorUserID)
	}
	if recorded.AmountCents != detail.PaymentIntent.AmountCents {
		t.Fatalf("ledger recorded wrong amount %d", recorded.AmountCents)
	}
	if recorded.Type != enums.LedgerEventTypeVendorPayout {
		t.Fatalf("unexpected ledger type %s", recorded.Type)
	}

	if !outbox.called || outbox.event.EventType != enums.EventOrderPaid {
		t.Fatalf("expected order_paid event, got %v", outbox.event.EventType)
	}
	event, ok := outbox.event.Data.(OrderPaidEvent)
	if !ok {
		t.Fatalf("unexpected event payload %T", outbox.event.Data)
	}
	if event.OrderID != orderID {
		t.Fatalf("unexpected order id in event %v", event.OrderID)
	}
	if event.AmountCents != detail.PaymentIntent.AmountCents {
		t.Fatalf("unexpected amount %d", event.AmountCents)
	}
	if event.BuyerStoreID != buyerID || event.VendorStoreID != vendorID {
		t.Fatalf("unexpected stores in event %+v", event)
	}
	if event.PaymentIntentID != detail.PaymentIntent.ID {
		t.Fatalf("unexpected payment intent id %v", event.PaymentIntentID)
	}
	if event.VendorPaidAt.IsZero() {
		t.Fatalf("vendor paid timestamp missing")
	}
}

func TestService_ConfirmPayoutIdempotent(t *testing.T) {
	orderID := uuid.New()
	detail := &OrderDetail{
		Order: &VendorOrderSummary{
			Status: enums.VendorOrderStatusClosed,
		},
		BuyerStore:  OrderStoreSummary{ID: uuid.New()},
		VendorStore: OrderStoreSummary{ID: uuid.New()},
		PaymentIntent: &PaymentIntentDetail{
			ID:     uuid.New(),
			Status: string(enums.PaymentStatusPaid),
		},
	}
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return detail, nil
		},
	}

	ledgerCalled := false
	ledgerSvc := newStubLedgerService(func(ctx context.Context, input ledger.RecordLedgerEventInput) (*models.LedgerEvent, error) {
		ledgerCalled = true
		return &models.LedgerEvent{ID: uuid.New()}, nil
	})

	outbox := &stubOutboxPublisher{}
	svc, _ := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{}, &stubInventoryReserver{}, ledgerSvc)
	err := svc.ConfirmPayout(context.Background(), ConfirmPayoutInput{
		OrderID:      orderID,
		ActorUserID:  uuid.New(),
		ActorStoreID: uuid.New(),
		ActorRole:    "admin",
	})
	if err != nil {
		t.Fatalf("expected success got %v", err)
	}
	if ledgerCalled {
		t.Fatal("expected ledger not to be called")
	}
	if outbox.called {
		t.Fatal("expected outbox not to be called")
	}
	if repo.orderUpdates != nil {
		t.Fatalf("expected no order update, got %v", repo.orderUpdates)
	}
	if repo.paymentUpdates != nil {
		t.Fatalf("expected no payment update, got %v", repo.paymentUpdates)
	}
}

func TestService_ConfirmPayoutValidation(t *testing.T) {
	svc, _ := NewService(&stubOrdersRepo{}, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{}, newStubLedgerService(nil))

	if err := svc.ConfirmPayout(context.Background(), ConfirmPayoutInput{OrderID: uuid.Nil, ActorUserID: uuid.New()}); err == nil {
		t.Fatal("expected validation error for missing order")
	}
	if err := svc.ConfirmPayout(context.Background(), ConfirmPayoutInput{OrderID: uuid.New(), ActorUserID: uuid.Nil}); err == nil {
		t.Fatal("expected validation error for missing actor")
	}
}

func TestService_ConfirmPayoutMissingPaymentIntent(t *testing.T) {
	orderID := uuid.New()
	repo := &stubOrdersRepo{
		order: &models.VendorOrder{ID: orderID},
		findOrderDetail: func(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
			return &OrderDetail{
				Order:       &VendorOrderSummary{},
				BuyerStore:  OrderStoreSummary{ID: uuid.New()},
				VendorStore: OrderStoreSummary{ID: uuid.New()},
			}, nil
		},
	}
	svc, _ := NewService(repo, stubTxRunner{}, &stubOutboxPublisher{}, &stubInventoryReleaser{}, &stubInventoryReserver{}, newStubLedgerService(nil))

	if err := svc.ConfirmPayout(context.Background(), ConfirmPayoutInput{OrderID: orderID, ActorUserID: uuid.New()}); err == nil {
		t.Fatal("expected error for missing payment intent")
	}
}
