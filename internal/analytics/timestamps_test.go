package analytics

import (
	"testing"
	"time"
)

func TestRevenueTimestampPriority(t *testing.T) {
	now := time.Date(2025, 2, 1, 9, 0, 0, 0, time.UTC)
	paid := now.Add(2 * time.Hour)
	cash := now.Add(1 * time.Hour)
	fallback := now.Add(-1 * time.Hour)

	got := RevenueTimestamp(&paid, &cash, fallback)
	if !got.Equal(paid.UTC()) {
		t.Fatalf("expected paid timestamp, got %v", got)
	}

	got = RevenueTimestamp(nil, &cash, fallback)
	if !got.Equal(cash.UTC()) {
		t.Fatalf("expected cash timestamp, got %v", got)
	}

	got = RevenueTimestamp(nil, nil, fallback)
	if !got.Equal(fallback.UTC()) {
		t.Fatalf("expected fallback timestamp, got %v", got)
	}
}
