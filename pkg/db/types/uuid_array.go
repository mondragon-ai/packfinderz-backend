package dbtypes

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type UUIDArray []uuid.UUID

func (a *UUIDArray) Scan(src any) error {
	if src == nil {
		*a = UUIDArray{}
		return nil
	}

	switch v := src.(type) {
	case string:
		return a.parseFromString(v)
	case []byte:
		return a.parseFromString(string(v))
	default:
		return fmt.Errorf("UUIDArray: unsupported Scan type %T", src)
	}
}

func (a UUIDArray) Value() (driver.Value, error) {
	// Postgres array literal: {uuid,uuid}
	if len(a) == 0 {
		return "{}", nil
	}
	parts := make([]string, 0, len(a))
	for _, id := range a {
		parts = append(parts, id.String())
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (a *UUIDArray) parseFromString(s string) error {
	s = strings.TrimSpace(s)
	if s == "{}" || s == "" {
		*a = UUIDArray{}
		return nil
	}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if strings.TrimSpace(s) == "" {
		*a = UUIDArray{}
		return nil
	}

	raw := strings.Split(s, ",")
	out := make([]uuid.UUID, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(strings.Trim(r, `"`))
		id, err := uuid.Parse(r)
		if err != nil {
			return fmt.Errorf("UUIDArray: parse %q: %w", r, err)
		}
		out = append(out, id)
	}
	*a = UUIDArray(out)
	return nil
}
