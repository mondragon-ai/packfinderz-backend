package reservation

import (
	"context"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InventoryReservationRequest describes the data required to reserve inventory for a product.
type InventoryReservationRequest struct {
	CartItemID uuid.UUID
	ProductID  uuid.UUID
	Qty        int
}

// InventoryReservationResult reports whether a reservation succeeded per line item.
type InventoryReservationResult struct {
	CartItemID uuid.UUID
	ProductID  uuid.UUID
	Qty        int
	Reserved   bool
	Reason     string
}

// ReserveInventory atomically decrements available inventory and increments reserved qty per request.
func ReserveInventory(ctx context.Context, db *gorm.DB, requests []InventoryReservationRequest) ([]InventoryReservationResult, error) {
	if db == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "database required for reservation")
	}
	results := make([]InventoryReservationResult, len(requests))
	tx := db.WithContext(ctx)
	for i, req := range requests {
		if req.Qty <= 0 {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "reservation quantity must be positive")
		}

		res := tx.Exec(
			`UPDATE inventory_items
       SET available_qty = available_qty - ?, reserved_qty = reserved_qty + ?, updated_at = CURRENT_TIMESTAMP
       WHERE product_id = ? AND available_qty >= ?`,
			req.Qty,
			req.Qty,
			req.ProductID,
			req.Qty,
		)
		if res.Error != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, res.Error, "reserve inventory")
		}

		result := InventoryReservationResult{
			CartItemID: req.CartItemID,
			ProductID:  req.ProductID,
			Qty:        req.Qty,
		}
		if res.RowsAffected == 0 {
			result.Reserved = false
			result.Reason = "insufficient_inventory"
		} else {
			result.Reserved = true
		}
		results[i] = result
	}
	return results, nil
}
