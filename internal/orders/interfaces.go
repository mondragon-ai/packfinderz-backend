package orders

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines persistence operations for checkout/order tables.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error)
	CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error
	CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error)
	FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error)
	FindVendorOrderByCheckoutGroupAndVendor(ctx context.Context, checkoutGroupID, vendorStoreID uuid.UUID) (*models.VendorOrder, error)
	FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error)
	FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error)
	FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error)
	ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters BuyerOrderFilters) (*BuyerOrderList, error)
	ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters VendorOrderFilters) (*VendorOrderList, error)
	ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*AgentOrderQueueList, error)
	ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*AgentOrderQueueList, error)
	ListPayoutOrders(ctx context.Context, params pagination.Params) (*PayoutOrderList, error)
	FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*OrderDetail, error)
	FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error)
	FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error)
	UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error
	UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error
	UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error
	UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error
	UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error
}
