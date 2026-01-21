package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
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

func TestStoreUpdateSuccess(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	payload := []byte(`{
		"company_name": "Updated",
		"phone": "+15551234567",
		"banner_url": "https://example.com/banner",
		"ratings": {"quality": 5},
		"categories": ["flower","edibles"]
	}`)
	respDTO := &stores.StoreDTO{
		ID:          storeID,
		CompanyName: "Updated",
		Phone:       stringPtr("+15551234567"),
		BannerURL:   stringPtr("https://example.com/banner"),
		Ratings:     map[string]int{"quality": 5},
		Categories:  []string{"flower", "edibles"},
	}
	handler := StoreUpdate(stubStoreService{updateResp: respDTO}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/stores/me", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
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
	if envelope.Data.CompanyName != "Updated" {
		t.Fatalf("expected updated company name, got %s", envelope.Data.CompanyName)
	}
	if envelope.Data.Ratings["quality"] != 5 {
		t.Fatalf("expected rating quality=5 got %v", envelope.Data.Ratings)
	}
}

func TestStoreUpdateRejectsAddress(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	payload := []byte(`{"address": {"line1": "1"}}`)
	handler := StoreUpdate(stubStoreService{}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/stores/me", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for address field got %d", rec.Code)
	}
}

func TestStoreUpdateForbidden(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	handler := StoreUpdate(stubStoreService{updateErr: pkgerrors.New(pkgerrors.CodeForbidden, "denied")}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/stores/me", bytes.NewReader([]byte(`{"company_name": "nope"}`)))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", rec.Code)
	}
}

func TestStoreUsersSuccess(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	members := []memberships.StoreUserDTO{
		{
			MembershipID: uuid.New(),
			StoreID:      storeID,
			UserID:       uuid.New(),
			Email:        "member@example.com",
			FirstName:    "Member",
			LastName:     "User",
			Role:         enums.MemberRoleManager,
			Status:       enums.MembershipStatusActive,
			CreatedAt:    time.Now(),
		},
	}
	handler := StoreUsers(stubStoreService{users: members}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/me/users", nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	var envelope struct {
		Data []memberships.StoreUserDTO `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("expected 1 member got %d", len(envelope.Data))
	}
	if envelope.Data[0].Email != "member@example.com" {
		t.Fatalf("unexpected user email %s", envelope.Data[0].Email)
	}
}

func TestStoreUsersMissingContext(t *testing.T) {
	handler := StoreUsers(stubStoreService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/me/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 missing store context got %d", rec.Code)
	}
}

func TestStoreUsersUnauthorized(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	handler := StoreUsers(stubStoreService{usersErr: pkgerrors.New(pkgerrors.CodeForbidden, "denied")}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/me/users", nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when service denies got %d", rec.Code)
	}
}

type stubStoreService struct {
	dto        *stores.StoreDTO
	err        error
	updateResp *stores.StoreDTO
	updateErr  error
	users      []memberships.StoreUserDTO
	usersErr   error
}

func (s stubStoreService) GetByID(_ context.Context, _ uuid.UUID) (*stores.StoreDTO, error) {
	return s.dto, s.err
}

func (s stubStoreService) Update(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	return s.updateResp, s.updateErr
}

func (s stubStoreService) ListUsers(_ context.Context, _ uuid.UUID, _ uuid.UUID) ([]memberships.StoreUserDTO, error) {
	return s.users, s.usersErr
}

func stringPtr(s string) *string { return &s }
