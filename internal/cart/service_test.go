package cart

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
)

func TestSelectVolumeDiscount(t *testing.T) {
	t.Parallel()

	tiers := []models.ProductVolumeDiscount{
		{MinQty: 10, UnitPriceCents: 800},
		{MinQty: 5, UnitPriceCents: 900},
		{MinQty: 20, UnitPriceCents: 700},
	}

	if res := selectVolumeDiscount(12, tiers); res == nil || res.MinQty != 10 {
		t.Fatalf("expected tier with min qty 10, got %+v", res)
	}

	if res := selectVolumeDiscount(4, tiers); res != nil {
		t.Fatalf("expected no tier for qty 4, got %+v", res)
	}

	if res := selectVolumeDiscount(25, tiers); res == nil || res.MinQty != 20 {
		t.Fatalf("expected highest tier for qty 25, got %+v", res)
	}
}
