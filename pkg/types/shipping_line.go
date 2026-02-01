package types

import (
	"database/sql/driver"
	"encoding/json"
)

// ShippingLine stores a Shopify-like shipping estimate for a vendor order.
type ShippingLine struct {
	Code       string `json:"code"`
	Title      string `json:"title"`
	PriceCents int    `json:"price_cents"`
}

// Value serializes the shipping line to JSON.
func (s *ShippingLine) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan decodes JSONB into the shipping line struct.
func (s *ShippingLine) Scan(value interface{}) error {
	if value == nil {
		*s = ShippingLine{}
		return nil
	}
	raw, err := asJSON(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, s)
}

// JSONMap stores an arbitrary JSON object inside a JSONB column.
type JSONMap map[string]any

// Value serializes the map to JSON.
func (j *JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan decodes JSONB into the map.
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	raw, err := asJSON(value)
	if err != nil {
		return err
	}
	var decoded JSONMap
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*j = decoded
	return nil
}
