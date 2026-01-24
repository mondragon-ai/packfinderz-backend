package types

import (
	"testing"

	"github.com/google/uuid"
)

func TestCartLevelDiscountsValueAndScan(t *testing.T) {
	t.Helper()

	discount := CartLevelDiscount{
		Type:      stringPtr("promotion"),
		Title:     "WELCOME10",
		ID:        uuid.New(),
		Value:     "10.00",
		ValueType: "percentage",
		VendorID:  uuid.New(),
	}

	payload := CartLevelDiscounts{discount}
	val, err := payload.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var decoded CartLevelDiscounts
	if err := decoded.Scan(val); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("expected 1 discount, got %d", len(decoded))
	}

	got := decoded[0]
	if got.Title != discount.Title {
		t.Fatalf("expected title %q, got %q", discount.Title, got.Title)
	}
	if got.Type == nil || discount.Type == nil || *got.Type != *discount.Type {
		t.Fatalf("type mismatch")
	}
	if got.ID != discount.ID {
		t.Fatalf("expected id %s, got %s", discount.ID, got.ID)
	}
	if got.Value != discount.Value {
		t.Fatalf("expected value %q, got %q", discount.Value, got.Value)
	}
	if got.ValueType != discount.ValueType {
		t.Fatalf("expected value type %q, got %q", discount.ValueType, got.ValueType)
	}
	if got.VendorID != discount.VendorID {
		t.Fatalf("expected vendor %s, got %s", discount.VendorID, got.VendorID)
	}
}

func TestCartLevelDiscountsScanNil(t *testing.T) {
	var discounts CartLevelDiscounts
	if err := discounts.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if discounts != nil {
		t.Fatalf("expected nil slice, got %#v", discounts)
	}
}

func stringPtr(value string) *string {
	return &value
}
