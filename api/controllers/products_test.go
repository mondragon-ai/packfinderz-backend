package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	productsvc "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
)

func TestVendorDeleteProduct(t *testing.T) {
	logg := logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard})
	storeID := uuid.New()
	userID := uuid.New()
	productID := uuid.New()

	makeRequest := func(ctx context.Context) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/vendor/products/"+productID.String(), nil)
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("productId", productID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		VendorDeleteProduct(&stubDeleteProductService{}, logg).ServeHTTP(rec, req)
		return rec
	}

	t.Run("missing store", func(t *testing.T) {
		ctx := middleware.WithUserID(context.Background(), userID.String())
		rec := makeRequest(ctx)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 when store missing, got %d", rec.Code)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		ctx := middleware.WithStoreID(context.Background(), storeID.String())
		rec := makeRequest(ctx)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 when user missing, got %d", rec.Code)
		}
	})

	t.Run("invalid product id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), chi.RouteCtxKey, chi.NewRouteContext())
		ctx = middleware.WithStoreID(ctx, storeID.String())
		ctx = middleware.WithUserID(ctx, userID.String())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/vendor/products/invalid", nil)
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("productId", "not-a-uuid")
		ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		VendorDeleteProduct(&stubDeleteProductService{}, logg).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid id, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx := middleware.WithStoreID(context.Background(), storeID.String())
		ctx = middleware.WithUserID(ctx, userID.String())
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("productId", productID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/vendor/products/"+productID.String(), nil)
		req = req.WithContext(ctx)

		stub := &stubDeleteProductService{}
		rec := httptest.NewRecorder()
		VendorDeleteProduct(stub, logg).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 on success, got %d", rec.Code)
		}
		if !stub.called {
			t.Fatalf("expected DeleteProduct to be invoked")
		}
	})
}

type stubDeleteProductService struct {
	called bool
}

func (s *stubDeleteProductService) CreateProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, input productsvc.CreateProductInput) (*productsvc.ProductDTO, error) {
	panic("unimplemented")
}

func (s *stubDeleteProductService) UpdateProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, productID uuid.UUID, input productsvc.UpdateProductInput) (*productsvc.ProductDTO, error) {
	panic("unimplemented")
}

func (s *stubDeleteProductService) DeleteProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, productID uuid.UUID) error {
	s.called = true
	return nil
}

func (*stubDeleteProductService) ListProducts(ctx context.Context, input productsvc.ListProductsInput) (*productsvc.ProductListResult, error) {
	return nil, nil
}

func TestBrowseProducts(t *testing.T) {
	logg := logger.New(logger.Options{ServiceName: "test", Level: logger.ParseLevel("debug"), Output: io.Discard})
	storeID := uuid.New()
	userID := uuid.New()

	t.Run("buyer missing state", func(t *testing.T) {
		ctx := middleware.WithStoreID(context.Background(), storeID.String())
		ctx = middleware.WithUserID(ctx, userID.String())
		ctx = middleware.WithStoreType(ctx, enums.StoreTypeBuyer)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler := BrowseProducts(&stubProductListService{}, stubStoreService{
			dto: &stores.StoreDTO{
				ID:      storeID,
				Type:    enums.StoreTypeBuyer,
				OwnerID: uuid.New(),
				Address: types.Address{
					Line1:      "123 Test",
					City:       "Tulsa",
					State:      "OK",
					PostalCode: "74101",
					Country:    "US",
					Lat:        36.12,
					Lng:        -95.9,
				},
			},
		}, logg)

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when state missing, got %d", rec.Code)
		}
	})

	t.Run("buyer state mismatch", func(t *testing.T) {
		ctx := middleware.WithStoreID(context.Background(), storeID.String())
		ctx = middleware.WithUserID(ctx, userID.String())
		ctx = middleware.WithStoreType(ctx, enums.StoreTypeBuyer)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products?state=TX", nil)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler := BrowseProducts(&stubProductListService{}, stubStoreService{
			dto: &stores.StoreDTO{
				ID:      storeID,
				Type:    enums.StoreTypeBuyer,
				OwnerID: uuid.New(),
				Address: types.Address{
					Line1:      "123 Test",
					City:       "Tulsa",
					State:      "OK",
					PostalCode: "74101",
					Country:    "US",
					Lat:        36.12,
					Lng:        -95.9,
				},
			},
		}, logg)

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when state mismatched, got %d", rec.Code)
		}
	})

	t.Run("vendor success", func(t *testing.T) {
		ctx := middleware.WithStoreID(context.Background(), storeID.String())
		ctx = middleware.WithUserID(ctx, userID.String())
		ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=1&q=grid&has_promo=true", nil)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		stubSvc := &stubProductListService{
			result: &productsvc.ProductListResult{
				Products: []productsvc.ProductSummary{
					{ID: uuid.New()},
				},
				NextCursor: "next-cursor",
			},
		}
		handler := BrowseProducts(stubSvc, stubStoreService{}, logg)

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on success, got %d", rec.Code)
		}

		var envelope struct {
			Data productsvc.ProductListResult `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if envelope.Data.NextCursor != "next-cursor" {
			t.Fatalf("expected next cursor, got %s", envelope.Data.NextCursor)
		}
		if len(envelope.Data.Products) != 1 {
			t.Fatalf("expected product summary, got %d", len(envelope.Data.Products))
		}
		if stubSvc.lastInput.Filters.HasPromo == nil || !*stubSvc.lastInput.Filters.HasPromo {
			t.Fatalf("expected has_promo true in filters")
		}
		if stubSvc.lastInput.Filters.Query != "grid" {
			t.Fatalf("expected query trimmed to %q, got %q", "grid", stubSvc.lastInput.Filters.Query)
		}
	})
}

type stubProductListService struct {
	lastInput productsvc.ListProductsInput
	result    *productsvc.ProductListResult
	err       error
}

func (s *stubProductListService) CreateProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, input productsvc.CreateProductInput) (*productsvc.ProductDTO, error) {
	return nil, nil
}

func (s *stubProductListService) UpdateProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, productID uuid.UUID, input productsvc.UpdateProductInput) (*productsvc.ProductDTO, error) {
	return nil, nil
}

func (s *stubProductListService) DeleteProduct(ctx context.Context, userID uuid.UUID, storeID uuid.UUID, productID uuid.UUID) error {
	return nil
}

func (s *stubProductListService) ListProducts(ctx context.Context, input productsvc.ListProductsInput) (*productsvc.ProductListResult, error) {
	s.lastInput = input
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}
