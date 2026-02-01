package cart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cartdto "github.com/angelmondragon/packfinderz-backend/api/controllers/cart/dto"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

type stubCartService struct {
	record         *models.CartRecord
	err            error
	lastQuoteInput cartsvc.QuoteCartInput
}

func (s *stubCartService) QuoteCart(ctx context.Context, buyerStoreID uuid.UUID, input cartsvc.QuoteCartInput) (*models.CartRecord, error) {
	s.lastQuoteInput = input
	return s.record, s.err
}

func (s *stubCartService) GetActiveCart(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return s.record, s.err
}

func TestCartFetchSuccess(t *testing.T) {
	storeID := uuid.New()
	record := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: storeID,
		Status:       enums.CartStatusActive,
	}
	handler := CartFetch(&stubCartService{record: record}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data cartdto.CartQuote `json:"data"`
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
	handler := CartFetch(&stubCartService{err: pkgerrors.New(pkgerrors.CodeNotFound, "missing")}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.Code)
	}
}

func TestCartFetchMissingStoreContext(t *testing.T) {
	handler := CartFetch(&stubCartService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}

func TestCartQuoteSuccess(t *testing.T) {
	storeID := uuid.New()
	productID := uuid.New()
	vendorID := uuid.New()
	record := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: storeID,
		Status:       enums.CartStatusActive,
	}
	service := &stubCartService{record: record}
	handler := CartQuote(service, nil)

	body := fmt.Sprintf(`{
		"buyer_store_id": "%s",
		"items": [{
			"product_id": "%s",
			"vendor_store_id": "%s",
			"quantity": 2
		}]
	}`, storeID, productID, vendorID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart", strings.NewReader(body))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var envelope struct {
		Data cartdto.CartQuote `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != record.ID {
		t.Fatalf("unexpected cart id: %s", envelope.Data.ID)
	}

	if len(service.lastQuoteInput.Items) != 1 {
		t.Fatalf("expected quote input with 1 item, got %d", len(service.lastQuoteInput.Items))
	}
	if service.lastQuoteInput.Items[0].ProductID != productID {
		t.Fatalf("expected product id %s, got %s", productID, service.lastQuoteInput.Items[0].ProductID)
	}
}

func TestCartQuoteBuyerStoreMismatch(t *testing.T) {
	storeID := uuid.New()
	otherStoreID := uuid.New()
	handler := CartQuote(&stubCartService{}, nil)

	body := fmt.Sprintf(`{
		"buyer_store_id": "%s",
		"items": [{
			"product_id": "%s",
			"vendor_store_id": "%s",
			"quantity": 1
		}]
	}`, otherStoreID, uuid.New(), uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart", strings.NewReader(body))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
}

func TestCartQuoteValidationFails(t *testing.T) {
	storeID := uuid.New()
	handler := CartQuote(&stubCartService{}, nil)

	body := fmt.Sprintf(`{
		"buyer_store_id": "%s",
		"items": []
	}`, storeID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart", strings.NewReader(body))
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
}
