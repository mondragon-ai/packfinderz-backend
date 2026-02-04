package types

import (
	"encoding/json"
	"testing"
)

func TestNullableUUIDUnmarshal(t *testing.T) {
	type payload struct {
		ID NullableUUID `json:"id"`
	}

	var got payload
	if err := json.Unmarshal([]byte(`{"id": "00000000-0000-0000-0000-000000000001"}`), &got); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if !got.ID.Valid || got.ID.Value == nil {
		t.Fatalf("expected valid uuid, got %v", got.ID)
	}
	if got.ID.Value.String() != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected uuid %s", got.ID.Value)
	}

	got = payload{}
	if err := json.Unmarshal([]byte(`{"id": null}`), &got); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !got.ID.Valid || got.ID.Value != nil {
		t.Fatalf("expected null to be valid but nil, got %v", got.ID)
	}

	got = payload{}
	if err := json.Unmarshal([]byte(`{}`), &got); err != nil {
		t.Fatalf("unmarshal missing: %v", err)
	}
	if got.ID.Valid {
		t.Fatalf("expected invalid flag for missing field, got %+v", got.ID)
	}
}
