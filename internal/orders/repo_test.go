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
  checkout_group_id TEXT NOT NULL,
  buyer_store_id TEXT NOT NULL,
  vendor_store_id TEXT NOT NULL,
  status TEXT NOT NULL,
  refund_status TEXT NOT NULL,
  subtotal_cents INTEGER NOT NULL,
  discount_cents INTEGER NOT NULL,
  tax_cents INTEGER NOT NULL,
  transport_fee_cents INTEGER NOT NULL,
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
  name TEXT NOT NULL,
  category TEXT NOT NULL,
  strain TEXT,
  classification TEXT,
  unit TEXT NOT NULL,
  unit_price_cents INTEGER NOT NULL,
  qty INTEGER NOT NULL,
  discount_cents INTEGER NOT NULL,
  total_cents INTEGER NOT NULL,
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
	require.NoError(t, db.Exec(stores).Error)
	require.NoError(t, db.Exec(vendorOrders).Error)
	require.NoError(t, db.Exec(orderLineItems).Error)
	require.NoError(t, db.Exec(paymentIntents).Error)
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
		BuyerStoreID:      buyer.ID,
		VendorStoreID:     vendor.ID,
		Status:            status,
		SubtotalCents:     total,
		DiscountCents:     0,
		TotalCents:        total,
		BalanceDueCents:   total,
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

	item := &models.OrderLineItem{
		OrderID:        order.ID,
		Name:           "Test Item",
		Category:       "test",
		Unit:           enums.ProductUnitUnit,
		UnitPriceCents: 1000,
		Qty:            qty,
		TotalCents:     1000 * qty,
		Status:         enums.LineItemStatusAccepted,
		CreatedAt:      order.CreatedAt,
		UpdatedAt:      order.CreatedAt,
	}
	item.ID = uuid.New()
	require.NoError(t, db.Create(item).Error)
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

func ptr[T any](v T) *T {
	return &v
}
