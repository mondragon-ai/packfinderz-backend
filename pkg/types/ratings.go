package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type Ratings map[string]int

func (r Ratings) Value() (driver.Value, error) {
	if r == nil {
		return "{}", nil
	}

	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (r *Ratings) Scan(value interface{}) error {
	if value == nil {
		*r = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*r = Ratings{}
			return nil
		}
		return json.Unmarshal(v, r)
	case string:
		if v == "" {
			*r = Ratings{}
			return nil
		}
		return json.Unmarshal([]byte(v), r)
	default:
		return fmt.Errorf("ratings: unsupported scan type %T", value)
	}
}
