package types

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// Social mirrors the social_t composite Postgres type.
type Social struct {
	Twitter   *string `json:"twitter,omitempty"`
	Facebook  *string `json:"facebook,omitempty"`
	Instagram *string `json:"instagram,omitempty"`
	LinkedIn  *string `json:"linkedin,omitempty"`
	YouTube   *string `json:"youtube,omitempty"`
	Website   *string `json:"website,omitempty"`
}

func (s Social) Value() (driver.Value, error) {
	parts := []string{
		quoteCompositeNullable(s.Twitter),
		quoteCompositeNullable(s.Facebook),
		quoteCompositeNullable(s.Instagram),
		quoteCompositeNullable(s.LinkedIn),
		quoteCompositeNullable(s.YouTube),
		quoteCompositeNullable(s.Website),
	}
	return "(" + strings.Join(parts, ",") + ")", nil
}

func (s *Social) Scan(value interface{}) error {
	if value == nil {
		*s = Social{}
		return nil
	}

	raw, ok := toString(value)
	if !ok {
		return fmt.Errorf("social: unsupported scan type %T", value)
	}

	fields, err := parseComposite(raw, 6)
	if err != nil {
		return err
	}

	s.Twitter = newCompositeNullable(fields[0])
	s.Facebook = newCompositeNullable(fields[1])
	s.Instagram = newCompositeNullable(fields[2])
	s.LinkedIn = newCompositeNullable(fields[3])
	s.YouTube = newCompositeNullable(fields[4])
	s.Website = newCompositeNullable(fields[5])

	return nil
}
