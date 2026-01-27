package ledger

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository manages persistence for ledger events.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	Create(ctx context.Context, event *models.LedgerEvent) error
	ListByOrderID(ctx context.Context, orderID uuid.UUID) ([]models.LedgerEvent, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns a ledger repository bound to the provided database.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) WithTx(tx *gorm.DB) Repository {
	if tx == nil {
		return r
	}
	return &repository{db: tx}
}

func (r *repository) Create(ctx context.Context, event *models.LedgerEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *repository) ListByOrderID(ctx context.Context, orderID uuid.UUID) ([]models.LedgerEvent, error) {
	var events []models.LedgerEvent
	if err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
