package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

type stubCartFetcher struct {
	record *models.CartRecord
	err    error
}

func (s stubCartFetcher) UpsertCart(ctx context.Context, buyerStoreID uuid.UUID, input cartsvc.UpsertCartInput) (*models.CartRecord, error) {
	return nil, nil
}

func (s stubCartFetcher) GetActiveCart(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return s.record, s.err
}

func TestCartFetchSuccess(t *testing.T) {
	storeID := uuid.New()
	record := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: storeID,
		Status:       enums.CartStatusActive,
	}
	handler := CartFetch(stubCartFetcher{record: record}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data cartRecordResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != record.ID {
		t.Fatalf("unexpected cart id: %s", envelope.Data.ID)
	}
}

func TestCartFetchNotFound(t *testing.T) {
	storeID := uuid.New()
	handler := CartFetch(stubCartFetcher{err: pkgerrors.New(pkgerrors.CodeNotFound, "missing")}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.Code)
	}
}

func TestCartFetchMissingStoreContext(t *testing.T) {
	handler := CartFetch(stubCartFetcher{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}
