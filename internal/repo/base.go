package repo

import (
	"context"

	"gorm.io/gorm"
)

// Base provides a shared foundation for domain repositories.
type Base struct {
	db *gorm.DB
}

// NewBase constructs a Base repository backed by the provided GORM connection.
func NewBase(db *gorm.DB) Base {
	return Base{db: db}
}

// DB returns the GORM connection bound to the supplied context (if any).
func (b Base) DB(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return b.db
	}
	return b.db.WithContext(ctx)
}
