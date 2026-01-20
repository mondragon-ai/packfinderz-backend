package types

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Address mirrors the address_t composite Postgres type.
type Address struct {
	Line1      string  `json:"line1"`
	Line2      *string `json:"line2,omitempty"`
	City       string  `json:"city"`
	State      string  `json:"state"`
	PostalCode string  `json:"postal_code"`
	Country    string  `json:"country"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	GeoHash    *string `json:"geohash,omitempty"`
}

// Value marshals Address into a Postgres composite literal.
func (a Address) Value() (driver.Value, error) {
	if strings.TrimSpace(a.Line1) == "" {
		return nil, fmt.Errorf("address: missing line1")
	}
	if strings.TrimSpace(a.City) == "" {
		return nil, fmt.Errorf("address: missing city")
	}
	if strings.TrimSpace(a.State) == "" {
		return nil, fmt.Errorf("address: missing state")
	}
	if strings.TrimSpace(a.PostalCode) == "" {
		return nil, fmt.Errorf("address: missing postal_code")
	}

	country := strings.TrimSpace(a.Country)
	if country == "" {
		country = "US"
	}

	parts := []string{
		quoteCompositeString(a.Line1),
		quoteCompositeNullable(a.Line2),
		quoteCompositeString(a.City),
		quoteCompositeString(a.State),
		quoteCompositeString(a.PostalCode),
		quoteCompositeString(country),
		strconv.FormatFloat(a.Lat, 'f', -1, 64),
		strconv.FormatFloat(a.Lng, 'f', -1, 64),
		quoteCompositeNullable(a.GeoHash),
	}

	return "(" + strings.Join(parts, ",") + ")", nil
}

// Scan decodes the Postgres composite literal.
func (a *Address) Scan(value interface{}) error {
	if value == nil {
		*a = Address{}
		return nil
	}

	raw, ok := toString(value)
	if !ok {
		return fmt.Errorf("address: unsupported scan type %T", value)
	}

	fields, err := parseComposite(raw, 9)
	if err != nil {
		return err
	}

	a.Line1 = fields[0]
	a.Line2 = newCompositeNullable(fields[1])
	a.City = fields[2]
	a.State = fields[3]
	a.PostalCode = fields[4]

	country := strings.TrimSpace(fields[5])
	if country == "" || isCompositeNull(fields[5]) {
		country = "US"
	}
	a.Country = country

	if fields[6] == "" || isCompositeNull(fields[6]) {
		return fmt.Errorf("address: lat missing")
	}
	lat, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return fmt.Errorf("address: parse lat %w", err)
	}
	a.Lat = lat

	if fields[7] == "" || isCompositeNull(fields[7]) {
		return fmt.Errorf("address: lng missing")
	}
	lng, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return fmt.Errorf("address: parse lng %w", err)
	}
	a.Lng = lng

	a.GeoHash = newCompositeNullable(fields[8])

	return nil
}

func toString(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	case fmt.Stringer:
		return v.String(), true
	default:
		return "", false
	}
}
