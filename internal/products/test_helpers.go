package product

import (
	"fmt"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

func mustCreateTestUser(t *testing.T, tx *gorm.DB) *models.User {
	t.Helper()
	user := &models.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("pf_test_%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		FirstName:    "Repo",
		LastName:     "Tester",
		IsActive:     true,
		StoreIDs:     []uuid.UUID{},
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func mustCreateTestStore(t *testing.T, tx *gorm.DB, ownerID uuid.UUID) *models.Store {
	t.Helper()
	store := &models.Store{
		ID:                   uuid.New(),
		Type:                 enums.StoreTypeVendor,
		CompanyName:          "Repo Store",
		OwnerID:              ownerID,
		KYCStatus:            enums.KYCStatusVerified,
		SubscriptionActive:   true,
		DeliveryRadiusMeters: 200,
		Address: types.Address{
			Line1:      "123 Repo Way",
			City:       "Tulsa",
			State:      "OK",
			PostalCode: "74104",
			Country:    "US",
			Lat:        36.153984,
			Lng:        -95.992775,
		},
		Geom: types.GeographyPoint{
			Lat: 36.153984,
			Lng: -95.992775,
		},
	}
	if err := tx.Create(store).Error; err != nil {
		t.Fatalf("create store: %v", err)
	}
	return store
}

func mustCreateTestProduct(t *testing.T, tx *gorm.DB, storeID uuid.UUID) *models.Product {
	t.Helper()
	product := &models.Product{
		StoreID:  storeID,
		SKU:      fmt.Sprintf("SKU-%s", uuid.NewString()),
		Title:    "Test Product",
		Category: enums.ProductCategoryFlower,
		Feelings: pq.StringArray{
			enums.ProductFeelingRelaxed.String(),
		},
		Flavors: pq.StringArray{
			enums.ProductFlavorEarthy.String(),
		},
		Usage: pq.StringArray{
			enums.ProductUsageStressRelief.String(),
		},
		Unit:                enums.ProductUnitUnit,
		MOQ:                 1,
		PriceCents:          1000,
		CompareAtPriceCents: func() *int { v := 1500; return &v }(),
		IsActive:            true,
		IsFeatured:          false,
	}
	if err := tx.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	return product
}
