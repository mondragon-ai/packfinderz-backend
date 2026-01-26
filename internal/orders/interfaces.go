package orders

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines persistence operations for checkout/order tables.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	CreateCheckoutGroup(ctx context.Context, group *models.CheckoutGroup) (*models.CheckoutGroup, error)
	CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error)
	CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error
	CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error)
	FindCheckoutGroupByID(ctx context.Context, id uuid.UUID) (*models.CheckoutGroup, error)
	FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error)
	FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error)
	FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error)
	ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters BuyerOrderFilters) (*BuyerOrderList, error)
	ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters VendorOrderFilters) (*VendorOrderList, error)
}
