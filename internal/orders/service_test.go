package orders

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
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
	createCheckoutGroup  func(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error)
	createVendorOrder    func(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error)
	createOrderLineItems func(ctx context.Context, items []models.OrderLineItem) error
	createPaymentIntent  func(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error)
	findPaymentIntent    func(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error)
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

func (s *stubOrdersRepo) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*AgentOrderQueueList, error) {
	return &AgentOrderQueueList{}, nil
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
	svc, err := NewService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	svc, _ := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{}, reserver)
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
	svc, _ := NewService(repo, stubTxRunner{}, outbox, &stubInventoryReleaser{}, reserver)
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
	svc, err := NewService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	svc, err := NewService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	svc, err := NewService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	svc, err := NewService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
	svc, _ := NewService(repo, stubTxRunner{}, outbox, inventory, reserver)
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
