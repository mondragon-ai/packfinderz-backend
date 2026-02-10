package address

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/maps"
)

func TestMapPlaceDetails(t *testing.T) {
	details := &maps.PlaceDetails{
		FormattedAddress: "123 Demo St, Example City, OK 73106, US",
		Location: maps.LatLng{
			Latitude:  35.4676,
			Longitude: -97.5164,
		},
		AddressComponents: []maps.AddressComponent{
			{LongName: "123", Types: []string{"street_number"}},
			{LongName: "Demo St", Types: []string{"route"}},
			{LongName: "Suite 5", Types: []string{"subpremise"}},
			{LongName: "Example City", Types: []string{"locality"}},
			{LongName: "Oklahoma", Types: []string{"administrative_area_level_1"}},
			{LongName: "73106", Types: []string{"postal_code"}},
			{LongName: "United States", Types: []string{"country"}},
		},
	}

	result, err := mapPlaceDetails(details)
	if err != nil {
		t.Fatalf("mapPlaceDetails failed: %v", err)
	}
	if result.Line1 != "123 Demo St" {
		t.Fatalf("unexpected line1 %q", result.Line1)
	}
	if result.Line2 == nil || *result.Line2 != "Suite 5" {
		t.Fatalf("unexpected line2 %v", result.Line2)
	}
	if result.City != "Example City" {
		t.Fatalf("unexpected city %q", result.City)
	}
	if result.State != "Oklahoma" {
		t.Fatalf("unexpected state %q", result.State)
	}
	if result.PostalCode != "73106" {
		t.Fatalf("unexpected postal %q", result.PostalCode)
	}
	if result.Country != "United States" {
		t.Fatalf("unexpected country %q", result.Country)
	}
	if result.Lat != 35.4676 || result.Lng != -97.5164 {
		t.Fatalf("unexpected location %+v", result)
	}
}

func TestMapPlaceDetailsMissingCity(t *testing.T) {
	details := &maps.PlaceDetails{
		AddressComponents: []maps.AddressComponent{
			{LongName: "123", Types: []string{"street_number"}},
			{LongName: "Demo St", Types: []string{"route"}},
			{LongName: "Oklahoma", Types: []string{"administrative_area_level_1"}},
			{LongName: "73106", Types: []string{"postal_code"}},
			{LongName: "United States", Types: []string{"country"}},
		},
		Location: maps.LatLng{
			Latitude:  35.4676,
			Longitude: -97.5164,
		},
	}

	if _, err := mapPlaceDetails(details); err == nil {
		t.Fatal("expected error when city missing")
	}
}
