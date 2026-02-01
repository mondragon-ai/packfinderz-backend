package orders

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOrdersTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	stores := `
CREATE TABLE IF NOT EXISTS stores (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  company_name TEXT NOT NULL,
  dba_name TEXT,
  description TEXT,
  phone TEXT,
  email TEXT,
  kyc_status TEXT NOT NULL DEFAULT 'pending_verification',
  subscription_active INTEGER NOT NULL DEFAULT 0,
  delivery_radius_meters INTEGER NOT NULL DEFAULT 0,
  address TEXT NOT NULL,
  geom TEXT NOT NULL,
  social TEXT,
  banner_url TEXT,
  logo_url TEXT,
  ratings TEXT,
  categories TEXT,
  owner TEXT NOT NULL,
  last_active_at DATETIME,
  created_at DATETIME,
  updated_at DATETIME
);`
	vendorOrders := `
CREATE TABLE IF NOT EXISTS vendor_orders (
  id TEXT PRIMARY KEY,
  cart_id TEXT NOT NULL,
  checkout_group_id TEXT NOT NULL,
  buyer_store_id TEXT NOT NULL,
  vendor_store_id TEXT NOT NULL,
  currency TEXT NOT NULL DEFAULT 'USD',
  shipping_address TEXT,
  status TEXT NOT NULL,
  refund_status TEXT NOT NULL,
  subtotal_cents INTEGER NOT NULL,
  discounts_cents INTEGER NOT NULL,
  tax_cents INTEGER NOT NULL,
  transport_fee_cents INTEGER NOT NULL,
  warnings TEXT,
  promo TEXT,
  payment_method TEXT NOT NULL DEFAULT 'cash',
  shipping_line TEXT,
  attributed_token TEXT,
  total_cents INTEGER NOT NULL,
  balance_due_cents INTEGER NOT NULL,
  fulfillment_status TEXT NOT NULL,
  shipping_status TEXT NOT NULL,
  order_number INTEGER NOT NULL,
  notes TEXT,
  internal_notes TEXT,
  fulfilled_at DATETIME,
  delivered_at DATETIME,
  canceled_at DATETIME,
  expired_at DATETIME,
  created_at DATETIME,
  updated_at DATETIME
);`
	orderLineItems := `
CREATE TABLE IF NOT EXISTS order_line_items (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL,
  product_id TEXT,
  cart_item_id TEXT,
  name TEXT NOT NULL,
  category TEXT NOT NULL,
  strain TEXT,
  classification TEXT,
  unit TEXT NOT NULL,
  unit_price_cents INTEGER NOT NULL,
  moq INTEGER NOT NULL,
  max_qty INTEGER,
  qty INTEGER NOT NULL,
  discount_cents INTEGER NOT NULL,
  line_subtotal_cents INTEGER NOT NULL,
  total_cents INTEGER NOT NULL,
  warnings TEXT,
  applied_volume_discount TEXT,
  attributed_token TEXT,
  status TEXT NOT NULL,
  notes TEXT,
  created_at DATETIME,
  updated_at DATETIME
);`
	paymentIntents := `
CREATE TABLE IF NOT EXISTS payment_intents (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL,
  method TEXT NOT NULL,
  status TEXT NOT NULL,
  amount_cents INTEGER NOT NULL,
  cash_collected_at DATETIME,
  vendor_paid_at DATETIME,
  created_at DATETIME,
  updated_at DATETIME
);`
	orderAssignments := `
CREATE TABLE IF NOT EXISTS order_assignments (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL,
  agent_user_id TEXT NOT NULL,
  assigned_by_user_id TEXT,
  assigned_at DATETIME NOT NULL,
  unassigned_at DATETIME,
  active INTEGER NOT NULL DEFAULT 1
);`
	require.NoError(t, db.Exec(stores).Error)
	require.NoError(t, db.Exec(vendorOrders).Error)
	require.NoError(t, db.Exec(orderLineItems).Error)
	require.NoError(t, db.Exec(paymentIntents).Error)
	require.NoError(t, db.Exec(orderAssignments).Error)
	return db
}

func newStore(t *testing.T, db *gorm.DB, name string, st enums.StoreType) *models.Store {
	t.Helper()

	store := &models.Store{
		ID:          uuid.New(),
		Type:        st,
		CompanyName: name,
		OwnerID:     uuid.New(),
		Address: types.Address{
			Line1:      "123 Test Ave",
			City:       "Norman",
			State:      "OK",
			PostalCode: "73072",
			Country:    "US",
			Lat:        35.2226,
			Lng:        -97.4395,
		},
		Geom: types.GeographyPoint{
			Lat: 35.2226,
			Lng: -97.4395,
		},
	}
	require.NoError(t, db.Create(store).Error)
	return store
}

func createOrder(t *testing.T, db *gorm.DB, buyer, vendor *models.Store, number int64, created time.Time, qty int, paymentStatus enums.PaymentStatus, status enums.VendorOrderStatus, fulfillment enums.VendorOrderFulfillmentStatus, shipping enums.VendorOrderShippingStatus) *models.VendorOrder {
	t.Helper()

	total := qty * 1000
	order := &models.VendorOrder{
		CartID:            uuid.New(),
		BuyerStoreID:      buyer.ID,
		VendorStoreID:     vendor.ID,
		Status:            status,
		SubtotalCents:     total,
		DiscountsCents:    0,
		TotalCents:        total,
		BalanceDueCents:   total,
		Currency:          enums.CurrencyUSD,
		PaymentMethod:     enums.PaymentMethodCash,
		FulfillmentStatus: fulfillment,
		ShippingStatus:    shipping,
		OrderNumber:       number,
		CreatedAt:         created,
		UpdatedAt:         created,
	}
	order.ID = uuid.New()
	require.NoError(t, db.Create(order).Error)

	createLineItem(t, db, order, qty)

	intent := &models.PaymentIntent{
		OrderID:     order.ID,
		Status:      paymentStatus,
		AmountCents: total,
		CreatedAt:   created,
		UpdatedAt:   created,
	}
	intent.ID = uuid.New()
	require.NoError(t, db.Create(intent).Error)
	return order
}

func createLineItem(t *testing.T, db *gorm.DB, order *models.VendorOrder, qty int) {
	t.Helper()

	cartItemID := uuid.New()
	item := &models.OrderLineItem{
		OrderID:               order.ID,
		CartItemID:            &cartItemID,
		Name:                  "Test Item",
		Category:              "test",
		Unit:                  enums.ProductUnitUnit,
		UnitPriceCents:        1000,
		MOQ:                   1,
		LineSubtotalCents:     1000 * qty,
		Qty:                   qty,
		TotalCents:            1000 * qty,
		Warnings:              nil,
		AppliedVolumeDiscount: nil,
		AttributedToken:       nil,
		Status:                enums.LineItemStatusAccepted,
		CreatedAt:             order.CreatedAt,
		UpdatedAt:             order.CreatedAt,
	}
	item.ID = uuid.New()
	require.NoError(t, db.Create(item).Error)
}

func TestRepository_ListPayoutOrders(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyer := newStore(t, db, "Buyer", enums.StoreTypeBuyer)
	vendor := newStore(t, db, "Vendor", enums.StoreTypeVendor)
	now := time.Now().UTC()

	delivered := now.Add(-time.Minute)
	order := createOrder(t, db, buyer, vendor, 1, now, 1, enums.PaymentStatusSettled, enums.VendorOrderStatusDelivered, enums.VendorOrderFulfillmentStatusFulfilled, enums.VendorOrderShippingStatusDelivered)
	order.DeliveredAt = &delivered
	require.NoError(t, db.Save(order).Error)

	createOrder(t, db, buyer, vendor, 2, now.Add(time.Hour), 1, enums.PaymentStatusSettled, enums.VendorOrderStatusCreatedPending, enums.VendorOrderFulfillmentStatusPending, enums.VendorOrderShippingStatusPending)

	list, err := repo.ListPayoutOrders(context.Background(), pagination.Params{Limit: 10})
	require.NoError(t, err)
	require.Len(t, list.Orders, 1)
	assert.Equal(t, order.ID, list.Orders[0].OrderID)
	assert.Equal(t, vendor.ID, list.Orders[0].VendorStoreID)
	assert.Equal(t, delivered, list.Orders[0].DeliveredAt)
	assert.Equal(t, order.TotalCents, list.Orders[0].AmountCents)
	assert.Empty(t, list.NextCursor)
}

func TestRepository_ListPayoutOrders_Pagination(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyer := newStore(t, db, "Buyer", enums.StoreTypeBuyer)
	vendor := newStore(t, db, "Vendor", enums.StoreTypeVendor)
	now := time.Now().UTC()

	firstDelivered := now.Add(-2 * time.Hour)
	secondDelivered := now.Add(-time.Hour)

	first := createOrder(t, db, buyer, vendor, 1, now, 1, enums.PaymentStatusSettled, enums.VendorOrderStatusDelivered, enums.VendorOrderFulfillmentStatusFulfilled, enums.VendorOrderShippingStatusDelivered)
	first.DeliveredAt = &firstDelivered
	require.NoError(t, db.Save(first).Error)

	second := createOrder(t, db, buyer, vendor, 2, now, 1, enums.PaymentStatusSettled, enums.VendorOrderStatusDelivered, enums.VendorOrderFulfillmentStatusFulfilled, enums.VendorOrderShippingStatusDelivered)
	second.DeliveredAt = &secondDelivered
	require.NoError(t, db.Save(second).Error)

	list, err := repo.ListPayoutOrders(context.Background(), pagination.Params{Limit: 1})
	require.NoError(t, err)
	require.Len(t, list.Orders, 1)
	require.NotEmpty(t, list.NextCursor)
	if !list.Orders[0].DeliveredAt.Equal(firstDelivered) {
		t.Fatalf("expected first page to include earliest delivered order got %v", list.Orders[0].DeliveredAt)
	}

	next, err := repo.ListPayoutOrders(context.Background(), pagination.Params{Limit: 1, Cursor: list.NextCursor})
	require.NoError(t, err)
	require.Len(t, next.Orders, 1)
	if !list.Orders[0].DeliveredAt.Before(next.Orders[0].DeliveredAt) {
		t.Fatalf("expected delivered order timestamps to ascend got %v -> %v", list.Orders[0].DeliveredAt, next.Orders[0].DeliveredAt)
	}
	assert.Empty(t, next.NextCursor)
}

func assignOrder(t *testing.T, db *gorm.DB, orderID, agentID, assignedBy uuid.UUID) {
	t.Helper()

	now := time.Now().UTC()
	require.NoError(t, db.Exec(`
		INSERT INTO order_assignments (id, order_id, agent_user_id, assigned_by_user_id, assigned_at, active)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.New(), orderID, agentID, assignedBy, now, 1).Error)
}

func TestRepositoryListBuyerOrders_pagination(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyer := newStore(t, db, "Buyer One", enums.StoreTypeBuyer)
	vendorA := newStore(t, db, "Vendor A", enums.StoreTypeVendor)
	vendorB := newStore(t, db, "Vendor B", enums.StoreTypeVendor)

	now := time.Now().UTC()
	createOrder(t, db, buyer, vendorA, 1, now.Add(-time.Hour), 2, enums.PaymentStatusSettled, enums.VendorOrderStatusAccepted, enums.VendorOrderFulfillmentStatusPartial, enums.VendorOrderShippingStatusInTransit)
	createOrder(t, db, buyer, vendorB, 2, now, 3, enums.PaymentStatusUnpaid, enums.VendorOrderStatusCreatedPending, enums.VendorOrderFulfillmentStatusPending, enums.VendorOrderShippingStatusPending)
	list, err := repo.ListBuyerOrders(context.Background(), buyer.ID, pagination.Params{Limit: 1}, BuyerOrderFilters{})
	require.NoError(t, err)
	require.Len(t, list.Orders, 1)
	assert.NotEmpty(t, list.NextCursor)
	assert.Equal(t, int64(2), list.Orders[0].OrderNumber)
	assert.Equal(t, "Vendor B", list.Orders[0].Vendor.CompanyName)
	assert.Equal(t, enums.PaymentStatusUnpaid, list.Orders[0].PaymentStatus)

	second, err := repo.ListBuyerOrders(context.Background(), buyer.ID, pagination.Params{Limit: 1, Cursor: list.NextCursor}, BuyerOrderFilters{})
	require.NoError(t, err)
	require.Len(t, second.Orders, 1)
	assert.Equal(t, int64(1), second.Orders[0].OrderNumber)
	assert.Equal(t, "Vendor A", second.Orders[0].Vendor.CompanyName)
	assert.Equal(t, enums.PaymentStatusSettled, second.Orders[0].PaymentStatus)
	assert.Empty(t, second.NextCursor)
}

func TestRepositoryListBuyerOrders_filtersAndSearch(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyer := newStore(t, db, "Buyer Two", enums.StoreTypeBuyer)
	vendor := newStore(t, db, "Search Vendor", enums.StoreTypeVendor)

	now := time.Now().UTC()
	createOrder(t, db, buyer, vendor, 5, now, 4, enums.PaymentStatusPaid, enums.VendorOrderStatusFulfilled, enums.VendorOrderFulfillmentStatusFulfilled, enums.VendorOrderShippingStatusDelivered)

	filters := BuyerOrderFilters{
		Query:             "search vendor",
		PaymentStatus:     ptr(enums.PaymentStatusPaid),
		FulfillmentStatus: ptr(enums.VendorOrderFulfillmentStatusFulfilled),
		ShippingStatus:    ptr(enums.VendorOrderShippingStatusDelivered),
	}
	list, err := repo.ListBuyerOrders(context.Background(), buyer.ID, pagination.Params{Limit: 10}, filters)
	require.NoError(t, err)
	require.Len(t, list.Orders, 1)
	assert.Equal(t, "Search Vendor", list.Orders[0].Vendor.CompanyName)
	assert.Equal(t, 4, list.Orders[0].TotalItems)
	assert.Empty(t, list.NextCursor)
}

func TestRepositoryListVendorOrders_pagination(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyerA := newStore(t, db, "Buyer A", enums.StoreTypeBuyer)
	buyerB := newStore(t, db, "Buyer B", enums.StoreTypeBuyer)
	vendor := newStore(t, db, "Vendor Portal", enums.StoreTypeVendor)

	now := time.Now().UTC()
	createOrder(t, db, buyerA, vendor, 3, now.Add(-time.Hour), 1, enums.PaymentStatusPaid, enums.VendorOrderStatusFulfilled, enums.VendorOrderFulfillmentStatusFulfilled, enums.VendorOrderShippingStatusDelivered)
	createOrder(t, db, buyerB, vendor, 4, now, 2, enums.PaymentStatusUnpaid, enums.VendorOrderStatusCreatedPending, enums.VendorOrderFulfillmentStatusPending, enums.VendorOrderShippingStatusPending)

	list, err := repo.ListVendorOrders(context.Background(), vendor.ID, pagination.Params{Limit: 1}, VendorOrderFilters{})
	require.NoError(t, err)
	require.Len(t, list.Orders, 1)
	assert.Equal(t, int64(4), list.Orders[0].OrderNumber)
	assert.Equal(t, "Buyer B", list.Orders[0].Buyer.CompanyName)
	assert.NotEmpty(t, list.NextCursor)

	second, err := repo.ListVendorOrders(context.Background(), vendor.ID, pagination.Params{Limit: 1, Cursor: list.NextCursor}, VendorOrderFilters{})
	require.NoError(t, err)
	require.Len(t, second.Orders, 1)
	assert.Equal(t, int64(3), second.Orders[0].OrderNumber)
	assert.Equal(t, "Buyer A", second.Orders[0].Buyer.CompanyName)
	assert.Empty(t, second.NextCursor)
}

func TestRepositoryListVendorOrders_filtersAndSearch(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyer := newStore(t, db, "Search Buyer", enums.StoreTypeBuyer)
	vendor := newStore(t, db, "Portal Vendor", enums.StoreTypeVendor)

	now := time.Now().UTC()
	createOrder(t, db, buyer, vendor, 6, now, 3, enums.PaymentStatusUnpaid, enums.VendorOrderStatusCreatedPending, enums.VendorOrderFulfillmentStatusPending, enums.VendorOrderShippingStatusPending)

	filters := VendorOrderFilters{
		ActionableStatuses: []enums.VendorOrderStatus{enums.VendorOrderStatusCreatedPending},
		PaymentStatus:      ptr(enums.PaymentStatusUnpaid),
		FulfillmentStatus:  ptr(enums.VendorOrderFulfillmentStatusPending),
		ShippingStatus:     ptr(enums.VendorOrderShippingStatusPending),
		Query:              "search buyer",
	}

	list, err := repo.ListVendorOrders(context.Background(), vendor.ID, pagination.Params{Limit: 10}, filters)
	require.NoError(t, err)
	require.Len(t, list.Orders, 1)
	assert.Equal(t, "Search Buyer", list.Orders[0].Buyer.CompanyName)
	assert.Equal(t, enums.PaymentStatusUnpaid, list.Orders[0].PaymentStatus)
	assert.Empty(t, list.NextCursor)
}

func TestRepositoryFindOrderDetail(t *testing.T) {
	db := setupOrdersTestDB(t)
	repo := NewRepository(db)

	buyer := newStore(t, db, "Detail Buyer", enums.StoreTypeBuyer)
	vendor := newStore(t, db, "Detail Vendor", enums.StoreTypeVendor)

	now := time.Now().UTC()
	order := createOrder(t, db, buyer, vendor, 10, now, 2, enums.PaymentStatusPending, enums.VendorOrderStatusAccepted, enums.VendorOrderFulfillmentStatusPartial, enums.VendorOrderShippingStatusInTransit)
	assignOrder(t, db, order.ID, uuid.New(), uuid.New())

	detail, err := repo.FindOrderDetail(context.Background(), order.ID)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.NotNil(t, detail.Order)
	assert.Equal(t, int64(10), detail.Order.OrderNumber)
	assert.Equal(t, "Detail Buyer", detail.BuyerStore.CompanyName)
	assert.Equal(t, "Detail Vendor", detail.VendorStore.CompanyName)
	require.Len(t, detail.LineItems, 1)
	require.NotNil(t, detail.PaymentIntent)
	require.NotNil(t, detail.ActiveAssignment)
}

func ptr[T any](v T) *T {
	return &v
}
