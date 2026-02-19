package wishlist

import (
	"context"
	"errors"

	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ServiceParams groups dependencies for the wishlist service.
type ServiceParams struct {
	WishlistRepo *Repository
	ProductRepo  *products.Repository
	StoreRepo    *stores.Repository
}

// Service exposes business rules for wishlist management.
type Service interface {
	GetWishlist(ctx context.Context, storeID uuid.UUID, cursor string, limit int) (WishlistItemsPageDTO, error)
	GetWishlistIDs(ctx context.Context, storeID uuid.UUID, cursor string, limit int) (WishlistIDsDTO, error)
	AddItem(ctx context.Context, storeID, productID uuid.UUID) error
	RemoveItem(ctx context.Context, storeID, productID uuid.UUID) error
}

type service struct {
	wishlistRepo *Repository
	productRepo  *products.Repository
	storeRepo    *stores.Repository
}

// NewService builds a wishlist service with the required dependencies.
func NewService(params ServiceParams) (Service, error) {
	if params.WishlistRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "wishlist repo is required")
	}
	if params.ProductRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "product repo is required")
	}
	if params.StoreRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store repo is required")
	}
	return &service{
		wishlistRepo: params.WishlistRepo,
		productRepo:  params.ProductRepo,
		storeRepo:    params.StoreRepo,
	}, nil
}

// GetWishlist returns the paginated wishlist for a buyer store.
func (s *service) GetWishlist(ctx context.Context, storeID uuid.UUID, cursor string, limit int) (WishlistItemsPageDTO, error) {
	if err := s.ensureBuyerStore(ctx, storeID); err != nil {
		return WishlistItemsPageDTO{}, err
	}
	return s.wishlistRepo.ListItems(ctx, storeID, cursor, limit)
}

// GetWishlistIDs returns all liked product IDs for the store.
func (s *service) GetWishlistIDs(ctx context.Context, storeID uuid.UUID, cursor string, limit int) (WishlistIDsDTO, error) {
	if err := s.ensureBuyerStore(ctx, storeID); err != nil {
		return WishlistIDsDTO{}, err
	}
	ids, err := s.wishlistRepo.ListItemIDs(ctx, storeID, cursor, limit)
	if err != nil {
		return WishlistIDsDTO{}, err
	}
	return ids, nil
}

// AddItem ensures the product exists and adds it to the wishlist.
func (s *service) AddItem(ctx context.Context, storeID, productID uuid.UUID) error {
	if err := s.ensureBuyerStore(ctx, storeID); err != nil {
		return err
	}
	if productID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "product id is required")
	}
	if _, err := s.productRepo.FindByID(ctx, productID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.Wrap(pkgerrors.CodeNotFound, err, "product not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product")
	}
	return s.wishlistRepo.AddItem(ctx, storeID, productID)
}

// RemoveItem drops the wishlist entry regardless of prior state.
func (s *service) RemoveItem(ctx context.Context, storeID, productID uuid.UUID) error {
	if err := s.ensureBuyerStore(ctx, storeID); err != nil {
		return err
	}
	return s.wishlistRepo.RemoveItem(ctx, storeID, productID)
}

func (s *service) ensureBuyerStore(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	store, err := s.storeRepo.FindByID(ctx, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.Wrap(pkgerrors.CodeNotFound, err, "store not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
	}
	if store.Type != enums.StoreTypeBuyer {
		return pkgerrors.New(pkgerrors.CodeForbidden, "wishlist access is restricted to buyer stores")
	}
	return nil
}
