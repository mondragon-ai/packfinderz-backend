package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// CartItemWarning captures a warning attached to a persisted cart line.
type CartItemWarning struct {
	Type    enums.CartItemWarningType `json:"type"`
	Message string                    `json:"message"`
}

// CartItemWarnings is a slice marshaled as JSONB.
type CartItemWarnings []CartItemWarning

// Value serializes the warnings to JSON.
func (c CartItemWarnings) Value() (driver.Value, error) {
	if c == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(c)
}

// Scan decodes JSONB into the warning slice.
func (c *CartItemWarnings) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}
	raw, err := asJSON(value)
	if err != nil {
		return err
	}
	var decoded CartItemWarnings
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*c = decoded
	return nil
}

// VendorGroupWarning captures a warning associated with a vendor group.
type VendorGroupWarning struct {
	Type    enums.VendorGroupWarningType `json:"type"`
	Message string                       `json:"message"`
}

// VendorGroupWarnings persist vendor-level warnings as JSONB.
type VendorGroupWarnings []VendorGroupWarning

// Value serializes the vendor group warnings to JSON.
func (v VendorGroupWarnings) Value() (driver.Value, error) {
	if v == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(v)
}

// Scan decodes JSONB into the vendor group warnings slice.
func (v *VendorGroupWarnings) Scan(value interface{}) error {
	if value == nil {
		*v = nil
		return nil
	}
	raw, err := asJSON(value)
	if err != nil {
		return err
	}
	var decoded VendorGroupWarnings
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*v = decoded
	return nil
}

// AppliedVolumeDiscount represents the volume discount saved on a cart item.
type AppliedVolumeDiscount struct {
	Label       string `json:"label"`
	AmountCents int    `json:"amount_cents"`
}

// Value serializes the discount object to JSON.
func (a *AppliedVolumeDiscount) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// Scan decodes a JSON object into the discount struct.
func (a *AppliedVolumeDiscount) Scan(value interface{}) error {
	if value == nil {
		*a = AppliedVolumeDiscount{}
		return nil
	}
	raw, err := asJSON(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, a)
}

// VendorGroupPromo captures the promo data persisted per vendor group.
type VendorGroupPromo struct {
	Code        string `json:"code"`
	AmountCents int    `json:"amount_cents"`
}

// Value serializes the promo object to JSON.
func (v *VendorGroupPromo) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// Scan decodes JSONB into the promo struct.
func (v *VendorGroupPromo) Scan(value interface{}) error {
	if value == nil {
		*v = VendorGroupPromo{}
		return nil
	}
	raw, err := asJSON(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func asJSON(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported scan type %T", value)
	}
}
