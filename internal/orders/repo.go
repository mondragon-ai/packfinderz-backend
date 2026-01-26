package orders

import (
	"context"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

// NewRepository builds an orders repository bound to the provided DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) WithTx(tx *gorm.DB) Repository {
	if tx == nil {
		return r
	}
	return &repository{db: tx}
}

func (r *repository) CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error) {
	if err := r.db.WithContext(ctx).Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

func (r *repository) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	if err := r.db.WithContext(ctx).Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func (r *repository) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *repository) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	if err := r.db.WithContext(ctx).Create(intent).Error; err != nil {
		return nil, err
	}
	return intent, nil
}

func (r *repository) FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error) {
	var group models.CheckoutGroup
	err := r.db.WithContext(ctx).
		Preload("VendorOrders.Items").
		Preload("VendorOrders.PaymentIntent").
		Where("id = ?", id).
		First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *repository) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	var orders []models.VendorOrder
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("PaymentIntent").
		Where("checkout_group_id = ?", checkoutGroupID).
		Order("created_at ASC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *repository) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	var items []models.OrderLineItem
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("created_at ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *repository) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	var intent models.PaymentIntent
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		First(&intent).Error
	if err != nil {
		return nil, err
	}
	return &intent, nil
}

func (r *repository) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters BuyerOrderFilters) (*BuyerOrderList, error) {
	pageSize := pagination.NormalizeLimit(params.Limit)
	limitWithBuffer := pagination.LimitWithBuffer(params.Limit)
	if limitWithBuffer <= pageSize {
		limitWithBuffer = pageSize + 1
	}

	cursor, err := pagination.ParseCursor(params.Cursor)
	if err != nil {
		return nil, err
	}

	qb := r.db.WithContext(ctx).Table("vendor_orders AS vo").
		Select(`vo.id,
			vo.created_at,
			vo.order_number,
			vo.total_cents,
			vo.discount_cents,
			vo.fulfillment_status,
			vo.shipping_status,
			pi.status AS payment_status,
			vs.id AS vendor_store_id,
			vs.company_name AS vendor_company_name,
			vs.dba_name AS vendor_dba_name,
			vs.logo_url AS vendor_logo_url,
			(SELECT COALESCE(SUM(qty), 0) FROM order_line_items WHERE order_id = vo.id) AS total_items`).
		Joins("JOIN payment_intents pi ON pi.order_id = vo.id").
		Joins("JOIN stores vs ON vs.id = vo.vendor_store_id").
		Joins("JOIN stores bs ON bs.id = vo.buyer_store_id").
		Where("vo.buyer_store_id = ?", buyerStoreID)

	if filters.OrderStatus != nil {
		qb = qb.Where("vo.status = ?", *filters.OrderStatus)
	}
	if filters.FulfillmentStatus != nil {
		qb = qb.Where("vo.fulfillment_status = ?", *filters.FulfillmentStatus)
	}
	if filters.ShippingStatus != nil {
		qb = qb.Where("vo.shipping_status = ?", *filters.ShippingStatus)
	}
	if filters.PaymentStatus != nil {
		qb = qb.Where("pi.status = ?", *filters.PaymentStatus)
	}
	if filters.DateFrom != nil {
		qb = qb.Where("vo.created_at >= ?", filters.DateFrom)
	}
	if filters.DateTo != nil {
		qb = qb.Where("vo.created_at <= ?", filters.DateTo)
	}

	if q := strings.TrimSpace(filters.Query); q != "" {
		pattern := "%" + strings.ToLower(q) + "%"
		qb = qb.Where(`(
			LOWER(vs.company_name) LIKE ? OR
			LOWER(COALESCE(vs.dba_name, '')) LIKE ? OR
			LOWER(bs.company_name) LIKE ? OR
			LOWER(COALESCE(bs.dba_name, '')) LIKE ?
		)`, pattern, pattern, pattern, pattern)
	}

	if cursor != nil {
		qb = qb.Where("(vo.created_at < ?) OR (vo.created_at = ? AND vo.id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	qb = qb.Order("vo.created_at DESC").Order("vo.id DESC").Limit(limitWithBuffer)

	var records []buyerOrderRecord
	if err := qb.Scan(&records).Error; err != nil {
		return nil, err
	}

	resultRows := records
	nextCursor := ""
	if len(records) > pageSize {
		resultRows = records[:pageSize]
		last := resultRows[len(resultRows)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}

	orders := make([]BuyerOrderSummary, 0, len(resultRows))
	for _, record := range resultRows {
		orders = append(orders, BuyerOrderSummary{
			CreatedAt:         record.CreatedAt,
			OrderNumber:       record.OrderNumber,
			TotalCents:        record.TotalCents,
			DiscountCents:     record.DiscountCents,
			TotalItems:        record.TotalItems,
			PaymentStatus:     record.PaymentStatus,
			FulfillmentStatus: record.FulfillmentStatus,
			ShippingStatus:    record.ShippingStatus,
			Vendor: OrderStoreSummary{
				ID:          record.VendorStoreID,
				CompanyName: record.VendorCompanyName,
				DBAName:     record.VendorDBAName,
				LogoURL:     record.VendorLogoURL,
			},
		})
	}

	return &BuyerOrderList{
		Orders:     orders,
		NextCursor: nextCursor,
	}, nil
}

type buyerOrderRecord struct {
	ID                uuid.UUID
	CreatedAt         time.Time
	OrderNumber       int64
	TotalCents        int
	DiscountCents     int
	FulfillmentStatus enums.VendorOrderFulfillmentStatus
	ShippingStatus    enums.VendorOrderShippingStatus
	PaymentStatus     enums.PaymentStatus
	VendorStoreID     uuid.UUID
	VendorCompanyName string
	VendorDBAName     *string
	VendorLogoURL     *string
	TotalItems        int
}
