package checkout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestServiceUsesVendorGroupTotals(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	invalidVendorID := uuid.New()
	productID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(10 * time.Minute),
		Items: []models.CartItem{
			{
				ID:                    uuid.New(),
				ProductID:             productID,
				VendorStoreID:         vendorID,
				Quantity:              2,
				UnitPriceCents:        1500,
				LineSubtotalCents:     2500,
				Status:                enums.CartItemStatusOK,
				AppliedVolumeDiscount: &types.AppliedVolumeDiscount{Label: "tier 2", AmountCents: 500},
			},
			{
				ID:                uuid.New(),
				ProductID:         uuid.New(),
				VendorStoreID:     invalidVendorID,
				Quantity:          1,
				UnitPriceCents:    1200,
				LineSubtotalCents: 1200,
				Status:            enums.CartItemStatusInvalid,
			},
		},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 3000,
				TotalCents:    2500,
			},
			{
				VendorStoreID: invalidVendorID,
				Status:        enums.VendorGroupStatusInvalid,
				SubtotalCents: 0,
				TotalCents:    0,
			},
		},
		ShippingAddress: &types.Address{Line1: "Old", City: "Broken", State: "OK", PostalCode: "00000", Country: "US"},
	}

	shippingAddress := &types.Address{Line1: "123 Market", City: "Tulsa", State: "OK", PostalCode: "74104", Country: "US"}
	shippingLine := &types.ShippingLine{Code: "express", Title: "Express", PriceCents: 500}

	cartRepo := &stubCartRepo{
		record: cartRecord,
	}

	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:          buyerID,
				Type:        enums.StoreTypeBuyer,
				KYCStatus:   enums.KYCStatusVerified,
				Address:     types.Address{State: "OK"},
				CompanyName: "Buyer",
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
				CompanyName:        "Vendor",
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			productID: {
				ID:       productID,
				StoreID:  vendorID,
				SKU:      "SKU123",
				Title:    "Test Product",
				Category: enums.ProductCategoryFlower,
				Unit:     enums.ProductUnitGram,
				Strain:   ptrString("Blue Dream"),
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	for _, item := range cartRecord.Items {
		if item.Status == enums.CartItemStatusOK {
			reserver.results[item.ID] = reservation.InventoryReservationResult{
				CartItemID: item.ID,
				ProductID:  item.ProductID,
				Qty:        item.Quantity,
				Reserved:   true,
			}
		}
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	result, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey:  "key",
		ShippingAddress: shippingAddress,
		PaymentMethod:   enums.PaymentMethodCash,
		ShippingLine:    shippingLine,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if cartRepo.updated == nil {
		t.Fatalf("expected cart update")
	}
	if cartRepo.updated.Status != enums.CartStatusConverted {
		t.Fatalf("cart status not converted: %s", cartRepo.updated.Status)
	}
	if cartRepo.updated.PaymentMethod == nil || *cartRepo.updated.PaymentMethod != enums.PaymentMethodCash {
		t.Fatalf("cart payment method missing")
	}
	if cartRepo.updated.ShippingLine == nil || cartRepo.updated.ShippingLine.Code != "express" {
		t.Fatalf("cart shipping line not updated")
	}

	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(result.VendorOrders))
	}

	order := result.VendorOrders[0]
	if order.ShippingAddress == nil || order.ShippingAddress.Line1 != "123 Market" {
		t.Fatalf("vendor order missing shipping address")
	}
	if order.VendorStoreID != vendorID {
		t.Fatalf("unexpected vendor: %s", order.VendorStoreID)
	}
	if order.SubtotalCents != 3000 {
		t.Fatalf("subtotal mismatch: got %d", order.SubtotalCents)
	}
	if order.TotalCents != 2500 {
		t.Fatalf("total mismatch: got %d", order.TotalCents)
	}
	if order.DiscountsCents != 500 {
		t.Fatalf("discount mismatch: got %d", order.DiscountsCents)
	}
	if order.BalanceDueCents != 2500 {
		t.Fatalf("balance due mismatch: %d", order.BalanceDueCents)
	}

	if len(order.Items) != 1 {
		t.Fatalf("expected 1 line item, got %d", len(order.Items))
	}
	item := order.Items[0]
	if item.TotalCents != 2500 {
		t.Fatalf("line total mismatch: %d", item.TotalCents)
	}
	if item.DiscountCents != 500 {
		t.Fatalf("line discount mismatch: %d", item.DiscountCents)
	}
	if item.LineSubtotalCents != 2500 {
		t.Fatalf("line subtotal mismatch: %d", item.LineSubtotalCents)
	}

	if len(orderRepo.vendorOrders) != 1 {
		t.Fatalf("unexpected vendor order count in repo: %d", len(orderRepo.vendorOrders))
	}
	if len(result.CartVendorGroups) != len(cartRecord.VendorGroups) {
		t.Fatalf("expected %d vendor groups in response, got %d", len(cartRecord.VendorGroups), len(result.CartVendorGroups))
	}
}

func ptrString(value string) *string {
	return &value
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubCartRepo struct {
	record  *models.CartRecord
	updated *models.CartRecord
}

func (s *stubCartRepo) WithTx(tx *gorm.DB) cart.CartRepository {
	return s
}

func (s *stubCartRepo) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubCartRepo) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if s.record == nil || s.record.ID != id || s.record.BuyerStoreID != buyerStoreID {
		return nil, gorm.ErrRecordNotFound
	}
	return s.record, nil
}

func (s *stubCartRepo) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCartRepo) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	s.updated = record
	return record, nil
}

func (s *stubCartRepo) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	return errors.New("not implemented")
}

func (s *stubCartRepo) ReplaceVendorGroups(ctx context.Context, cartID uuid.UUID, groups []models.CartVendorGroup) error {
	return errors.New("not implemented")
}

func (s *stubCartRepo) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	return errors.New("not implemented")
}

type stubStoreService struct {
	records map[uuid.UUID]*stores.StoreDTO
}

func (s *stubStoreService) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	if store, ok := s.records[id]; ok {
		return store, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (*stubStoreService) Update(ctx context.Context, userID, storeID uuid.UUID, input stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	return nil, errors.New("not implemented")
}

func (*stubStoreService) ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	return nil, errors.New("not implemented")
}

func (*stubStoreService) InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	return nil, "", errors.New("not implemented")
}

func (*stubStoreService) RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error {
	return errors.New("not implemented")
}

type stubProductLoader struct {
	products map[uuid.UUID]*models.Product
}

func (s stubProductLoader) FindByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	if product, ok := s.products[id]; ok {
		return product, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type stubReservationRunner struct {
	results map[uuid.UUID]reservation.InventoryReservationResult
}

func (s stubReservationRunner) Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error) {
	results := make([]reservation.InventoryReservationResult, len(requests))
	for i, req := range requests {
		if res, ok := s.results[req.CartItemID]; ok {
			results[i] = res
			continue
		}
		results[i] = reservation.InventoryReservationResult{
			CartItemID: req.CartItemID,
			ProductID:  req.ProductID,
			Qty:        req.Qty,
			Reserved:   true,
		}
	}
	return results, nil
}

type stubOutboxPublisher struct {
	calls int
}

func (s *stubOutboxPublisher) Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error {
	s.calls++
	return nil
}

type stubOrdersRepository struct {
	vendorOrders   map[uuid.UUID]*models.VendorOrder
	lineItems      map[uuid.UUID][]models.OrderLineItem
	paymentIntents map[uuid.UUID]*models.PaymentIntent
	createdGroup   *models.CheckoutGroup
}

func newStubOrdersRepository() *stubOrdersRepository {
	return &stubOrdersRepository{
		vendorOrders:   make(map[uuid.UUID]*models.VendorOrder),
		lineItems:      make(map[uuid.UUID][]models.OrderLineItem),
		paymentIntents: make(map[uuid.UUID]*models.PaymentIntent),
	}
}

func (s *stubOrdersRepository) WithTx(tx *gorm.DB) orders.Repository {
	return s
}

func (s *stubOrdersRepository) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	s.createdGroup = group
	return group, nil
}

func (s *stubOrdersRepository) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	s.vendorOrders[order.ID] = order
	return order, nil
}

func (s *stubOrdersRepository) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	if len(items) == 0 {
		return nil
	}
	orderID := items[0].OrderID
	s.lineItems[orderID] = append(s.lineItems[orderID], items...)
	return nil
}

func (s *stubOrdersRepository) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	s.paymentIntents[intent.OrderID] = intent
	return intent, nil
}

func (s *stubOrdersRepository) FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error) {
	if s.createdGroup == nil || s.createdGroup.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	group := &models.CheckoutGroup{
		ID:           id,
		BuyerStoreID: s.createdGroup.BuyerStoreID,
		CartID:       s.createdGroup.CartID,
	}
	for _, order := range s.vendorOrders {
		copy := *order
		copy.Items = append([]models.OrderLineItem(nil), s.lineItems[order.ID]...)
		if intent, ok := s.paymentIntents[order.ID]; ok {
			copy.PaymentIntent = intent
		}
		group.VendorOrders = append(group.VendorOrders, copy)
	}
	return group, nil
}

func (*stubOrdersRepository) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters orders.BuyerOrderFilters) (*orders.BuyerOrderList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters orders.VendorOrderFilters) (*orders.VendorOrderList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListPayoutOrders(ctx context.Context, params pagination.Params) (*orders.PayoutOrderList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*orders.OrderDetail, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	return errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	return errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	return errors.New("not implemented")
}

func (*stubOrdersRepository) UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	return errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	return errors.New("not implemented")
}
