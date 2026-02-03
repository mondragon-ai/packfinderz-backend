package product

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
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

func TestRepositoryListProductSummaries(t *testing.T) {
	conn := openTestDB(t)
	tx := conn.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	t.Cleanup(func() {
		_ = tx.Rollback()
	})

	ctx := context.Background()
	repo := NewRepository(tx)
	user := mustCreateTestUser(t, tx)
	storeA := mustCreateTestStore(t, tx, user.ID)
	storeB := mustCreateTestStore(t, tx, user.ID)
	if err := tx.Model(&models.Store{}).Where("id = ?", storeB.ID).Update("subscription_active", false).Error; err != nil {
		t.Fatalf("update store subscription: %v", err)
	}

	active := mustInsertProduct(t, tx, storeA.ID, "ACTIVE", enums.ProductCategoryFlower, enums.ProductClassificationSativa, 1200, true, floatPtr(22), floatPtr(0.4))
	inactive := mustInsertProduct(t, tx, storeA.ID, "INACTIVE", enums.ProductCategoryFlower, enums.ProductClassificationIndica, 900, false, floatPtr(15), floatPtr(0.5))
	promo := mustInsertProduct(t, tx, storeA.ID, "PROMO", enums.ProductCategoryVape, enums.ProductClassificationHybrid, 2000, true, floatPtr(18), floatPtr(0.2))
	if err := tx.Create(&models.ProductVolumeDiscount{
		StoreID:         storeA.ID,
		ProductID:       promo.ID,
		MinQty:          5,
		DiscountPercent: 10,
	}).Error; err != nil {
		t.Fatalf("create discount: %v", err)
	}
	_ = mustInsertProduct(t, tx, storeB.ID, "OTHER", enums.ProductCategoryFlower, enums.ProductClassificationIndica, 1500, true, floatPtr(10), floatPtr(0.1))

	filters := ProductListFilters{
		Category:       strPtr(enums.ProductCategoryFlower),
		Classification: strPtr(enums.ProductClassificationSativa),
		PriceMaxCents:  intPtr(1500),
		THCMin:         floatPtr(20),
		HasPromo:       boolPtr(false),
	}

	firstPage, err := repo.ListProductSummaries(ctx, productListQuery{
		Pagination:     pagination.Params{Limit: 10},
		Filters:        filters,
		RequestedState: "OK",
	})
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if len(firstPage.Products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(firstPage.Products))
	}
	if firstPage.Products[0].ID != active.ID {
		t.Fatalf("expected active product, got %s", firstPage.Products[0].ID)
	}
	if firstPage.Products[0].HasPromo {
		t.Fatal("expected has_promo false")
	}

	withPromo, err := repo.ListProductSummaries(ctx, productListQuery{
		Pagination:     pagination.Params{Limit: 10},
		Filters:        ProductListFilters{HasPromo: boolPtr(true)},
		RequestedState: "OK",
	})
	if err != nil {
		t.Fatalf("list promo products: %v", err)
	}
	if len(withPromo.Products) != 1 || withPromo.Products[0].ID != promo.ID {
		t.Fatalf("expected promo product, got %v", withPromo.Products)
	}

	firstVendorPage, err := repo.ListProductSummaries(ctx, productListQuery{
		Pagination:    pagination.Params{Limit: 1},
		Filters:       ProductListFilters{},
		VendorStoreID: &storeA.ID,
	})
	if err != nil {
		t.Fatalf("list vendor page: %v", err)
	}
	if len(firstVendorPage.Products) != 1 || firstVendorPage.Products[0].ID != promo.ID {
		t.Fatalf("expected newest promo product first, got %v", firstVendorPage.Products)
	}
	if firstVendorPage.NextCursor == "" {
		t.Fatalf("expected next cursor for vendor pagination")
	}

	secondVendorPage, err := repo.ListProductSummaries(ctx, productListQuery{
		Pagination: pagination.Params{
			Limit:  1,
			Cursor: firstVendorPage.NextCursor,
		},
		Filters:       ProductListFilters{},
		VendorStoreID: &storeA.ID,
	})
	if err != nil {
		t.Fatalf("list vendor second page: %v", err)
	}
	if len(secondVendorPage.Products) != 1 || secondVendorPage.Products[0].ID != inactive.ID {
		t.Fatalf("expected second product, got %v", secondVendorPage.Products)
	}
}

func mustInsertProduct(t *testing.T, tx *gorm.DB, storeID uuid.UUID, sku string, category enums.ProductCategory, classification enums.ProductClassification, price int, active bool, thc, cbd *float64) *models.Product {
	t.Helper()
	product := &models.Product{
		StoreID:    storeID,
		SKU:        sku,
		Title:      sku,
		Category:   category,
		Feelings:   pq.StringArray{enums.ProductFeelingRelaxed.String()},
		Flavors:    pq.StringArray{enums.ProductFlavorEarthy.String()},
		Usage:      pq.StringArray{enums.ProductUsageStressRelief.String()},
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: price,
		IsActive:   active,
		Classification: func() *enums.ProductClassification {
			c := classification
			return &c
		}(),
		THCPercent: thc,
		CBDPercent: cbd,
	}
	if err := tx.Create(product).Error; err != nil {
		t.Fatalf("insert product: %v", err)
	}
	return product
}
func strPtr[T any](v T) *T { return &v }

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
