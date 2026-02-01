package cart

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CartRepository defines the persistence surface required by the cart service.
type CartRepository interface {
	WithTx(tx *gorm.DB) CartRepository
	FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error)
	FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error)
	Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error)
	Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error)
	ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error
	ReplaceVendorGroups(ctx context.Context, cartID uuid.UUID, groups []models.CartVendorGroup) error
	UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error
}
