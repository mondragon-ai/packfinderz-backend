package types

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CartLevelDiscount mirrors the cart_level_discount composite type.
type CartLevelDiscount struct {
	Type      *string   `json:"type,omitempty"`
	Title     string    `json:"title"`
	ID        uuid.UUID `json:"id"`
	Value     string    `json:"value"`
	ValueType string    `json:"value_type"`
	VendorID  uuid.UUID `json:"vendor_id"`
}

// CartLevelDiscounts represents a postgres array of cart_level_discount.
type CartLevelDiscounts []CartLevelDiscount

// Value implements the driver.Valuer interface so the slice can be inserted.
func (c CartLevelDiscounts) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	if len(c) == 0 {
		return "{}", nil
	}
	values := make([]string, 0, len(c))
	for _, entry := range c {
		composite, err := entry.toComposite()
		if err != nil {
			return nil, err
		}
		values = append(values, composite)
	}
	return pq.Array(values).Value()
}

// Scan implements sql.Scanner for the Postgres cart_level_discount[] column.
func (c *CartLevelDiscounts) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}

	var raw pq.StringArray
	if err := raw.Scan(value); err != nil {
		return err
	}

	if len(raw) == 0 {
		*c = CartLevelDiscounts{}
		return nil
	}

	result := make(CartLevelDiscounts, 0, len(raw))
	for _, entry := range raw {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		discount, err := parseCartLevelDiscount(entry)
		if err != nil {
			return err
		}
		result = append(result, discount)
	}

	*c = result
	return nil
}

func (d CartLevelDiscount) toComposite() (string, error) {
	if strings.TrimSpace(d.Title) == "" {
		return "", fmt.Errorf("cart level discount: missing title")
	}
	if d.ID == uuid.Nil {
		return "", fmt.Errorf("cart level discount: missing id")
	}
	if strings.TrimSpace(d.Value) == "" {
		return "", fmt.Errorf("cart level discount: missing value")
	}
	if strings.TrimSpace(d.ValueType) == "" {
		return "", fmt.Errorf("cart level discount: missing value type")
	}
	if d.VendorID == uuid.Nil {
		return "", fmt.Errorf("cart level discount: missing vendor id")
	}

	parts := []string{
		quoteCompositeNullable(d.Type),
		quoteCompositeString(d.Title),
		quoteCompositeString(d.ID.String()),
		quoteCompositeString(d.Value),
		quoteCompositeString(d.ValueType),
		quoteCompositeString(d.VendorID.String()),
	}
	return "(" + strings.Join(parts, ",") + ")", nil
}

func parseCartLevelDiscount(raw string) (CartLevelDiscount, error) {
	fields, err := parseComposite(raw, 6)
	if err != nil {
		return CartLevelDiscount{}, err
	}

	var discount CartLevelDiscount
	if !isCompositeNull(fields[0]) {
		value := fields[0]
		discount.Type = &value
	}

	if strings.TrimSpace(fields[1]) == "" {
		return CartLevelDiscount{}, fmt.Errorf("cart level discount: empty title")
	}
	discount.Title = fields[1]

	id, err := uuid.Parse(fields[2])
	if err != nil {
		return CartLevelDiscount{}, fmt.Errorf("cart level discount: parse id %w", err)
	}
	discount.ID = id

	if strings.TrimSpace(fields[3]) == "" {
		return CartLevelDiscount{}, fmt.Errorf("cart level discount: empty value")
	}
	if strings.TrimSpace(fields[4]) == "" {
		return CartLevelDiscount{}, fmt.Errorf("cart level discount: empty value type")
	}
	discount.Value = fields[3]
	discount.ValueType = fields[4]

	vendorID, err := uuid.Parse(fields[5])
	if err != nil {
		return CartLevelDiscount{}, fmt.Errorf("cart level discount: parse vendor id %w", err)
	}
	discount.VendorID = vendorID

	return discount, nil
}
