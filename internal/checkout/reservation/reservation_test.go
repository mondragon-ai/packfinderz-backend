package reservation

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReserveInventory(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	ctx := context.Background()
	productA := uuid.New()
	productB := uuid.New()

	for _, item := range []models.InventoryItem{
		{ProductID: productA, AvailableQty: 5},
		{ProductID: productB, AvailableQty: 1},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("seed inventory: %v", err)
		}
	}

	requests := []InventoryReservationRequest{
		{CartItemID: uuid.New(), ProductID: productA, Qty: 3},
		{CartItemID: uuid.New(), ProductID: productA, Qty: 4},
		{CartItemID: uuid.New(), ProductID: productB, Qty: 1},
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		results, terr := ReserveInventory(ctx, tx, requests)
		if terr != nil {
			return terr
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
		if !results[0].Reserved || results[0].Reason != "" {
			t.Fatalf("expected first reservation to succeed")
		}
		if results[1].Reserved || results[1].Reason == "" {
			t.Fatalf("expected second reservation to fail with reason")
		}
		if !results[2].Reserved {
			t.Fatalf("expected third reservation to succeed")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reserve transaction: %v", err)
	}

	var invA, invB models.InventoryItem
	if err := db.First(&invA, "product_id = ?", productA).Error; err != nil {
		t.Fatalf("load inventory a: %v", err)
	}
	if err := db.First(&invB, "product_id = ?", productB).Error; err != nil {
		t.Fatalf("load inventory b: %v", err)
	}
	if invA.AvailableQty != 2 || invA.ReservedQty != 3 {
		t.Fatalf("unexpected inventory a state: %+v", invA)
	}
	if invB.AvailableQty != 0 || invB.ReservedQty != 1 {
		t.Fatalf("unexpected inventory b state: %+v", invB)
	}
}

func TestReserveInventoryInvalidQty(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	ctx := context.Background()
	product := uuid.New()
	if err := db.Create(&models.InventoryItem{ProductID: product, AvailableQty: 5}).Error; err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	_, err := ReserveInventory(ctx, db, []InventoryReservationRequest{{ProductID: product, Qty: 0}})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:reservation_" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.InventoryItem{}); err != nil {
		t.Fatalf("migrate inventory: %v", err)
	}
	return db
}
