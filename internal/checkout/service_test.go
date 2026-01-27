package checkout

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestServiceExecuteSuccess(t *testing.T) {
	t.Parallel()

	buyerStoreID := uuid.New()
	vendorStoreID := uuid.New()
	cartID := uuid.New()
	lineA := uuid.New()
	lineB := uuid.New()

	cartRepo := &stubCartRepo{
		record: &models.CartRecord{
			ID:           cartID,
			BuyerStoreID: buyerStoreID,
			Status:       enums.CartStatusActive,
			Items: []models.CartItem{
				{ID: lineA, ProductID: uuid.New(), VendorStoreID: vendorStoreID, Qty: 2, UnitPriceCents: 1000},
				{ID: lineB, ProductID: uuid.New(), VendorStoreID: vendorStoreID, Qty: 1, UnitPriceCents: 500},
			},
		},
	}

	storeSvc := &stubStoresService{
		lookup: map[uuid.UUID]*stores.StoreDTO{
			buyerStoreID: {
				ID:                 buyerStoreID,
				Type:               enums.StoreTypeBuyer,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
			},
			vendorStoreID: {
				ID:                 vendorStoreID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
			},
		},
	}

	productRepo := &stubProductLoader{
		products: map[uuid.UUID]*models.Product{},
	}
	for _, item := range cartRepo.record.Items {
		productRepo.products[item.ProductID] = &models.Product{ID: item.ProductID, Title: "Sample", Category: enums.ProductCategoryCart}
	}

	outboxPublisher := &stubOutboxPublisher{}
	ordersRepo := &stubOrdersRepo{}
	reservationRunner := &stubReservationRunner{
		results: []reservation.InventoryReservationResult{
			{CartItemID: lineA, ProductID: cartRepo.record.Items[0].ProductID, Qty: 2, Reserved: true},
			{CartItemID: lineB, ProductID: cartRepo.record.Items[1].ProductID, Qty: 1, Reserved: false, Reason: "insufficient_inventory"},
		},
	}

	svc, err := NewService(
		stubTxRunner{},
		cartRepo,
		ordersRepo,
		storeSvc,
		productRepo,
		reservationRunner,
		outboxPublisher,
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	group, err := svc.Execute(context.Background(), buyerStoreID, cartID, CheckoutInput{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if group == nil {
		t.Fatal("expected group")
	}
	if group.BuyerStoreID != buyerStoreID {
		t.Fatalf("unexpected buyer store: %s", group.BuyerStoreID)
	}
	if len(group.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(group.VendorOrders))
	}
	if !cartRepo.updated {
		t.Fatal("expected cart marked converted")
	}
	order := group.VendorOrders[0]
	if len(order.Items) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(order.Items))
	}
	if order.Items[1].Status != enums.LineItemStatusRejected {
		t.Fatalf("expected rejected status, got %s", order.Items[1].Status)
	}
	if order.Items[0].Status != enums.LineItemStatusPending {
		t.Fatalf("expected pending status, got %s", order.Items[0].Status)
	}
	if order.PaymentIntent == nil {
		t.Fatal("expected payment intent")
	}

	if len(outboxPublisher.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outboxPublisher.events))
	}
	event := outboxPublisher.events[0]
	if event.EventType != enums.EventOrderCreated {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	if event.AggregateType != enums.AggregateCheckoutGroup {
		t.Fatalf("unexpected aggregate type: %s", event.AggregateType)
	}
	if event.AggregateID != group.ID {
		t.Fatalf("unexpected aggregate id: %s", event.AggregateID)
	}
	payload, ok := event.Data.(OrderCreatedEvent)
	if !ok {
		t.Fatalf("event payload type mismatch")
	}
	if payload.CheckoutGroupID != group.ID {
		t.Fatalf("unexpected payload checkout group: %s", payload.CheckoutGroupID)
	}
	if len(payload.VendorOrderIDs) != len(ordersRepo.orderSequence) {
		t.Fatalf("expected %d vendor orders in payload, got %d", len(ordersRepo.orderSequence), len(payload.VendorOrderIDs))
	}
	for i, id := range ordersRepo.orderSequence {
		if payload.VendorOrderIDs[i] != id {
			t.Fatalf("unexpected vendor order id at idx %d: want %s got %s", i, id, payload.VendorOrderIDs[i])
		}
	}
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubCartRepo struct {
	record  *models.CartRecord
	updated bool
}

func (s *stubCartRepo) WithTx(tx *gorm.DB) cart.CartRepository { return s }
func (s *stubCartRepo) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}
func (s *stubCartRepo) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if s.record == nil || s.record.ID != id {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "cart not found")
	}
	return s.record, nil
}
func (s *stubCartRepo) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not implemented")
}
func (s *stubCartRepo) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not implemented")
}
func (s *stubCartRepo) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	return nil
}
func (s *stubCartRepo) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	if status == enums.CartStatusConverted {
		s.updated = true
	}
	return nil
}

type stubOrdersRepo struct {
	group          *models.CheckoutGroup
	vendorOrders   map[uuid.UUID]*models.VendorOrder
	orderSequence  []uuid.UUID
	lineItems      map[uuid.UUID][]models.OrderLineItem
	paymentIntents map[uuid.UUID]*models.PaymentIntent
}

// ListAssignedOrders implements [orders.Repository].
func (s *stubOrdersRepo) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	panic("unimplemented")
}

// ListUnassignedHoldOrders implements [orders.Repository].
func (s *stubOrdersRepo) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	panic("unimplemented")
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

func (s *stubOrdersRepo) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	panic("unimplemented")
}

func (s *stubOrdersRepo) WithTx(tx *gorm.DB) orders.Repository { return s }
func (s *stubOrdersRepo) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	group.ID = uuid.New()
	s.group = &models.CheckoutGroup{
		ID:           group.ID,
		BuyerStoreID: group.BuyerStoreID,
		CartID:       group.CartID,
	}
	return s.group, nil
}
func (s *stubOrdersRepo) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	if s.vendorOrders == nil {
		s.vendorOrders = make(map[uuid.UUID]*models.VendorOrder)
	}
	order.ID = uuid.New()
	stored := *order
	s.vendorOrders[order.ID] = &stored
	s.orderSequence = append(s.orderSequence, order.ID)
	return &stored, nil
}
func (s *stubOrdersRepo) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	if len(items) == 0 {
		return nil
	}
	if s.lineItems == nil {
		s.lineItems = make(map[uuid.UUID][]models.OrderLineItem)
	}
	orderID := items[0].OrderID
	s.lineItems[orderID] = append([]models.OrderLineItem{}, items...)
	return nil
}
func (s *stubOrdersRepo) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	if s.paymentIntents == nil {
		s.paymentIntents = make(map[uuid.UUID]*models.PaymentIntent)
	}
	s.paymentIntents[intent.OrderID] = intent
	return intent, nil
}
func (s *stubOrdersRepo) FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error) {
	if s.group == nil || s.group.ID != id {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "checkout group missing")
	}
	result := *s.group
	for _, orderID := range s.orderSequence {
		order := s.vendorOrders[orderID]
		if items, ok := s.lineItems[orderID]; ok {
			copyItems := make([]models.OrderLineItem, len(items))
			copy(copyItems, items)
			order.Items = copyItems
		}
		if intent, ok := s.paymentIntents[orderID]; ok {
			order.PaymentIntent = intent
		}
		result.VendorOrders = append(result.VendorOrders, *order)
	}
	return &result, nil
}
func (s *stubOrdersRepo) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}
func (s *stubOrdersRepo) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}
func (s *stubOrdersRepo) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}

func (s *stubOrdersRepo) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}

func (s *stubOrdersRepo) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	return nil
}

func (s *stubOrdersRepo) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters orders.BuyerOrderFilters) (*orders.BuyerOrderList, error) {
	return &orders.BuyerOrderList{}, nil
}

func (s *stubOrdersRepo) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters orders.VendorOrderFilters) (*orders.VendorOrderList, error) {
	return &orders.VendorOrderList{}, nil
}

func (s *stubOrdersRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*orders.OrderDetail, error) {
	return &orders.OrderDetail{}, nil
}

type stubStoresService struct {
	lookup map[uuid.UUID]*stores.StoreDTO
}

func (s *stubStoresService) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	if store, ok := s.lookup[id]; ok {
		return store, nil
	}
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "store missing")
}
func (s *stubStoresService) Update(ctx context.Context, userID, storeID uuid.UUID, input stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}
func (s *stubStoresService) ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}
func (s *stubStoresService) InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	return nil, "", pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}
func (s *stubStoresService) RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error {
	return pkgerrors.New(pkgerrors.CodeNotFound, "not used")
}

type stubProductLoader struct {
	products map[uuid.UUID]*models.Product
}

func (s *stubProductLoader) FindByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	if product, ok := s.products[id]; ok {
		return product, nil
	}
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "product missing")
}

type stubReservationRunner struct {
	results []reservation.InventoryReservationResult
}

func (s stubReservationRunner) Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error) {
	return s.results, nil
}

type stubOutboxPublisher struct {
	events []outbox.DomainEvent
}

func (s *stubOutboxPublisher) Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error {
	s.events = append(s.events, event)
	return nil
}
