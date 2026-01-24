package controllers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	productsvc "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
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
