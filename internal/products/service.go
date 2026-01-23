package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service exposes vendor product management operations.
type Service interface {
	CreateProduct(ctx context.Context, userID, storeID uuid.UUID, input CreateProductInput) (*ProductDTO, error)
}

// CreateProductInput holds the validated payload to create a product.
type CreateProductInput struct {
	SKU                 string
	Title               string
	Subtitle            *string
	BodyHTML            *string
	Category            enums.ProductCategory
	Feelings            []string
	Flavors             []string
	Usage               []string
	Strain              *string
	Classification      *enums.ProductClassification
	Unit                enums.ProductUnit
	MOQ                 int
	PriceCents          int
	CompareAtPriceCents *int
	IsActive            bool
	IsFeatured          bool
	THCPercent          *float64
	CBDPercent          *float64
	Inventory           InventoryInput
	MediaIDs            []uuid.UUID
	VolumeDiscounts     []VolumeDiscountInput
}

// InventoryInput captures the starting quantity for a product.
type InventoryInput struct {
	AvailableQty int
	ReservedQty  int
}

// VolumeDiscountInput defines a tiered price for a given min quantity.
type VolumeDiscountInput struct {
	MinQty         int
	UnitPriceCents int
}

type storeLoader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error)
}

type membershipChecker interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

type mediaReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
}

// service implements the product service.
type service struct {
	repo              *Repository
	dbClient          *db.Client
	storeRepo         storeLoader
	membershipChecker membershipChecker
	mediaRepo         mediaReader
}

// NewService constructs a product service instance.
func NewService(repo *Repository, dbClient *db.Client, storeRepo storeLoader, membershipChecker membershipChecker, mediaRepo mediaReader) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("product repository required")
	}
	if dbClient == nil {
		return nil, fmt.Errorf("db client required")
	}
	if storeRepo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	if membershipChecker == nil {
		return nil, fmt.Errorf("membership checker required")
	}
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	return &service{
		repo:              repo,
		dbClient:          dbClient,
		storeRepo:         storeRepo,
		membershipChecker: membershipChecker,
		mediaRepo:         mediaRepo,
	}, nil
}

// CreateProduct creates the product with inventory, discounts, and media.
func (s *service) CreateProduct(ctx context.Context, userID, storeID uuid.UUID, input CreateProductInput) (*ProductDTO, error) {
	if input.Inventory.ReservedQty > input.Inventory.AvailableQty {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "reserved_qty cannot exceed available_qty")
	}

	if err := s.ensureVendorStore(ctx, storeID); err != nil {
		return nil, err
	}
	if err := s.ensureUserRole(ctx, userID, storeID); err != nil {
		return nil, err
	}

	if err := ensureUniqueDiscounts(input.VolumeDiscounts); err != nil {
		return nil, err
	}

	var createdProductID uuid.UUID
	if err := s.dbClient.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)

		product := &models.Product{
			StoreID:             storeID,
			SKU:                 input.SKU,
			Title:               input.Title,
			Subtitle:            input.Subtitle,
			BodyHTML:            input.BodyHTML,
			Category:            input.Category,
			Feelings:            input.Feelings,
			Flavors:             input.Flavors,
			Usage:               input.Usage,
			Strain:              input.Strain,
			Classification:      input.Classification,
			Unit:                input.Unit,
			MOQ:                 input.MOQ,
			PriceCents:          input.PriceCents,
			CompareAtPriceCents: input.CompareAtPriceCents,
			IsActive:            input.IsActive,
			IsFeatured:          input.IsFeatured,
			THCPercent:          input.THCPercent,
			CBDPercent:          input.CBDPercent,
		}

		created, err := txRepo.CreateProduct(ctx, product)
		if err != nil {
			return err
		}
		createdProductID = created.ID

		inventory := &models.InventoryItem{
			ProductID:    created.ID,
			AvailableQty: input.Inventory.AvailableQty,
			ReservedQty:  input.Inventory.ReservedQty,
		}
		if _, err := txRepo.UpsertInventory(ctx, inventory); err != nil {
			return err
		}

		for _, discount := range input.VolumeDiscounts {
			tier := &models.ProductVolumeDiscount{
				ProductID:      created.ID,
				MinQty:         discount.MinQty,
				UnitPriceCents: discount.UnitPriceCents,
			}
			if _, err := txRepo.CreateVolumeDiscount(ctx, tier); err != nil {
				return err
			}
		}

		if len(input.MediaIDs) > 0 {
			for i, mediaID := range input.MediaIDs {
				mediaRow, err := s.mediaRepo.FindByID(ctx, mediaID)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return pkgerrors.New(pkgerrors.CodeValidation, "media not found")
					}
					return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load media")
				}
				if mediaRow.StoreID != storeID {
					return pkgerrors.New(pkgerrors.CodeValidation, "media must belong to the active store")
				}
				if mediaRow.Kind != enums.MediaKindProduct {
					return pkgerrors.New(pkgerrors.CodeValidation, "media must be product kind")
				}
				pm := &models.ProductMedia{
					ProductID: created.ID,
					GCSKey:    mediaRow.GCSKey,
					Position:  i,
				}
				if err := tx.WithContext(ctx).Create(pm).Error; err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		if pkgerrors.As(err) != nil {
			return nil, err
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create product")
	}

	product, summary, err := s.repo.GetProductDetail(ctx, createdProductID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product detail")
	}
	return NewProductDTO(product, summary), nil
}

func (s *service) ensureVendorStore(ctx context.Context, storeID uuid.UUID) error {
	store, err := s.storeRepo.FindByID(ctx, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
	}
	if store.Type != enums.StoreTypeVendor {
		return pkgerrors.New(pkgerrors.CodeForbidden, "store is not a vendor")
	}
	return nil
}

func (s *service) ensureUserRole(ctx context.Context, userID, storeID uuid.UUID) error {
	allowed := []enums.MemberRole{
		enums.MemberRoleOwner,
		enums.MemberRoleAdmin,
		enums.MemberRoleManager,
		enums.MemberRoleStaff,
		enums.MemberRoleAgent,
		enums.MemberRoleOps,
	}
	ok, err := s.membershipChecker.UserHasRole(ctx, userID, storeID, allowed...)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership")
	}
	if !ok {
		return pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}
	return nil
}

func ensureUniqueDiscounts(discounts []VolumeDiscountInput) error {
	seen := make(map[int]struct{}, len(discounts))
	for _, tier := range discounts {
		if _, ok := seen[tier.MinQty]; ok {
			return pkgerrors.New(pkgerrors.CodeValidation, "duplicate volume discount min_qty")
		}
		seen[tier.MinQty] = struct{}{}
	}
	return nil
}
