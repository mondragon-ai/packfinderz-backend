package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxLastErrorLen = 1024

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(tx *gorm.DB, event models.OutboxEvent) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	return tx.Create(&event).Error
}

func (r *Repository) FetchUnpublished(limit int) ([]models.OutboxEvent, error) {
	var rows []models.OutboxEvent
	err := r.db.Where("published_at IS NULL").
		Order("created_at ASC").
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) FetchUnpublishedForPublish(tx *gorm.DB, limit, maxAttempts int) ([]models.OutboxEvent, error) {
	if tx == nil {
		return nil, errors.New("transaction required")
	}
	if limit <= 0 {
		limit = 1
	}

	query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("published_at IS NULL")
	if maxAttempts > 0 {
		query = query.Where("attempt_count < ?", maxAttempts)
	}

	var rows []models.OutboxEvent
	err := query.
		Order("created_at ASC").
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) MarkPublished(id uuid.UUID) error {
	return r.MarkPublishedTx(r.db, id)
}

func (r *Repository) MarkPublishedTx(tx *gorm.DB, id uuid.UUID) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	return tx.Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"published_at": time.Now(),
		}).Error
}

func (r *Repository) MarkFailed(id uuid.UUID, err error) error {
	return r.MarkFailedTx(r.db, id, err)
}

func (r *Repository) MarkFailedTx(tx *gorm.DB, id uuid.UUID, err error) error {
	if tx == nil {
		return errors.New("transaction required")
	}

	lastErr := ""
	if err != nil {
		lastErr = truncateError(err.Error())
	}
	var lastErrPtr *string
	if lastErr != "" {
		lastErrPtr = new(string)
		*lastErrPtr = lastErr
	}

	return tx.Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_error":    lastErrPtr,
			"attempt_count": gorm.Expr("attempt_count + 1"),
		}).Error
}

func (r *Repository) MarkTerminalTx(tx *gorm.DB, id uuid.UUID, err error, terminalAttempts int) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	if terminalAttempts <= 0 {
		terminalAttempts = 1
	}

	lastErr := ""
	if err != nil {
		lastErr = truncateError(err.Error())
	}
	var lastErrPtr *string
	if lastErr != "" {
		lastErrPtr = new(string)
		*lastErrPtr = lastErr
	}

	return tx.Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_error":    lastErrPtr,
			"attempt_count": gorm.Expr("GREATEST(attempt_count, ?)", terminalAttempts),
		}).Error
}

func (r *Repository) Exists(ctx context.Context, eventType enums.OutboxEventType, aggregateType enums.OutboxAggregateType, aggregateID uuid.UUID) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("event_type = ? AND aggregate_type = ? AND aggregate_id = ?", eventType, aggregateType, aggregateID).
		Count(&count).Error
	return count > 0, err
}

// ExistsTx checks for an existing event using the provided transaction.
func (r *Repository) ExistsTx(tx *gorm.DB, eventType enums.OutboxEventType, aggregateType enums.OutboxAggregateType, aggregateID uuid.UUID) (bool, error) {
	if tx == nil {
		return false, errors.New("transaction required")
	}
	var count int64
	err := tx.Model(&models.OutboxEvent{}).
		Where("event_type = ? AND aggregate_type = ? AND aggregate_id = ?", eventType, aggregateType, aggregateID).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) DeletePublishedBefore(ctx context.Context, tx *gorm.DB, cutoff time.Time, minAttemptCount int) (int64, error) {
	db := r.db
	if tx != nil {
		db = tx
	}
	query := db.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("published_at IS NOT NULL").
		Where("published_at < ?", cutoff)
	if minAttemptCount > 0 {
		query = query.Where("attempt_count >= ?", minAttemptCount)
	}
	result := query.Delete(&models.OutboxEvent{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func truncateError(message string) string {
	if len(message) <= maxLastErrorLen {
		return message
	}
	return message[:maxLastErrorLen]
}
