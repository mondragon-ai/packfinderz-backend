package product

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	UpdateProduct(ctx context.Context, userID, storeID, productID uuid.UUID, input UpdateProductInput) (*ProductDTO, error)
	DeleteProduct(ctx context.Context, userID, storeID, productID uuid.UUID) error
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

// UpdateProductInput holds optional mutation values for a product.
type UpdateProductInput struct {
	SKU                 *string
	Title               *string
	Subtitle            *string
	BodyHTML            *string
	Category            *enums.ProductCategory
	Feelings            *[]string
	Flavors             *[]string
	Usage               *[]string
	Strain              *string
	Classification      *enums.ProductClassification
	Unit                *enums.ProductUnit
	MOQ                 *int
	PriceCents          *int
	CompareAtPriceCents *int
	IsActive            *bool
	IsFeatured          *bool
	THCPercent          *float64
	CBDPercent          *float64
	Inventory           *InventoryInput
	MediaIDs            *[]uuid.UUID
	VolumeDiscounts     *[]VolumeDiscountInput
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
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "db: insert product")
		}
		createdProductID = created.ID

		inventory := &models.InventoryItem{
			ProductID:    created.ID,
			AvailableQty: input.Inventory.AvailableQty,
			ReservedQty:  input.Inventory.ReservedQty,
		}
		if _, err := txRepo.UpsertInventory(ctx, inventory); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "db: upsert inventory")
		}

		for _, discount := range input.VolumeDiscounts {
			tier := &models.ProductVolumeDiscount{
				StoreID:        storeID,
				ProductID:      created.ID,
				MinQty:         discount.MinQty,
				UnitPriceCents: discount.UnitPriceCents,
			}
			if _, err := txRepo.CreateVolumeDiscount(ctx, tier); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "db: insert volume discount")
			}
		}

		if len(input.MediaIDs) > 0 {
			entries, err := s.buildProductMediaRows(ctx, storeID, created.ID, input.MediaIDs)
			if err != nil {
				return err
			}
			if err := txRepo.ReplaceProductMedia(ctx, created.ID, entries); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "db: replace product media")
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

// UpdateProduct updates an existing product and related rows.
func (s *service) UpdateProduct(ctx context.Context, userID, storeID, productID uuid.UUID, input UpdateProductInput) (*ProductDTO, error) {
	fmt.Printf("[UpdateProduct] START user=%s store=%s product=%s\n", userID, storeID, productID)

	if input.Inventory != nil {
		fmt.Printf("[UpdateProduct] input.Inventory available=%d reserved=%d\n", input.Inventory.AvailableQty, input.Inventory.ReservedQty)
	}
	if input.VolumeDiscounts != nil {
		fmt.Printf("[UpdateProduct] input.VolumeDiscounts count=%d\n", len(*input.VolumeDiscounts))
	}
	if input.MediaIDs != nil {
		fmt.Printf("[UpdateProduct] input.MediaIDs count=%d\n", len(*input.MediaIDs))
	}

	if input.Inventory != nil && input.Inventory.ReservedQty > input.Inventory.AvailableQty {
		fmt.Printf("[UpdateProduct] VALIDATION FAIL reserved > available\n")
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "reserved_qty cannot exceed available_qty")
	}

	fmt.Printf("[UpdateProduct] ensureVendorStore...\n")
	if err := s.ensureVendorStore(ctx, storeID); err != nil {
		fmt.Printf("[UpdateProduct] ensureVendorStore ERROR: %v\n", err)
		return nil, err
	}

	fmt.Printf("[UpdateProduct] ensureUserRole...\n")
	if err := s.ensureUserRole(ctx, userID, storeID); err != nil {
		fmt.Printf("[UpdateProduct] ensureUserRole ERROR: %v\n", err)
		return nil, err
	}

	if input.VolumeDiscounts != nil {
		fmt.Printf("[UpdateProduct] ensureUniqueDiscounts...\n")
		if err := ensureUniqueDiscounts(*input.VolumeDiscounts); err != nil {
			fmt.Printf("[UpdateProduct] ensureUniqueDiscounts ERROR: %v\n", err)
			return nil, err
		}
	}

	fmt.Printf("[UpdateProduct] repo.FindByID...\n")
	product, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "product not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product")
	}
	if product.StoreID != storeID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "product does not belong to store")
	}

	var updatedID uuid.UUID
	if err := s.dbClient.WithTx(ctx, func(tx *gorm.DB) error {
		fmt.Printf("[UpdateProduct.tx] START productID=%s\n", product.ID)
		txRepo := s.repo.WithTx(tx)

		fmt.Printf("[UpdateProduct.tx] applyUpdateToProduct...\n")
		applyUpdateToProduct(product, input)

		fmt.Printf("[UpdateProduct.tx] txRepo.UpdateProduct...\n")
		if _, err := txRepo.UpdateProduct(ctx, product); err != nil {
			fmt.Printf("[UpdateProduct.tx] UpdateProduct ERROR: %v\n", err)
			return err
		}

		fmt.Printf("[UpdateProduct.tx] inventory branch? %t\n", input.Inventory != nil)
		if input.Inventory != nil {
			fmt.Printf("[UpdateProduct.tx] about to build InventoryItem; product.Inventory nil? %t\n", product.Inventory == nil)

			// 👇 Put a marker RIGHT before the line that currently panics:
			fmt.Printf("[UpdateProduct.tx] building models.InventoryItem...\n")

			inventory := &models.InventoryItem{
				ProductID:    product.ID,
				AvailableQty: input.Inventory.AvailableQty,
				// ReservedQty:  product.Inventory.ReservedQty, // <- likely panic
				ReservedQty: input.Inventory.ReservedQty, // <- safe + correct
			}

			fmt.Printf("[UpdateProduct.tx] txRepo.UpsertInventory...\n")
			if _, err := txRepo.UpsertInventory(ctx, inventory); err != nil {
				fmt.Printf("[UpdateProduct.tx] UpsertInventory ERROR: %v\n", err)
				return err
			}
		}

		fmt.Printf("[UpdateProduct.tx] volume discounts branch? %t\n", input.VolumeDiscounts != nil)
		if input.VolumeDiscounts != nil {
			tiers := make([]models.ProductVolumeDiscount, len(*input.VolumeDiscounts))
			for i, tier := range *input.VolumeDiscounts {
				tiers[i] = models.ProductVolumeDiscount{
					StoreID:        storeID,
					ProductID:      product.ID,
					MinQty:         tier.MinQty,
					UnitPriceCents: tier.UnitPriceCents,
				}
			}
			if err := txRepo.ReplaceVolumeDiscounts(ctx, product.ID, tiers); err != nil {
				return err
			}
		}

		if input.MediaIDs != nil {
			fmt.Printf("[UpdateProduct.tx] buildProductMediaRows...\n")
			entries, err := s.buildProductMediaRows(ctx, storeID, product.ID, *input.MediaIDs)
			if err != nil {
				fmt.Printf("[UpdateProduct.tx] buildProductMediaRows ERROR: %v\n", err)
				return err
			}
			fmt.Printf("[UpdateProduct.tx] ReplaceProductMedia entries=%d...\n", len(entries))
			if err := txRepo.ReplaceProductMedia(ctx, product.ID, entries); err != nil {
				fmt.Printf("[UpdateProduct.tx] ReplaceProductMedia ERROR: %v\n", err)
				return err
			}
		}

		updatedID = product.ID
		fmt.Printf("[UpdateProduct.tx] COMMIT OK updatedID=%s\n", updatedID)
		return nil
	}); err != nil {
		if pkgerrors.As(err) != nil {
			return nil, err
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update product")
	}

	updated, summary, err := s.repo.GetProductDetail(ctx, updatedID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product detail")
	}
	return NewProductDTO(updated, summary), nil
}

// DeleteProduct removes a product and relies on FK cascades for related rows.
func (s *service) DeleteProduct(ctx context.Context, userID, storeID, productID uuid.UUID) error {
	if err := s.ensureVendorStore(ctx, storeID); err != nil {
		return err
	}
	if err := s.ensureUserRole(ctx, userID, storeID); err != nil {
		return err
	}

	product, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.New(pkgerrors.CodeNotFound, "product not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product")
	}
	if product.StoreID != storeID {
		return pkgerrors.New(pkgerrors.CodeForbidden, "product does not belong to store")
	}

	if err := s.repo.DeleteProduct(ctx, productID); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "delete product")
	}
	return nil
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

func applyUpdateToProduct(product *models.Product, input UpdateProductInput) {
	if input.SKU != nil {
		product.SKU = strings.TrimSpace(*input.SKU)
	}
	if input.Title != nil {
		product.Title = strings.TrimSpace(*input.Title)
	}
	if input.Subtitle != nil {
		product.Subtitle = input.Subtitle
	}
	if input.BodyHTML != nil {
		product.BodyHTML = input.BodyHTML
	}
	if input.Category != nil {
		product.Category = *input.Category
	}
	if input.Feelings != nil {
		product.Feelings = append([]string(nil), *input.Feelings...)
	}
	if input.Flavors != nil {
		product.Flavors = append([]string(nil), *input.Flavors...)
	}
	if input.Usage != nil {
		product.Usage = append([]string(nil), *input.Usage...)
	}
	if input.Strain != nil {
		product.Strain = input.Strain
	}
	if input.Classification != nil {
		product.Classification = input.Classification
	}
	if input.Unit != nil {
		product.Unit = *input.Unit
	}
	if input.MOQ != nil {
		product.MOQ = *input.MOQ
	}
	if input.PriceCents != nil {
		product.PriceCents = *input.PriceCents
	}
	if input.CompareAtPriceCents != nil {
		product.CompareAtPriceCents = input.CompareAtPriceCents
	}
	if input.IsActive != nil {
		product.IsActive = *input.IsActive
	}
	if input.IsFeatured != nil {
		product.IsFeatured = *input.IsFeatured
	}
	if input.THCPercent != nil {
		product.THCPercent = input.THCPercent
	}
	if input.CBDPercent != nil {
		product.CBDPercent = input.CBDPercent
	}
}

func (s *service) buildProductMediaRows(ctx context.Context, storeID, productID uuid.UUID, mediaIDs []uuid.UUID) ([]models.ProductMedia, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(mediaIDs))
	rows := make([]models.ProductMedia, 0, len(mediaIDs))
	for idx, mediaID := range mediaIDs {
		if _, ok := seen[mediaID]; ok {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "duplicate media ids")
		}
		seen[mediaID] = struct{}{}

		mediaRow, err := s.mediaRepo.FindByID(ctx, mediaID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, pkgerrors.New(pkgerrors.CodeValidation, "media not found")
			}
			return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load media")
		}
		if mediaRow.StoreID != storeID {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "media must belong to the active store")
		}
		if mediaRow.Kind != enums.MediaKindProduct {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "media must be product kind")
		}

		rows = append(rows, models.ProductMedia{
			ProductID: productID,
			GCSKey:    mediaRow.GCSKey,
			Position:  idx,
		})
	}
	return rows, nil
}
