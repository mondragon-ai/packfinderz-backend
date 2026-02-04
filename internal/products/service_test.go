package product

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestEnsureUniqueDiscounts(t *testing.T) {
	t.Run("uniqueMinQty", func(t *testing.T) {
		err := ensureUniqueDiscounts([]VolumeDiscountInput{
			{MinQty: 1, DiscountPercent: 10},
			{MinQty: 2, DiscountPercent: 5},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("duplicateMinQty", func(t *testing.T) {
		err := ensureUniqueDiscounts([]VolumeDiscountInput{
			{MinQty: 1, DiscountPercent: 10},
			{MinQty: 1, DiscountPercent: 20},
		})
		if err == nil {
			t.Fatal("expected validation error for duplicate min_qty")
		}
		if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
			t.Fatalf("expected validation error code, got %v", err)
		}
	})
}

func TestApplyUpdateToProductTrimsAndCopies(t *testing.T) {
	product := &models.Product{
		SKU:   "old-sku",
		Title: "old title",
	}

	feelings := []string{"relaxed", "focused"}
	flavors := []string{"citrus"}
	usage := []string{"stress_relief"}

	input := UpdateProductInput{
		SKU:      stringPtr("  new-sku  "),
		Title:    stringPtr("  New Title "),
		Feelings: &feelings,
		Flavors:  &flavors,
		Usage:    &usage,
	}

	applyUpdateToProduct(product, input)

	if product.SKU != "new-sku" {
		t.Fatalf("expected trimmed sku, got %s", product.SKU)
	}
	if product.Title != "New Title" {
		t.Fatalf("expected trimmed title, got %s", product.Title)
	}
	if len(product.Feelings) != len(feelings) {
		t.Fatalf("expected %d feelings, got %d", len(feelings), len(product.Feelings))
	}
	for i, val := range product.Feelings {
		if val != feelings[i] {
			t.Fatalf("expected feeling %q at %d, got %q", feelings[i], i, val)
		}
	}
	if len(product.Flavors) != len(flavors) || product.Flavors[0] != flavors[0] {
		t.Fatalf("expected flavors %v, got %v", flavors, product.Flavors)
	}
	if len(product.Usage) != len(usage) || product.Usage[0] != usage[0] {
		t.Fatalf("expected usage %v, got %v", usage, product.Usage)
	}
}

func TestBuildProductMediaRows(t *testing.T) {
	storeID := uuid.New()
	productID := uuid.New()
	mediaProductID := uuid.New()
	storeMismatchID := uuid.New()
	adsMediaID := uuid.New()

	repo := &fakeMediaReader{
		rows: map[uuid.UUID]*models.Media{
			mediaProductID: {
				ID:      mediaProductID,
				StoreID: storeID,
				Kind:    enums.MediaKindProduct,
				GCSKey:  "gcs-key-product",
			},
			storeMismatchID: {
				ID:      storeMismatchID,
				StoreID: uuid.New(),
				Kind:    enums.MediaKindProduct,
				GCSKey:  "gcs-key-other-store",
			},
			adsMediaID: {
				ID:      adsMediaID,
				StoreID: storeID,
				Kind:    enums.MediaKindAds,
				GCSKey:  "gcs-key-ads",
			},
		},
	}
	svc := &service{
		mediaRepo: repo,
	}

	t.Run("success", func(t *testing.T) {
		rows, err := svc.buildProductMediaRows(context.Background(), storeID, productID, []uuid.UUID{mediaProductID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 media row, got %d", len(rows))
		}
		if rows[0].Position != 0 {
			t.Fatalf("expected position 0, got %d", rows[0].Position)
		}
		if rows[0].GCSKey != "gcs-key-product" {
			t.Fatalf("expected gcs key, got %s", rows[0].GCSKey)
		}
		if rows[0].MediaID == nil || *rows[0].MediaID != mediaProductID {
			t.Fatalf("expected media id %s, got %v", mediaProductID, rows[0].MediaID)
		}
	})

	t.Run("duplicate ids", func(t *testing.T) {
		_, err := svc.buildProductMediaRows(context.Background(), storeID, productID, []uuid.UUID{mediaProductID, mediaProductID})
		if err == nil {
			t.Fatal("expected error for duplicate media ids")
		}
		if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("store mismatch", func(t *testing.T) {
		_, err := svc.buildProductMediaRows(context.Background(), storeID, productID, []uuid.UUID{storeMismatchID})
		if err == nil {
			t.Fatal("expected error for media from other store")
		}
		if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("kind mismatch", func(t *testing.T) {
		_, err := svc.buildProductMediaRows(context.Background(), storeID, productID, []uuid.UUID{adsMediaID})
		if err == nil {
			t.Fatal("expected error for wrong media kind")
		}
		if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
}

func stringPtr(value string) *string {
	return &value
}

func TestValidateMaxQty(t *testing.T) {
	if err := validateMaxQty(-1); err == nil {
		t.Fatal("expected validation error for negative max_qty")
	}
	if err := validateMaxQty(0); err != nil {
		t.Fatalf("expected no error for zero max_qty, got %v", err)
	}
}

func TestValidateLowStockThreshold(t *testing.T) {
	if err := validateLowStockThreshold(-5); err == nil {
		t.Fatal("expected validation error for negative low_stock_threshold")
	}
	if err := validateLowStockThreshold(0); err != nil {
		t.Fatalf("expected no error for zero threshold, got %v", err)
	}
}

func TestValidateDiscountPercent(t *testing.T) {
	if err := validateDiscountPercent(-1); err == nil {
		t.Fatal("expected validation error for negative discount_percent")
	}
	if err := validateDiscountPercent(101); err == nil {
		t.Fatal("expected validation error for discount_percent > 100")
	}
	if err := validateDiscountPercent(25); err != nil {
		t.Fatalf("expected no error for valid discount_percent, got %v", err)
	}
}

type fakeMediaReader struct {
	rows map[uuid.UUID]*models.Media
}

func (f *fakeMediaReader) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	if row, ok := f.rows[id]; ok {
		return row, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func TestFetchStoreScopedMedia(t *testing.T) {
	storeID := uuid.New()
	productMediaID := uuid.New()
	otherStoreID := uuid.New()
	coaMediaID := uuid.New()

	repo := &fakeMediaReader{
		rows: map[uuid.UUID]*models.Media{
			productMediaID: {
				ID:      productMediaID,
				StoreID: storeID,
				Kind:    enums.MediaKindProduct,
			},
			otherStoreID: {
				ID:      otherStoreID,
				StoreID: uuid.New(),
				Kind:    enums.MediaKindProduct,
			},
			coaMediaID: {
				ID:      coaMediaID,
				StoreID: storeID,
				Kind:    enums.MediaKindCOA,
			},
		},
	}
	svc := &service{
		mediaRepo: repo,
	}

	t.Run("success", func(t *testing.T) {
		row, err := svc.fetchStoreScopedMedia(context.Background(), storeID, productMediaID, enums.MediaKindProduct)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if row.ID != productMediaID {
			t.Fatalf("expected media id %s, got %s", productMediaID, row.ID)
		}
	})

	t.Run("store mismatch", func(t *testing.T) {
		if _, err := svc.fetchStoreScopedMedia(context.Background(), storeID, otherStoreID, enums.MediaKindProduct); err == nil {
			t.Fatal("expected error for cross-store media")
		} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("kind mismatch", func(t *testing.T) {
		if _, err := svc.fetchStoreScopedMedia(context.Background(), storeID, coaMediaID, enums.MediaKindProduct); err == nil {
			t.Fatal("expected error for wrong media kind")
		} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
}
