package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

func TestStoreProfileSuccess(t *testing.T) {
	storeID := uuid.New()
	dto := &stores.StoreDTO{
		ID:                   storeID,
		Type:                 enums.StoreTypeVendor,
		CompanyName:          "Vendor HQ",
		KYCStatus:            enums.KYCStatusVerified,
		SubscriptionActive:   true,
		DeliveryRadiusMeters: 8000,
		Address: types.Address{
			Line1:      "456 Market St",
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
		OwnerID:   uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	handler := StoreProfile(stubStoreService{dto: dto}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/me", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}

	var envelope struct {
		Data stores.StoreDTO `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != storeID {
		t.Fatalf("expected id %s got %s", storeID, envelope.Data.ID)
	}
}

func TestStoreProfileNotFound(t *testing.T) {
	storeID := uuid.New()
	handler := StoreProfile(stubStoreService{err: pkgerrors.New(pkgerrors.CodeNotFound, "missing")}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/me", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", rec.Code)
	}
}

func TestStoreProfileMissingContext(t *testing.T) {
	handler := StoreProfile(stubStoreService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", rec.Code)
	}
}

type stubStoreService struct {
	dto *stores.StoreDTO
	err error
}

func (s stubStoreService) GetByID(_ context.Context, _ uuid.UUID) (*stores.StoreDTO, error) {
	return s.dto, s.err
}
