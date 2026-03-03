package ads

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const adsSchema = `
CREATE TABLE stores (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  subscription_active BOOLEAN NOT NULL,
  kyc_status TEXT NOT NULL
);
CREATE TABLE ads (
  id TEXT PRIMARY KEY,
  store_id TEXT NOT NULL,
  status TEXT NOT NULL,
  placement TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  bid_cents INTEGER NOT NULL,
  daily_budget_cents INTEGER NOT NULL,
  starts_at DATETIME,
  ends_at DATETIME,
  created_at DATETIME,
  updated_at DATETIME
);
CREATE TABLE ad_creatives (
  id TEXT PRIMARY KEY,
  ad_id TEXT NOT NULL,
  media_id TEXT,
  destination_url TEXT NOT NULL,
  headline TEXT,
  body TEXT,
  created_at DATETIME,
  updated_at DATETIME
);
`

func setupAdsRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(adsSchema).Error)
	return db
}

func TestRepository_CreateAdReturnsCreativesInOrder(t *testing.T) {
	db := setupAdsRepoTestDB(t)
	repo := NewRepository(db)

	now := time.Now().UTC()
	input := CreateAdInput{
		StoreID:          uuid.New(),
		Status:           enums.AdStatusActive,
		Placement:        enums.AdPlacementHero,
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         100,
		DailyBudgetCents: 1_000,
		StartsAt:         &now,
		EndsAt:           nil,
		Creatives: []AdCreativeInput{
			{DestinationURL: "https://a.example.com"},
			{DestinationURL: "https://b.example.com"},
		},
	}

	created, err := repo.CreateAd(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, created.Creatives, 2)

	for i := 1; i < len(created.Creatives); i++ {
		assert.Truef(
			t,
			created.Creatives[i-1].ID.String() < created.Creatives[i].ID.String(),
			"creatives should be sorted by id ascending",
		)
	}
}

func TestRepository_ListAdsPaginationAndFilters(t *testing.T) {
	db := setupAdsRepoTestDB(t)
	repo := NewRepository(db)

	storeID := uuid.New()
	now := time.Now().UTC()

	// Insert three hero-active ads for the store with descending created_at timestamps.
	for i := 0; i < 3; i++ {
		ad := models.Ad{
			StoreID:          storeID,
			Status:           enums.AdStatusActive,
			Placement:        string(enums.AdPlacementHero),
			TargetType:       enums.AdTargetTypeStore,
			TargetID:         uuid.New(),
			BidCents:         int64(100 + i),
			DailyBudgetCents: 1_000,
			CreatedAt:        now.Add(time.Duration(-i) * time.Minute),
			UpdatedAt:        now.Add(time.Duration(-i) * time.Minute),
		}
		require.NoError(t, db.Create(&ad).Error)
	}

	// Insert an ad for another store to ensure scoping.
	require.NoError(t, db.Create(&models.Ad{
		StoreID:          uuid.New(),
		Status:           enums.AdStatusActive,
		Placement:        string(enums.AdPlacementHero),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         42,
		DailyBudgetCents: 500,
		CreatedAt:        now.Add(-10 * time.Minute),
		UpdatedAt:        now.Add(-10 * time.Minute),
	}).Error)

	input := ListAdsInput{
		StoreID: storeID,
		Filters: ListAdsFilters{
			Status:    ptrAdStatus(enums.AdStatusActive),
			Placement: ptrAdPlacement(enums.AdPlacementHero),
		},
		Pagination: pagination.Params{
			Limit: 2,
		},
	}

	firstPage, err := repo.ListAds(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, firstPage.Ads, 2)
	assert.NotEmpty(t, firstPage.Pagination.Next)
	assert.Equal(t, 3, firstPage.Pagination.Total)

	secondInput := input
	secondInput.Pagination.Cursor = firstPage.Pagination.Next
	second, err := repo.ListAds(context.Background(), secondInput)
	require.NoError(t, err)
	require.Len(t, second.Ads, 1)
	assert.Empty(t, second.Pagination.Next)
	assert.Equal(t, firstPage.Pagination.Next, second.Pagination.Prev)
}

func TestRepository_GetAdByIDEnforcesStoreScope(t *testing.T) {
	db := setupAdsRepoTestDB(t)
	repo := NewRepository(db)

	storeID := uuid.New()
	ad := models.Ad{
		ID:               uuid.New(),
		StoreID:          storeID,
		Status:           enums.AdStatusActive,
		Placement:        string(enums.AdPlacementHero),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         10,
		DailyBudgetCents: 1_000,
	}
	require.NoError(t, db.Create(&ad).Error)

	ctx := context.Background()
	fetched, err := repo.GetAdByID(ctx, storeID, ad.ID)
	require.NoError(t, err)
	assert.Equal(t, ad.ID, fetched.ID)

	_, err = repo.GetAdByID(ctx, uuid.New(), ad.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRepository_ListEligibleAdsForServeFiltersByPlacementAndWindow(t *testing.T) {
	db := setupAdsRepoTestDB(t)
	repo := NewRepository(db)

	now := time.Now().UTC()
	storeID := uuid.New()
	insertStoreRecord(t, db, storeID)

	highBid := models.Ad{
		StoreID:          storeID,
		Status:           enums.AdStatusActive,
		Placement:        string(enums.AdPlacementHero),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         2000,
		DailyBudgetCents: 5_000,
	}
	lowBid := models.Ad{
		StoreID:          storeID,
		Status:           enums.AdStatusActive,
		Placement:        string(enums.AdPlacementHero),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         1000,
		DailyBudgetCents: 5_000,
		StartsAt:         ptrTime(now.Add(-time.Hour)),
		EndsAt:           ptrTime(now.Add(time.Hour)),
	}
	require.NoError(t, db.Create(&highBid).Error)
	require.NoError(t, db.Create(&lowBid).Error)

	// Non-eligible records.
	require.NoError(t, db.Create(&models.Ad{
		StoreID:          storeID,
		Status:           enums.AdStatusPaused,
		Placement:        string(enums.AdPlacementHero),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         500,
		DailyBudgetCents: 1_000,
	}).Error)
	require.NoError(t, db.Create(&models.Ad{
		StoreID:          storeID,
		Status:           enums.AdStatusActive,
		Placement:        string(enums.AdPlacementStore),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         3000,
		DailyBudgetCents: 1_000,
	}).Error)
	require.NoError(t, db.Create(&models.Ad{
		StoreID:          storeID,
		Status:           enums.AdStatusActive,
		Placement:        string(enums.AdPlacementHero),
		TargetType:       enums.AdTargetTypeStore,
		TargetID:         uuid.New(),
		BidCents:         4000,
		DailyBudgetCents: 1_000,
		EndsAt:           ptrTime(now.Add(-time.Minute)),
	}).Error)

	candidates, err := repo.ListEligibleAdsForServe(context.Background(), enums.AdPlacementHero, 1, now)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, highBid.ID, candidates[0].ID)
}

func ptrAdStatus(value enums.AdStatus) *enums.AdStatus {
	return &value
}

func ptrAdPlacement(value enums.AdPlacement) *enums.AdPlacement {
	return &value
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func insertStoreRecord(t *testing.T, db *gorm.DB, storeID uuid.UUID) {
	t.Helper()
	if err := db.Exec(
		"INSERT INTO stores (id, type, subscription_active, kyc_status) VALUES (?, ?, ?, ?)",
		storeID.String(),
		string(enums.StoreTypeVendor),
		true,
		string(enums.KYCStatusVerified),
	).Error; err != nil {
		t.Fatalf("insert store record: %v", err)
	}
}
