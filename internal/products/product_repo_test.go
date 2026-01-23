package product

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestRepositoryProductFlow(t *testing.T) {
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

	created, err := repo.CreateProduct(ctx, product)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected product id to be generated")
	}

	detail, summary, err := repo.GetProductDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if summary.StoreID != store.ID {
		t.Fatalf("expected vendor summary store %s, got %s", store.ID, summary.StoreID)
	}
	if detail.SKU != product.SKU {
		t.Fatalf("expected SKU %s, got %s", product.SKU, detail.SKU)
	}

	created.Title = "Updated Title"
	if _, err := repo.UpdateProduct(ctx, created); err != nil {
		t.Fatalf("update product: %v", err)
	}

	fetched, _, err := repo.GetProductDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("get detail after update: %v", err)
	}
	if fetched.Title != "Updated Title" {
		t.Fatalf("expected updated title, got %s", fetched.Title)
	}

	list, err := repo.ListProductsByStore(ctx, store.ID)
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected at least one product")
	}

	if err := repo.DeleteProduct(ctx, created.ID); err != nil {
		t.Fatalf("delete product: %v", err)
	}
}
