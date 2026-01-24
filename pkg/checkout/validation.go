package checkout

import (
	"fmt"

	"github.com/google/uuid"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

// MOQValidationInput describes the data required to verify a line item's MOQ.
type MOQValidationInput struct {
	ProductID   uuid.UUID
	ProductName string
	MOQ         int
	Quantity    int
}

// MOQViolationDetail exposes the data returned to callers when a validation fails.
type MOQViolationDetail struct {
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name,omitempty"`
	RequiredQty  int       `json:"required_qty"`
	RequestedQty int       `json:"requested_qty"`
}

// ValidateMOQ ensures every provided line item meets its product's minimum order quantity.
func ValidateMOQ(items []MOQValidationInput) error {
	var violations []MOQViolationDetail
	for _, item := range items {
		if item.MOQ <= 1 {
			continue
		}
		if item.Quantity < item.MOQ {
			violations = append(violations, MOQViolationDetail{
				ProductID:    item.ProductID,
				ProductName:  item.ProductName,
				RequiredQty:  item.MOQ,
				RequestedQty: item.Quantity,
			})
		}
	}
	if len(violations) == 0 {
		return nil
	}
	return pkgerrors.New(pkgerrors.CodeStateConflict, fmt.Sprintf("minimum order quantity not met for %d item(s)", len(violations))).WithDetails(map[string]any{
		"violations": violations,
	})
}
