package ads

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/ads/token"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServeAdSelectsTieBreakWinner(t *testing.T) {
	db := setupAdsTestDB(t)
	repo := NewRepository(db)

	storeID := uuid.New()
	insertStore(t, db, storeID)

	ad1ID := uuid.New()
	ad2ID := uuid.New()
	insertAd(t, db, ad1ID, storeID, enums.AdPlacementHero, 100, 1_000)
	insertAd(t, db, ad2ID, storeID, enums.AdPlacementHero, 100, 1_000)
	insertCreative(t, db, uuid.New(), ad1ID, "https://a")
	insertCreative(t, db, uuid.New(), ad2ID, "https://b")

	redis := newFakeRedis()
	svc := newTestService(repo, redis)
	now := time.Date(2026, time.January, 6, 9, 0, 0, 0, time.UTC)
	requestID := "req-" + uuid.NewString()
	result, err := svc.ServeAd(context.Background(), ServeAdInput{
		Placement:    enums.AdPlacementHero,
		BuyerStoreID: uuid.New(),
		RequestID:    requestID,
		Now:          now,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	day := formatDay(now)
	score1 := tieBreakScore(requestID, enums.AdPlacementHero, day, ad1ID)
	score2 := tieBreakScore(requestID, enums.AdPlacementHero, day, ad2ID)
	var expected uuid.UUID
	if score1 < score2 {
		expected = ad1ID
	} else {
		expected = ad2ID
	}
	require.Equal(t, expected, result.AdID)
}

func TestServeAdReturnsRedisUnavailable(t *testing.T) {
	db := setupAdsTestDB(t)
	repo := NewRepository(db)

	storeID := uuid.New()
	insertStore(t, db, storeID)
	adID := uuid.New()
	insertAd(t, db, adID, storeID, enums.AdPlacementHero, 100, 1_000)
	insertCreative(t, db, uuid.New(), adID, "https://example")

	failure := newFakeRedis()
	failure.getErr = errors.New("boom")
	svc := newTestService(repo, failure)
	_, err := svc.ServeAd(context.Background(), ServeAdInput{
		Placement:    enums.AdPlacementHero,
		BuyerStoreID: uuid.New(),
		RequestID:    uuid.NewString(),
		Now:          time.Now().UTC(),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrRedisUnavailable))
}

func TestTrackImpressionRespectsDedupe(t *testing.T) {
	redis := newFakeRedis()
	svc := newTestService(nil, redis)

	now := time.Now().UTC()
	payload := token.Payload{
		TokenID:        uuid.New(),
		AdID:           uuid.New(),
		CreativeID:     uuid.New(),
		Placement:      enums.AdPlacementHero,
		TargetType:     enums.AdTargetTypeStore,
		TargetID:       uuid.New(),
		BuyerStoreID:   uuid.New(),
		EventType:      enums.AdEventFactTypeImpression,
		OccurredAt:     now,
		ExpiresAt:      now.Add(24 * time.Hour),
		RequestID:      "req-imp",
		BidCents:       200,
		DestinationURL: "https://example",
	}
	viewToken, err := token.MintToken("secret", payload)
	require.NoError(t, err)
	_, err = token.ParseToken("secret", viewToken)
	require.NoError(t, err)

	input := TrackImpressionInput{
		Token:        viewToken,
		RequestID:    payload.RequestID,
		BuyerStoreID: payload.BuyerStoreID,
		Now:          now,
	}
	require.NoError(t, svc.TrackImpression(context.Background(), input))
	require.Equal(t, int64(1), redis.incrValues[counterKey("imps", payload.AdID, formatDay(now))])
	require.Equal(t, float64(payload.BidCents)/1000.0, redis.floatValues[counterKey("spend", payload.AdID, formatDay(now))])

	// second call should not increment
	require.NoError(t, svc.TrackImpression(context.Background(), input))
	require.Equal(t, int64(1), redis.incrValues[counterKey("imps", payload.AdID, formatDay(now))])
}

func TestTrackClickDedupe(t *testing.T) {
	redis := newFakeRedis()
	svc := newTestService(nil, redis)

	now := time.Now().UTC()
	payload := token.Payload{
		TokenID:        uuid.New(),
		AdID:           uuid.New(),
		CreativeID:     uuid.New(),
		Placement:      enums.AdPlacementHero,
		TargetType:     enums.AdTargetTypeStore,
		TargetID:       uuid.New(),
		BuyerStoreID:   uuid.New(),
		EventType:      enums.AdEventFactTypeClick,
		OccurredAt:     now,
		ExpiresAt:      now.Add(24 * time.Hour),
		RequestID:      "req-click",
		BidCents:       250,
		DestinationURL: "https://click",
	}
	clickToken, err := token.MintToken("secret", payload)
	require.NoError(t, err)
	_, err = token.ParseToken("secret", clickToken)
	require.NoError(t, err)

	input := TrackClickInput{
		Token:        clickToken,
		RequestID:    payload.RequestID,
		BuyerStoreID: payload.BuyerStoreID,
		Now:          now,
	}
	result, err := svc.TrackClick(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "https://click", result.DestinationURL)
	require.Equal(t, int64(1), redis.incrValues[counterKey("clicks", payload.AdID, formatDay(now))])

	// second call should not increment
	result, err = svc.TrackClick(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "https://click", result.DestinationURL)
	require.Equal(t, int64(1), redis.incrValues[counterKey("clicks", payload.AdID, formatDay(now))])
}

func setupAdsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	const schema = `
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
  destination_url TEXT NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);
`
	require.NoError(t, db.Exec(schema).Error)
	return db
}

func insertStore(t *testing.T, db *gorm.DB, storeID uuid.UUID) {
	t.Helper()
	if err := db.Exec(
		"INSERT INTO stores (id, type, subscription_active, kyc_status) VALUES (?, ?, ?, ?)",
		storeID.String(),
		string(enums.StoreTypeVendor),
		true,
		string(enums.KYCStatusVerified),
	).Error; err != nil {
		t.Fatalf("insert store: %v", err)
	}
}

func insertAd(t *testing.T, db *gorm.DB, adID uuid.UUID, storeID uuid.UUID, placement enums.AdPlacement, bid, dailyBudget int64) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		"INSERT INTO ads (id, store_id, status, placement, target_type, target_id, bid_cents, daily_budget_cents, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		adID.String(),
		storeID.String(),
		string(enums.AdStatusActive),
		string(placement),
		string(enums.AdTargetTypeStore),
		uuid.New().String(),
		bid,
		dailyBudget,
		now,
		now,
	).Error; err != nil {
		t.Fatalf("insert ad: %v", err)
	}
}

func insertCreative(t *testing.T, db *gorm.DB, creativeID, adID uuid.UUID, url string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		"INSERT INTO ad_creatives (id, ad_id, destination_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		creativeID.String(),
		adID.String(),
		url,
		now,
		now,
	).Error; err != nil {
		t.Fatalf("insert creative: %v", err)
	}
}

func newTestService(repo *Repository, redis redisStore) *service {
	return &service{
		repo:        repo,
		redis:       redis,
		attachments: noopAttachmentReconciler{},
		analytics:   stubAnalytics{},
		tokenSecret: "secret",
		tokenTTL:    24 * time.Hour,
	}
}

type stubAnalytics struct{}

func (stubAnalytics) Query(_ context.Context, _ types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	return &types.MarketplaceQueryResponse{}, nil
}

func (stubAnalytics) QueryAd(_ context.Context, _ types.AdQueryRequest) (*types.AdQueryResponse, error) {
	return &types.AdQueryResponse{}, nil
}

type noopAttachmentReconciler struct{}

func (noopAttachmentReconciler) Reconcile(_ context.Context, _ *gorm.DB, _ string, _ uuid.UUID, _ uuid.UUID, _ []uuid.UUID, _ []uuid.UUID) error {
	return nil
}

type fakeRedis struct {
	values       map[string]string
	setnx        map[string]struct{}
	incrValues   map[string]int64
	floatValues  map[string]float64
	getErr       error
	setnxErr     error
	incrErr      error
	incrFloatErr error
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		values:      make(map[string]string),
		setnx:       make(map[string]struct{}),
		incrValues:  make(map[string]int64),
		floatValues: make(map[string]float64),
	}
}

func (f *fakeRedis) Get(_ context.Context, _ string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return "", redis.Nil
}

func (f *fakeRedis) SetNX(_ context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if f.setnxErr != nil {
		return false, f.setnxErr
	}
	if _, exists := f.setnx[key]; exists {
		return false, nil
	}
	f.setnx[key] = struct{}{}
	return true, nil
}

func (f *fakeRedis) Incr(_ context.Context, key string) (int64, error) {
	if f.incrErr != nil {
		return 0, f.incrErr
	}
	f.incrValues[key]++
	f.values[key] = fmt.Sprintf("%d", f.incrValues[key])
	return f.incrValues[key], nil
}

func (f *fakeRedis) IncrByFloat(_ context.Context, key string, value float64) (float64, error) {
	if f.incrFloatErr != nil {
		return 0, f.incrFloatErr
	}
	f.floatValues[key] += value
	f.values[key] = fmt.Sprintf("%f", f.floatValues[key])
	return f.floatValues[key], nil
}
