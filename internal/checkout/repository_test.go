package checkout

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestRepositoryFindByCheckoutGroupIDReturnsNilWhenNoOrders(t *testing.T) {
	repo := &repository{
		db:         (*gorm.DB)(nil),
		orders:     &stubOrdersRepo{},
		cartLoader: &stubCartLoader{},
	}

	result, err := repo.FindByCheckoutGroupID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result when no orders, got %+v", result)
	}
}

func TestRepositoryFindByCheckoutGroupIDUsesCartRecord(t *testing.T) {
	groupID := uuid.New()
	vendorOrder := models.VendorOrder{
		ID:              uuid.New(),
		CheckoutGroupID: groupID,
		BuyerStoreID:    uuid.New(),
		VendorStoreID:   uuid.New(),
		CartID:          uuid.New(),
		Status:          enums.VendorOrderStatusCreatedPending,
	}
	cartRecord := &models.CartRecord{
		ID:              vendorOrder.CartID,
		BuyerStoreID:    vendorOrder.BuyerStoreID,
		CheckoutGroupID: &groupID,
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorOrder.VendorStoreID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 1000,
				TotalCents:    1000,
			},
		},
	}

	repo := &repository{
		db:     (*gorm.DB)(nil),
		orders: &stubOrdersRepo{vendorOrders: map[uuid.UUID][]models.VendorOrder{groupID: {vendorOrder}}},
		cartLoader: &stubCartLoader{
			byCheckout: map[uuid.UUID]*models.CartRecord{groupID: cartRecord},
		},
	}

	result, err := repo.FindByCheckoutGroupID(context.Background(), groupID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected checkout group")
	}
	if result.BuyerStoreID != cartRecord.BuyerStoreID {
		t.Fatalf("buyer store mismatch: expected %s got %s", cartRecord.BuyerStoreID, result.BuyerStoreID)
	}
	if result.CartID == nil || *result.CartID != cartRecord.ID {
		t.Fatalf("cart id mismatch: expected %s got %v", cartRecord.ID, result.CartID)
	}
	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected vendor orders, got %d", len(result.VendorOrders))
	}
	if len(result.CartVendorGroups) != len(cartRecord.VendorGroups) {
		t.Fatalf("expected cart vendor groups copied")
	}
}

func TestRepositoryFindByCartIDDelegatesToCheckoutGroup(t *testing.T) {
	groupID := uuid.New()
	cartID := uuid.New()
	cartRecord := &models.CartRecord{
		ID:              cartID,
		CheckoutGroupID: &groupID,
		VendorGroups:    []models.CartVendorGroup{},
	}

	repo := &repository{
		db:     (*gorm.DB)(nil),
		orders: &stubOrdersRepo{vendorOrders: map[uuid.UUID][]models.VendorOrder{groupID: {{ID: uuid.New(), CheckoutGroupID: groupID, BuyerStoreID: uuid.New()}}}},
		cartLoader: &stubCartLoader{
			byID: map[uuid.UUID]*models.CartRecord{cartID: cartRecord},
		},
	}

	result, err := repo.FindByCartID(context.Background(), cartID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected checkout group")
	}
	if result.ID != groupID {
		t.Fatalf("expected group id %s got %s", groupID, result.ID)
	}
}

type stubOrdersRepo struct {
	vendorOrders map[uuid.UUID][]models.VendorOrder
}

func (s *stubOrdersRepo) WithTx(tx *gorm.DB) orders.Repository {
	return s
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

func (s *stubOrdersRepo) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	return s.vendorOrders[checkoutGroupID], nil
}

func (s *stubOrdersRepo) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters orders.BuyerOrderFilters) (*orders.BuyerOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters orders.VendorOrderFilters) (*orders.VendorOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) ListPayoutOrders(ctx context.Context, params pagination.Params) (*orders.PayoutOrderList, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*orders.OrderDetail, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	panic("not implemented")
}

func (s *stubOrdersRepo) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	panic("not implemented")
}

func (s *stubOrdersRepo) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	panic("not implemented")
}

func (s *stubOrdersRepo) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	panic("not implemented")
}

func (s *stubOrdersRepo) UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	panic("not implemented")
}

func (s *stubOrdersRepo) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	panic("not implemented")
}

type stubCartLoader struct {
	byCheckout map[uuid.UUID]*models.CartRecord
	byID       map[uuid.UUID]*models.CartRecord
}

func (s *stubCartLoader) WithTx(tx *gorm.DB) cartLoader {
	return s
}

func (s *stubCartLoader) LoadByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) (*models.CartRecord, error) {
	record, ok := s.byCheckout[checkoutGroupID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return record, nil
}

func (s *stubCartLoader) LoadByID(ctx context.Context, cartID uuid.UUID) (*models.CartRecord, error) {
	record, ok := s.byID[cartID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return record, nil
}
