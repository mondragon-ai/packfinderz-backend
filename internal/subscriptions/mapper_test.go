package subscriptions

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

func TestMapSquareStatus_KnownValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  enums.SubscriptionStatus
	}{
		{name: "pending maps to trialing", value: "pending", want: enums.SubscriptionStatusTrialing},
		{name: "past due with hyphen", value: "PAST-DUE", want: enums.SubscriptionStatusPastDue},
		{name: "completed", value: "completed", want: enums.SubscriptionStatusCanceled},
		{name: "suspended", value: "suspended", want: enums.SubscriptionStatusPaused},
		{name: "active", value: "ACTIVE", want: enums.SubscriptionStatusActive},
		{name: "trialing", value: "TRIALING", want: enums.SubscriptionStatusTrialing},
		{name: "incomplete expired", value: "INCOMPLETE_EXPIRED", want: enums.SubscriptionStatusIncompleteExpired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mapSquareStatus(tc.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestMapSquareStatus_UnknownValueDefaultsToActive(t *testing.T) {
	got, err := mapSquareStatus("brand_new_status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != enums.SubscriptionStatusActive {
		t.Fatalf("expected default active, got %s", got)
	}
}
