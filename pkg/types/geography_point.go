package types

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// GeographyPoint represents a PostGIS Point expressed in geography format.
type GeographyPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Value produces an EWKT literal so Postgres can cast the geography.
func (g GeographyPoint) Value() (driver.Value, error) {
	return fmt.Sprintf("SRID=4326;POINT(%f %f)", g.Lng, g.Lat), nil
}

// Scan accepts WKT/EWKT or WKB bytes returned by Postgres.
func (g *GeographyPoint) Scan(value interface{}) error {
	if value == nil {
		*g = GeographyPoint{}
		return nil
	}

	switch v := value.(type) {
	case string:
		return g.fromText(v)
	case []byte:
		text := strings.TrimSpace(string(v))
		upper := strings.ToUpper(text)
		if strings.HasPrefix(upper, "SRID=") || strings.HasPrefix(upper, "POINT(") {
			return g.fromText(text)
		}
		return g.fromWKB(v)
	default:
		if stringer, ok := value.(fmt.Stringer); ok {
			return g.fromText(stringer.String())
		}
		return fmt.Errorf("geography: unsupported scan type %T", value)
	}
}

func (g *GeographyPoint) fromText(raw string) error {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToUpper(raw), "SRID=") {
		if idx := strings.Index(raw, ";"); idx != -1 {
			raw = raw[idx+1:]
		}
	}

	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToUpper(raw), "POINT(") || !strings.HasSuffix(raw, ")") {
		return fmt.Errorf("geography: unsupported text %q", raw)
	}

	content := strings.TrimSpace(raw[len("POINT(") : len(raw)-1])
	segments := strings.Fields(content)
	if len(segments) != 2 {
		return fmt.Errorf("geography: unexpected POINT content %q", content)
	}

	lng, err := strconvParseFloat(segments[0])
	if err != nil {
		return err
	}
	lat, err := strconvParseFloat(segments[1])
	if err != nil {
		return err
	}

	g.Lng = lng
	g.Lat = lat
	return nil
}

func (g *GeographyPoint) fromWKB(raw []byte) error {
	if len(raw) < 21 {
		return fmt.Errorf("geography: wkb too short")
	}

	var order binary.ByteOrder
	switch raw[0] {
	case 0:
		order = binary.BigEndian
	case 1:
		order = binary.LittleEndian
	default:
		return fmt.Errorf("geography: invalid byte order %d", raw[0])
	}

	geomType := order.Uint32(raw[1:5])
	if geomType != 1 {
		return fmt.Errorf("geography: unexpected geometry type %d", geomType)
	}

	g.Lng = math.Float64frombits(order.Uint64(raw[5:13]))
	g.Lat = math.Float64frombits(order.Uint64(raw[13:21]))
	return nil
}

func strconvParseFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("geography: empty coordinate")
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("geography: parse coordinate %w", err)
	}
	return f, nil
}
