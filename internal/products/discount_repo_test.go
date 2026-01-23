package product

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
)

func TestRepositoryVolumeDiscounts(t *testing.T) {
	conn := openTestDB(t)
	tx := conn.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	t.Cleanup(func() {
		_ = tx.Rollback()
	})

	repo := NewRepository(tx)
	ctx := context.Background()

	user := mustCreateTestUser(t, tx)
	store := mustCreateTestStore(t, tx, user.ID)
	product := mustCreateTestProduct(t, tx, store.ID)

	discount := &models.ProductVolumeDiscount{
		ProductID:      product.ID,
		MinQty:         5,
		UnitPriceCents: 900,
	}
	if _, err := repo.CreateVolumeDiscount(ctx, discount); err != nil {
		t.Fatalf("create volume discount: %v", err)
	}

	duplicate := &models.ProductVolumeDiscount{
		ProductID:      product.ID,
		MinQty:         5,
		UnitPriceCents: 800,
	}
	if _, err := repo.CreateVolumeDiscount(ctx, duplicate); err == nil {
		t.Fatal("expected duplicate discount to fail")
	}

	higher := &models.ProductVolumeDiscount{
		ProductID:      product.ID,
		MinQty:         10,
		UnitPriceCents: 850,
	}
	if _, err := repo.CreateVolumeDiscount(ctx, higher); err != nil {
		t.Fatalf("create higher tier: %v", err)
	}

	list, err := repo.ListVolumeDiscounts(ctx, product.ID)
	if err != nil {
		t.Fatalf("list discounts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 discounts, got %d", len(list))
	}
	if list[0].MinQty <= list[1].MinQty {
		t.Fatalf("expected discounts ordered by min_qty DESC")
	}
}
