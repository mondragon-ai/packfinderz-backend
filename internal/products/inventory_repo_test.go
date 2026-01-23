package product

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
)

func TestRepositoryInventoryOneToOne(t *testing.T) {
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

	inventory := &models.InventoryItem{
		ProductID:    product.ID,
		AvailableQty: 10,
		ReservedQty:  0,
	}

	if _, err := repo.UpsertInventory(ctx, inventory); err != nil {
		t.Fatalf("upsert inventory: %v", err)
	}

	if err := tx.Create(&models.InventoryItem{ProductID: product.ID, AvailableQty: 5, ReservedQty: 0}).Error; err == nil {
		t.Fatal("expected duplicate inventory insert to fail")
	}

	fetched, err := repo.GetInventoryByProductID(ctx, product.ID)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	if fetched.AvailableQty != 10 {
		t.Fatalf("expected available 10, got %d", fetched.AvailableQty)
	}
}
