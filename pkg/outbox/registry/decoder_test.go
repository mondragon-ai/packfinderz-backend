package registry

import (
	"encoding/json"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

func TestDecoderRegistry(t *testing.T) {
	reg := NewDecoderRegistry()
	reg.Register(enums.EventLicenseStatusChanged, 1, func(payload json.RawMessage) (interface{}, error) {
		var decoded map[string]string
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	})

	input := json.RawMessage(`{"status":"verified"}`)
	output, err := reg.Decode(enums.EventLicenseStatusChanged, 1, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outMap, ok := output.(map[string]string); !ok || outMap["status"] != "verified" {
		t.Fatalf("unexpected output %+v", output)
	}
}
