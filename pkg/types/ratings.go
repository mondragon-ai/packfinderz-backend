package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Ratings represents a flexible scoring map persisted as JSONB.
type Ratings map[string]int

// Value marshals the map into JSON for Postgres.
func (r Ratings) Value() (driver.Value, error) {
	if r == nil {
		return "{}", nil
	}
	buf, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// Scan decodes JSONB into the map.
func (r *Ratings) Scan(value interface{}) error {
	if value == nil {
		*r = nil
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("ratings: unsupported scan type %T", value)
	}

	result := make(Ratings)
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	*r = result
	return nil
}
