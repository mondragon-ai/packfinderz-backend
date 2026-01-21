package stores

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestNewServiceRequiresRepo(t *testing.T) {
	_, err := NewService(nil)
	if err == nil {
		t.Fatal("expected error creating service without repo")
	}
}

func TestServiceGetByIDSuccess(t *testing.T) {
	store := &models.Store{
		ID:                   uuid.New(),
		Type:                 enums.StoreTypeBuyer,
		CompanyName:          "Test Store",
		KYCStatus:            enums.KYCStatusVerified,
		SubscriptionActive:   true,
		DeliveryRadiusMeters: 5000,
		Address: types.Address{
			Line1:      "123 Main St",
			City:       "Oklahoma City",
			State:      "OK",
			PostalCode: "73102",
			Country:    "US",
			Lat:        35.4676,
			Lng:        -97.5164,
		},
		Geom: types.GeographyPoint{
			Lat: 35.4676,
			Lng: -97.5164,
		},
		OwnerID:     uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Phone:       stringPtr("405-555-0000"),
		Email:       stringPtr("owner@example.com"),
		Description: stringPtr("flagship store"),
	}
	repo := stubStoreRepo{store: store}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	dto, err := svc.GetByID(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	if dto.ID != store.ID {
		t.Fatalf("expected id %s got %s", store.ID, dto.ID)
	}
	if dto.CompanyName != store.CompanyName {
		t.Fatalf("expected company name %s got %s", store.CompanyName, dto.CompanyName)
	}
	if dto.Phone == nil || *dto.Phone != *store.Phone {
		t.Fatalf("expected phone %q got %v", *store.Phone, dto.Phone)
	}
	if dto.Address.Line1 != store.Address.Line1 {
		t.Fatalf("address mismatch: expected %s got %s", store.Address.Line1, dto.Address.Line1)
	}
}

func TestServiceGetByIDNotFound(t *testing.T) {
	repo := stubStoreRepo{err: gorm.ErrRecordNotFound}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, gotErr := svc.GetByID(context.Background(), uuid.New())
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(gotErr); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("expected not found code, got %v", gotErr)
	}
}

func TestServiceGetByIDDependencyError(t *testing.T) {
	repo := stubStoreRepo{err: errors.New("boom")}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, gotErr := svc.GetByID(context.Background(), uuid.New())
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(gotErr); typed == nil || typed.Code() != pkgerrors.CodeDependency {
		t.Fatalf("expected dependency error, got %v", gotErr)
	}
}

type stubStoreRepo struct {
	store *models.Store
	err   error
}

func (s stubStoreRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error) {
	return s.store, s.err
}

func stringPtr(s string) *string { return &s }
