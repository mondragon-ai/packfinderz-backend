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
	"github.com/go-chi/chi/v5"
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

func TestStoreRemoveUserSuccess(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()
	handler := StoreRemoveUser(stubStoreService{}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stores/me/users/"+targetID.String(), nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	req = withRouteParam(req, "userId", targetID.String())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	var envelope struct {
		Data interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data != nil {
		t.Fatalf("expected nil data got %v", envelope.Data)
	}
}

func TestStoreRemoveUserMissingStoreContext(t *testing.T) {
	handler := StoreRemoveUser(stubStoreService{}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stores/me/users/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 missing store context got %d", rec.Code)
	}
}

func TestStoreRemoveUserMissingUserContext(t *testing.T) {
	handler := StoreRemoveUser(stubStoreService{}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stores/me/users/"+uuid.NewString(), nil)
	ctx := middleware.WithStoreID(req.Context(), uuid.NewString())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 missing user context got %d", rec.Code)
	}
}

func TestStoreRemoveUserInvalidTarget(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	handler := StoreRemoveUser(stubStoreService{}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stores/me/users/not-a-uuid", nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	req = withRouteParam(req, "userId", "not-a-uuid")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid target got %d", rec.Code)
	}
}

func TestStoreRemoveUserForbidden(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	targetID := uuid.NewString()
	handler := StoreRemoveUser(stubStoreService{removeErr: pkgerrors.New(pkgerrors.CodeForbidden, "denied")}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stores/me/users/"+targetID, nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	req = withRouteParam(req, "userId", targetID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when service denies got %d", rec.Code)
	}
}

func TestStoreRemoveUserConflict(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	targetID := uuid.NewString()
	handler := StoreRemoveUser(stubStoreService{removeErr: pkgerrors.New(pkgerrors.CodeConflict, "cannot remove last owner")}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stores/me/users/"+targetID, nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	req = withRouteParam(req, "userId", targetID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for conflict got %d", rec.Code)
	}
}

func TestStoreInviteSuccess(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	payload := []byte(`{
		"email":"new@example.com",
		"first_name":"New",
		"last_name":"User",
		"role":"manager"
	}`)
	user := memberships.StoreUserDTO{
		MembershipID: uuid.New(),
		StoreID:      storeID,
		UserID:       uuid.New(),
		Email:        "new@example.com",
		FirstName:    "New",
		LastName:     "User",
		Role:         enums.MemberRoleManager,
		Status:       enums.MembershipStatusActive,
		CreatedAt:    time.Now(),
	}
	handler := StoreInvite(stubStoreService{inviteResp: &user, invitePassword: "tmp"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/me/users/invite", bytes.NewReader(payload))
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
		Data struct {
			User              memberships.StoreUserDTO `json:"user"`
			TemporaryPassword string                   `json:"temporary_password"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.User.Email != "new@example.com" {
		t.Fatalf("unexpected email %s", envelope.Data.User.Email)
	}
	if envelope.Data.TemporaryPassword != "tmp" {
		t.Fatalf("expected temp pass, got %s", envelope.Data.TemporaryPassword)
	}
}

func TestStoreInviteMissingContext(t *testing.T) {
	handler := StoreInvite(stubStoreService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/me/users/invite", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 missing context got %d", rec.Code)
	}
}

func TestStoreInviteDuplicate(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	reqBody := []byte(`{"email":"dup@example.com","first_name":"Dup","last_name":"User","role":"manager"}`)
	handler := StoreInvite(stubStoreService{inviteResp: &memberships.StoreUserDTO{Email: "dup@example.com"}}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/me/users/invite", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for duplicate invite got %d", rec.Code)
	}
	var envelope struct {
		Data struct {
			User memberships.StoreUserDTO `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.User.Email != "dup@example.com" {
		t.Fatalf("unexpected email %s", envelope.Data.User.Email)
	}
}

type stubStoreService struct {
	dto            *stores.StoreDTO
	err            error
	updateResp     *stores.StoreDTO
	updateErr      error
	users          []memberships.StoreUserDTO
	usersErr       error
	inviteResp     *memberships.StoreUserDTO
	inviteErr      error
	invitePassword string
	removeErr      error
}

func (s stubStoreService) GetByID(_ context.Context, _ uuid.UUID) (*stores.StoreDTO, error) {
	return s.dto, s.err
}

func (s stubStoreService) Update(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	return s.updateResp, s.updateErr
}

func (s stubStoreService) InviteUser(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	return s.inviteResp, s.invitePassword, s.inviteErr
}

func (s stubStoreService) ListUsers(_ context.Context, _ uuid.UUID, _ uuid.UUID) ([]memberships.StoreUserDTO, error) {
	return s.users, s.usersErr
}

func (s stubStoreService) RemoveUser(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) error {
	return s.removeErr
}

func stringPtr(s string) *string { return &s }

func withRouteParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
