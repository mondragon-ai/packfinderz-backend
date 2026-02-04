package types

import (
	"bytes"
	"encoding/json"

	"github.com/google/uuid"
)

// NullableUUID tracks whether a UUID field was explicitly present in JSON.
type NullableUUID struct {
	Valid bool
	Value *uuid.UUID
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullableUUID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}

	if bytes.Equal(trimmed, []byte("null")) {
		n.Valid = true
		n.Value = nil
		return nil
	}

	var parsed uuid.UUID
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return err
	}
	n.Valid = true
	n.Value = &parsed
	return nil
}

// Clone returns a copy of the NullableUUID.
func (n NullableUUID) Clone() NullableUUID {
	if n.Value == nil {
		return NullableUUID{Valid: n.Valid}
	}
	copy := *n.Value
	return NullableUUID{Valid: n.Valid, Value: &copy}
}
