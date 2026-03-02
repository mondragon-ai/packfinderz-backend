package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

func TestBuildAdListFilters(t *testing.T) {
	targetID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/?status=active&placement=hero&target_type=store&target_id="+targetID.String(), nil)

	filters, err := buildAdListFilters(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if filters.Status == nil || *filters.Status != enums.AdStatusActive {
		t.Fatalf("unexpected status filter: %v", filters.Status)
	}
	if filters.Placement == nil || *filters.Placement != enums.AdPlacementHero {
		t.Fatalf("unexpected placement filter: %v", filters.Placement)
	}
	if filters.TargetType == nil || *filters.TargetType != enums.AdTargetTypeStore {
		t.Fatalf("unexpected target type: %v", filters.TargetType)
	}
	if filters.TargetID == nil || *filters.TargetID != targetID {
		t.Fatalf("unexpected target id: %v", filters.TargetID)
	}
}

func TestParseAdTimeframe(t *testing.T) {
	cases := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
		"90d": 90 * 24 * time.Hour,
		"1y":  365 * 24 * time.Hour,
	}

	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			got, ok := parseAdTimeframe(raw)
			if !ok {
				t.Fatalf("expected ok for %s", raw)
			}
			if got != want {
				t.Fatalf("expected %v, got %v", want, got)
			}
		})
	}

	if _, ok := parseAdTimeframe("bad"); ok {
		t.Fatalf("expected parse failure for invalid value")
	}
}

func TestResolveAdTimeframeDefault(t *testing.T) {
	now := time.Date(2026, time.January, 2, 15, 0, 0, 0, time.UTC)
	prev := adTimeNowUTC
	adTimeNowUTC = func() time.Time { return now }
	defer func() { adTimeNowUTC = prev }()

	req := httptest.NewRequest(http.MethodGet, "/?", nil)
	start, end, err := resolveAdTimeframe(req, adTimeNowUTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStart := now.Add(-30 * 24 * time.Hour)
	if !start.Equal(expectedStart) {
		t.Fatalf("expected start %v got %v", expectedStart, start)
	}
	if !end.Equal(now) {
		t.Fatalf("expected end %v got %v", now, end)
	}
}

func TestResolveAdTimeframeInvalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?timeframe=foo", nil)
	_, _, err := resolveAdTimeframe(req, time.Now().UTC())
	if err == nil {
		t.Fatalf("expected error for invalid timeframe")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("unexpected error code: %v", err)
	}
}
