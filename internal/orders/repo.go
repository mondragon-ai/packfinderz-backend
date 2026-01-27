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

func (r *repository) FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error) {
	var item models.OrderLineItem
	err := r.db.WithContext(ctx).
		Where("id = ?", lineItemID).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
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

func (r *repository) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	var order models.VendorOrder
	err := r.db.WithContext(ctx).
		Where("id = ?", orderID).
		First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *repository) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.VendorOrder{}).
		Where("id = ?", orderID).
		Updates(map[string]any{
			"status": status,
		}).Error
}

func (r *repository) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	updates := map[string]any{
		"status": status,
	}
	if notes != nil {
		updates["notes"] = notes
	}
	return r.db.WithContext(ctx).
		Model(&models.OrderLineItem{}).
		Where("id = ?", lineItemID).
		Updates(updates).Error
}

func (r *repository) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.VendorOrder{}).
		Where("id = ?", orderID).
		Updates(updates).Error
}

func (r *repository) UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.PaymentIntent{}).
		Where("order_id = ?", orderID).
		Updates(updates).Error
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

func (r *repository) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters VendorOrderFilters) (*VendorOrderList, error) {
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
			bs.id AS buyer_store_id,
			bs.company_name AS buyer_company_name,
			bs.dba_name AS buyer_dba_name,
			bs.logo_url AS buyer_logo_url,
			(SELECT COALESCE(SUM(qty), 0) FROM order_line_items WHERE order_id = vo.id) AS total_items`).
		Joins("JOIN payment_intents pi ON pi.order_id = vo.id").
		Joins("JOIN stores bs ON bs.id = vo.buyer_store_id").
		Joins("JOIN stores vs ON vs.id = vo.vendor_store_id").
		Where("vo.vendor_store_id = ?", vendorStoreID)

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
	if len(filters.ActionableStatuses) > 0 {
		qb = qb.Where("vo.status IN ?", filters.ActionableStatuses)
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

	var records []vendorOrderRecord
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

	orders := make([]VendorOrderSummary, 0, len(resultRows))
	for _, record := range resultRows {
		orders = append(orders, VendorOrderSummary{
			CreatedAt:         record.CreatedAt,
			OrderNumber:       record.OrderNumber,
			TotalCents:        record.TotalCents,
			DiscountCents:     record.DiscountCents,
			TotalItems:        record.TotalItems,
			PaymentStatus:     record.PaymentStatus,
			FulfillmentStatus: record.FulfillmentStatus,
			ShippingStatus:    record.ShippingStatus,
			Buyer: OrderStoreSummary{
				ID:          record.BuyerStoreID,
				CompanyName: record.BuyerCompanyName,
				DBAName:     record.BuyerDBAName,
				LogoURL:     record.BuyerLogoURL,
			},
		})
	}

	return &VendorOrderList{
		Orders:     orders,
		NextCursor: nextCursor,
	}, nil
}

func (r *repository) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*AgentOrderQueueList, error) {
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
			bs.id AS buyer_store_id,
			bs.company_name AS buyer_company_name,
			bs.dba_name AS buyer_dba_name,
			bs.logo_url AS buyer_logo_url,
			vs.id AS vendor_store_id,
			vs.company_name AS vendor_company_name,
			vs.dba_name AS vendor_dba_name,
			vs.logo_url AS vendor_logo_url,
			(SELECT COALESCE(SUM(qty), 0) FROM order_line_items WHERE order_id = vo.id) AS total_items`).
		Joins("JOIN payment_intents pi ON pi.order_id = vo.id").
		Joins("JOIN stores bs ON bs.id = vo.buyer_store_id").
		Joins("JOIN stores vs ON vs.id = vo.vendor_store_id").
		Joins("JOIN order_assignments oa ON oa.order_id = vo.id AND oa.active = true").
		Where("oa.agent_user_id = ?", agentID)

	if cursor != nil {
		qb = qb.Where("(vo.created_at < ?) OR (vo.created_at = ? AND vo.id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	qb = qb.Order("vo.created_at DESC").Order("vo.id DESC").Limit(limitWithBuffer)

	var records []agentOrderQueueRecord
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

	orders := make([]AgentOrderQueueSummary, 0, len(resultRows))
	for _, record := range resultRows {
		orders = append(orders, AgentOrderQueueSummary{
			OrderID:           record.ID,
			OrderNumber:       record.OrderNumber,
			CreatedAt:         record.CreatedAt,
			TotalCents:        record.TotalCents,
			DiscountCents:     record.DiscountCents,
			TotalItems:        record.TotalItems,
			PaymentStatus:     record.PaymentStatus,
			FulfillmentStatus: record.FulfillmentStatus,
			ShippingStatus:    record.ShippingStatus,
			Buyer: OrderStoreSummary{
				ID:          record.BuyerStoreID,
				CompanyName: record.BuyerCompanyName,
				DBAName:     record.BuyerDBAName,
				LogoURL:     record.BuyerLogoURL,
			},
			Vendor: OrderStoreSummary{
				ID:          record.VendorStoreID,
				CompanyName: record.VendorCompanyName,
				DBAName:     record.VendorDBAName,
				LogoURL:     record.VendorLogoURL,
			},
		})
	}

	return &AgentOrderQueueList{
		Orders:     orders,
		NextCursor: nextCursor,
	}, nil
}

func (r *repository) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*AgentOrderQueueList, error) {
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
			bs.id AS buyer_store_id,
			bs.company_name AS buyer_company_name,
			bs.dba_name AS buyer_dba_name,
			bs.logo_url AS buyer_logo_url,
			vs.id AS vendor_store_id,
			vs.company_name AS vendor_company_name,
			vs.dba_name AS vendor_dba_name,
			vs.logo_url AS vendor_logo_url,
			(SELECT COALESCE(SUM(qty), 0) FROM order_line_items WHERE order_id = vo.id) AS total_items`).
		Joins("JOIN payment_intents pi ON pi.order_id = vo.id").
		Joins("JOIN stores bs ON bs.id = vo.buyer_store_id").
		Joins("JOIN stores vs ON vs.id = vo.vendor_store_id").
		Joins("LEFT JOIN order_assignments oa ON oa.order_id = vo.id AND oa.active = true").
		Where("vo.status = ?", enums.VendorOrderStatusHold).
		Where("oa.order_id IS NULL")

	if cursor != nil {
		qb = qb.Where("(vo.created_at < ?) OR (vo.created_at = ? AND vo.id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	qb = qb.Order("vo.created_at DESC").Order("vo.id DESC").Limit(limitWithBuffer)

	var records []agentOrderQueueRecord
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

	orders := make([]AgentOrderQueueSummary, 0, len(resultRows))
	for _, record := range resultRows {
		orders = append(orders, AgentOrderQueueSummary{
			OrderID:           record.ID,
			OrderNumber:       record.OrderNumber,
			CreatedAt:         record.CreatedAt,
			TotalCents:        record.TotalCents,
			DiscountCents:     record.DiscountCents,
			TotalItems:        record.TotalItems,
			PaymentStatus:     record.PaymentStatus,
			FulfillmentStatus: record.FulfillmentStatus,
			ShippingStatus:    record.ShippingStatus,
			Buyer: OrderStoreSummary{
				ID:          record.BuyerStoreID,
				CompanyName: record.BuyerCompanyName,
				DBAName:     record.BuyerDBAName,
				LogoURL:     record.BuyerLogoURL,
			},
			Vendor: OrderStoreSummary{
				ID:          record.VendorStoreID,
				CompanyName: record.VendorCompanyName,
				DBAName:     record.VendorDBAName,
				LogoURL:     record.VendorLogoURL,
			},
		})
	}

	return &AgentOrderQueueList{
		Orders:     orders,
		NextCursor: nextCursor,
	}, nil
}

type payoutOrderRecord struct {
	ID            uuid.UUID
	OrderNumber   int64
	VendorStoreID uuid.UUID
	DeliveredAt   time.Time
	AmountCents   int
}

func (r *repository) ListPayoutOrders(ctx context.Context, params pagination.Params) (*PayoutOrderList, error) {
	limitWithBuffer := pagination.LimitWithBuffer(params.Limit)
	cursor, err := pagination.ParseCursor(strings.TrimSpace(params.Cursor))
	if err != nil {
		return nil, err
	}

	var records []payoutOrderRecord
	qb := r.db.WithContext(ctx).Table("vendor_orders vo").
		Select("vo.id, vo.order_number, vo.vendor_store_id, vo.delivered_at, pi.amount_cents").
		Joins("JOIN payment_intents pi ON pi.order_id = vo.id").
		Where("vo.status = ?", enums.VendorOrderStatusDelivered).
		Where("pi.status = ?", enums.PaymentStatusSettled)

	if cursor != nil {
		qb = qb.Where("(vo.delivered_at > ?) OR (vo.delivered_at = ? AND vo.id > ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	if err := qb.Order("vo.delivered_at ASC").Order("vo.id ASC").Limit(limitWithBuffer).Scan(&records).Error; err != nil {
		return nil, err
	}

	nextCursor := ""
	if len(records) == limitWithBuffer {
		last := records[len(records)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{CreatedAt: last.DeliveredAt, ID: last.ID})
		records = records[:len(records)-1]
	}

	list := &PayoutOrderList{
		Orders: make([]PayoutOrderSummary, 0, len(records)),
	}
	for _, rec := range records {
		list.Orders = append(list.Orders, PayoutOrderSummary{
			OrderID:       rec.ID,
			VendorStoreID: rec.VendorStoreID,
			OrderNumber:   rec.OrderNumber,
			AmountCents:   rec.AmountCents,
			DeliveredAt:   rec.DeliveredAt,
		})
	}
	list.NextCursor = nextCursor
	return list, nil
}

func (r *repository) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*OrderDetail, error) {
	var order models.VendorOrder
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("PaymentIntent").
		Preload("Assignments", "active = ?", true).
		Where("id = ?", orderID).
		First(&order).Error
	if err != nil {
		return nil, err
	}

	buyer, err := r.loadStoreSummary(ctx, order.BuyerStoreID)
	if err != nil {
		return nil, err
	}
	vendor, err := r.loadStoreSummary(ctx, order.VendorStoreID)
	if err != nil {
		return nil, err
	}

	var assignment *OrderAssignmentSummary
	if len(order.Assignments) > 0 {
		assignment = buildAssignmentSummary(&order.Assignments[0])
	}

	lineItems := make([]LineItemDetail, 0, len(order.Items))
	for _, item := range order.Items {
		lineItems = append(lineItems, buildLineItemDetail(item))
	}

	var payment *PaymentIntentDetail
	if order.PaymentIntent != nil {
		payment = buildPaymentIntentDetail(order.PaymentIntent)
	}

	return &OrderDetail{
		Order:            buildVendorOrderSummary(&order),
		LineItems:        lineItems,
		PaymentIntent:    payment,
		BuyerStore:       buyer,
		VendorStore:      vendor,
		ActiveAssignment: assignment,
	}, nil
}

func (r *repository) loadStoreSummary(ctx context.Context, storeID uuid.UUID) (OrderStoreSummary, error) {
	var store models.Store
	if err := r.db.WithContext(ctx).
		Select("id", "company_name", "dba_name", "logo_url").
		Where("id = ?", storeID).
		First(&store).Error; err != nil {
		return OrderStoreSummary{}, err
	}
	return OrderStoreSummary{
		ID:          store.ID,
		CompanyName: store.CompanyName,
		DBAName:     store.DBAName,
		LogoURL:     store.LogoURL,
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

type vendorOrderRecord struct {
	ID                uuid.UUID
	CreatedAt         time.Time
	OrderNumber       int64
	TotalCents        int
	DiscountCents     int
	FulfillmentStatus enums.VendorOrderFulfillmentStatus
	ShippingStatus    enums.VendorOrderShippingStatus
	PaymentStatus     enums.PaymentStatus
	BuyerStoreID      uuid.UUID
	BuyerCompanyName  string
	BuyerDBAName      *string
	BuyerLogoURL      *string
	TotalItems        int
}

type agentOrderQueueRecord struct {
	ID                uuid.UUID
	CreatedAt         time.Time
	OrderNumber       int64
	TotalCents        int
	DiscountCents     int
	FulfillmentStatus enums.VendorOrderFulfillmentStatus
	ShippingStatus    enums.VendorOrderShippingStatus
	PaymentStatus     enums.PaymentStatus
	BuyerStoreID      uuid.UUID
	BuyerCompanyName  string
	BuyerDBAName      *string
	BuyerLogoURL      *string
	VendorStoreID     uuid.UUID
	VendorCompanyName string
	VendorDBAName     *string
	VendorLogoURL     *string
	TotalItems        int
}

func buildVendorOrderSummary(order *models.VendorOrder) *VendorOrderSummary {
	if order == nil {
		return nil
	}
	return &VendorOrderSummary{
		Status:            order.Status,
		OrderNumber:       order.OrderNumber,
		CreatedAt:         order.CreatedAt,
		TotalCents:        order.TotalCents,
		DiscountCents:     order.DiscountCents,
		TotalItems:        sumOrderItems(order.Items),
		PaymentStatus:     paymentStatus(order.PaymentIntent),
		FulfillmentStatus: order.FulfillmentStatus,
		ShippingStatus:    order.ShippingStatus,
		DeliveredAt:       order.DeliveredAt,
	}
}

func sumOrderItems(items []models.OrderLineItem) int {
	total := 0
	for _, item := range items {
		total += item.Qty
	}
	return total
}

func paymentStatus(intent *models.PaymentIntent) enums.PaymentStatus {
	if intent == nil {
		return enums.PaymentStatusUnpaid
	}
	return intent.Status
}
func buildLineItemDetail(item models.OrderLineItem) LineItemDetail {
	return LineItemDetail{
		ID:             item.ID,
		Name:           item.Name,
		Category:       item.Category,
		Strain:         item.Strain,
		Classification: item.Classification,
		Unit:           string(item.Unit),
		UnitPriceCents: item.UnitPriceCents,
		Qty:            item.Qty,
		DiscountCents:  item.DiscountCents,
		TotalCents:     item.TotalCents,
		Status:         string(item.Status),
		Notes:          item.Notes,
	}
}

func buildPaymentIntentDetail(intent *models.PaymentIntent) *PaymentIntentDetail {
	if intent == nil {
		return nil
	}
	return &PaymentIntentDetail{
		ID:              intent.ID,
		Method:          string(intent.Method),
		Status:          string(intent.Status),
		AmountCents:     intent.AmountCents,
		CashCollectedAt: intent.CashCollectedAt,
		VendorPaidAt:    intent.VendorPaidAt,
	}
}

func buildAssignmentSummary(assignment *models.OrderAssignment) *OrderAssignmentSummary {
	if assignment == nil {
		return nil
	}
	return &OrderAssignmentSummary{
		ID:                      assignment.ID,
		AgentUserID:             assignment.AgentUserID,
		AssignedByUserID:        assignment.AssignedByUserID,
		AssignedAt:              assignment.AssignedAt,
		UnassignedAt:            assignment.UnassignedAt,
		PickupTime:              assignment.PickupTime,
		DeliveryTime:            assignment.DeliveryTime,
		CashPickupTime:          assignment.CashPickupTime,
		PickupSignatureGCSKey:   assignment.PickupSignatureGCSKey,
		DeliverySignatureGCSKey: assignment.DeliverySignatureGCSKey,
	}
}

func (r *repository) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.OrderAssignment{}).
		Where("id = ?", assignmentID).
		Updates(updates).Error
}
