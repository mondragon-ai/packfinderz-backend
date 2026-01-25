package cart

import (
	"context"
	"testing"

	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

func TestServiceGetActiveCartNotFound(t *testing.T) {
	t.Parallel()

	store := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US", Lat: 0, Lng: 0},
	}
	repo := &stubCartRepo{findErr: gorm.ErrRecordNotFound}
	svc := newTestService(repo, store)

	_, err := svc.GetActiveCart(context.Background(), store.ID)
	if err == nil {
		t.Fatal("expected error for missing cart")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestServiceGetActiveCartSuccess(t *testing.T) {
	t.Parallel()

	store := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US", Lat: 0, Lng: 0},
	}
	record := &models.CartRecord{ID: uuid.New(), BuyerStoreID: store.ID, Status: enums.CartStatusActive}
	repo := &stubCartRepo{record: record}
	svc := newTestService(repo, store)

	got, err := svc.GetActiveCart(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != record {
		t.Fatalf("expected record to match")
	}
}

func newTestService(repo CartRepository, store *stores.StoreDTO) Service {
	svc, err := NewService(repo, stubTxRunner{}, storeLoaderFunc(func(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
		return store, nil
	}), stubProductLoader{})
	if err != nil {
		panic(err)
	}
	return svc
}

type stubCartRepo struct {
	record  *models.CartRecord
	findErr error
}

func (s *stubCartRepo) WithTx(tx *gorm.DB) CartRepository { return s }
func (s *stubCartRepo) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.record == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.record, nil
}
func (s *stubCartRepo) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return nil, gorm.ErrRecordNotFound
}
func (s *stubCartRepo) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return record, nil
}
func (s *stubCartRepo) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return record, nil
}
func (s *stubCartRepo) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	return nil
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type storeLoaderFunc func(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error)

func (fn storeLoaderFunc) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	return fn(ctx, id)
}

type stubProductLoader struct{}

func (stubProductLoader) GetProductDetail(ctx context.Context, id uuid.UUID) (*models.Product, *products.VendorSummary, error) {
	return nil, nil, nil
}
