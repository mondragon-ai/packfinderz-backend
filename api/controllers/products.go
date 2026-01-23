package controllers

import (
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    "github.com/angelmondragon/packfinderz-backend/api/middleware"
    "github.com/angelmondragon/packfinderz-backend/api/responses"
    "github.com/angelmondragon/packfinderz-backend/api/validators"
    productsvc "github.com/angelmondragon/packfinderz-backend/internal/products"
    "github.com/angelmondragon/packfinderz-backend/pkg/enums"
    pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
    "github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// VendorCreateProduct handles product creation for vendor stores.
func VendorCreateProduct(svc productsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "product service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		var payload createProductRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toCreateInput()
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		product, err := svc.CreateProduct(r.Context(), uid, sid, input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, product)
	}
}

// VendorUpdateProduct handles patching existing products.
func VendorUpdateProduct(svc productsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "product service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}

		productIDParam := strings.TrimSpace(chi.URLParam(r, "productId"))
		if productIDParam == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "product id is required"))
			return
		}

		productID, err := uuid.Parse(productIDParam)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid product id"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		var payload updateProductRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toUpdateInput()
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		product, err := svc.UpdateProduct(r.Context(), uid, sid, productID, input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, product)
	}
}

type createProductRequest struct {
	SKU                 string                        `json:"sku" validate:"required"`
	Title               string                        `json:"title" validate:"required"`
	Subtitle            *string                       `json:"subtitle,omitempty"`
	BodyHTML            *string                       `json:"body_html,omitempty"`
	Category            string                        `json:"category" validate:"required"`
	Feelings            []string                      `json:"feelings" validate:"required,dive,required"`
	Flavors             []string                      `json:"flavors" validate:"required,dive,required"`
	Usage               []string                      `json:"usage" validate:"required,dive,required"`
	Strain              *string                       `json:"strain,omitempty"`
	Classification      *string                       `json:"classification,omitempty"`
	Unit                string                        `json:"unit" validate:"required"`
	MOQ                 int                           `json:"moq" validate:"required,min=1"`
	PriceCents          int                           `json:"price_cents" validate:"required,min=0"`
	CompareAtPriceCents *int                          `json:"compare_at_price_cents,omitempty" validate:"omitempty,min=0"`
	IsActive            *bool                         `json:"is_active,omitempty"`
	IsFeatured          *bool                         `json:"is_featured,omitempty"`
	THCPercent          *float64                      `json:"thc_percent,omitempty" validate:"omitempty,gte=0,lte=100"`
	CBDPercent          *float64                      `json:"cbd_percent,omitempty" validate:"omitempty,gte=0,lte=100"`
	Inventory           createInventoryRequest        `json:"inventory" validate:"required,dive"`
	MediaIDs            []string                      `json:"media_ids,omitempty"`
	VolumeDiscounts     []createVolumeDiscountRequest `json:"volume_discounts,omitempty"`
}

type createInventoryRequest struct {
	AvailableQty int `json:"available_qty" validate:"required,min=0"`
	ReservedQty  int `json:"reserved_qty" validate:"omitempty,min=0"`
}

type createVolumeDiscountRequest struct {
	MinQty         int `json:"min_qty" validate:"required,min=1"`
	UnitPriceCents int `json:"unit_price_cents" validate:"required,min=0"`
}

type updateProductRequest struct {
	SKU                 *string                        `json:"sku,omitempty"`
	Title               *string                        `json:"title,omitempty"`
	Subtitle            *string                        `json:"subtitle,omitempty"`
	BodyHTML            *string                        `json:"body_html,omitempty"`
	Category            *string                        `json:"category,omitempty"`
	Feelings            *[]string                      `json:"feelings,omitempty"`
	Flavors             *[]string                      `json:"flavors,omitempty"`
	Usage               *[]string                      `json:"usage,omitempty"`
	Strain              *string                        `json:"strain,omitempty"`
	Classification      *string                        `json:"classification,omitempty"`
	Unit                *string                        `json:"unit,omitempty"`
	MOQ                 *int                           `json:"moq,omitempty" validate:"omitempty,min=1"`
	PriceCents          *int                           `json:"price_cents,omitempty" validate:"omitempty,min=0"`
	CompareAtPriceCents *int                           `json:"compare_at_price_cents,omitempty" validate:"omitempty,min=0"`
	IsActive            *bool                          `json:"is_active,omitempty"`
	IsFeatured          *bool                          `json:"is_featured,omitempty"`
	THCPercent          *float64                       `json:"thc_percent,omitempty" validate:"omitempty,gte=0,lte=100"`
	CBDPercent          *float64                       `json:"cbd_percent,omitempty" validate:"omitempty,gte=0,lte=100"`
	Inventory           *updateInventoryRequest        `json:"inventory,omitempty"`
	MediaIDs            *[]string                      `json:"media_ids,omitempty"`
	VolumeDiscounts     *[]updateVolumeDiscountRequest `json:"volume_discounts,omitempty"`
}

type updateInventoryRequest struct {
	AvailableQty *int `json:"available_qty,omitempty" validate:"omitempty,min=0"`
	ReservedQty  *int `json:"reserved_qty,omitempty" validate:"omitempty,min=0"`
}

type updateVolumeDiscountRequest struct {
	MinQty         int `json:"min_qty" validate:"required,min=1"`
	UnitPriceCents int `json:"unit_price_cents" validate:"required,min=0"`
}

func (r createProductRequest) toCreateInput() (productsvc.CreateProductInput, error) {
	category, err := enums.ParseProductCategory(strings.TrimSpace(r.Category))
	if err != nil {
		return productsvc.CreateProductInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid category")
	}

	unit, err := enums.ParseProductUnit(strings.TrimSpace(r.Unit))
	if err != nil {
		return productsvc.CreateProductInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid unit")
	}

	feelings, err := parseEnumStrings(r.Feelings, enums.ParseProductFeeling)
	if err != nil {
		return productsvc.CreateProductInput{}, err
	}
	flavors, err := parseEnumStrings(r.Flavors, enums.ParseProductFlavor)
	if err != nil {
		return productsvc.CreateProductInput{}, err
	}
	usage, err := parseEnumStrings(r.Usage, enums.ParseProductUsage)
	if err != nil {
		return productsvc.CreateProductInput{}, err
	}

	var classification *enums.ProductClassification
	if r.Classification != nil {
		parsed, err := enums.ParseProductClassification(strings.TrimSpace(*r.Classification))
		if err != nil {
			return productsvc.CreateProductInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid classification")
		}
		classification = &parsed
	}

	mediaIDs, err := parseUUIDList(r.MediaIDs)
	if err != nil {
		return productsvc.CreateProductInput{}, err
	}

	discounts := make([]productsvc.VolumeDiscountInput, 0, len(r.VolumeDiscounts))
	for _, tier := range r.VolumeDiscounts {
		discounts = append(discounts, productsvc.VolumeDiscountInput{
			MinQty:         tier.MinQty,
			UnitPriceCents: tier.UnitPriceCents,
		})
	}

	isActive := true
	if r.IsActive != nil {
		isActive = *r.IsActive
	}
	isFeatured := false
	if r.IsFeatured != nil {
		isFeatured = *r.IsFeatured
	}

	return productsvc.CreateProductInput{
		SKU:                 strings.TrimSpace(r.SKU),
		Title:               strings.TrimSpace(r.Title),
		Subtitle:            r.Subtitle,
		BodyHTML:            r.BodyHTML,
		Category:            category,
		Feelings:            feelings,
		Flavors:             flavors,
		Usage:               usage,
		Strain:              r.Strain,
		Classification:      classification,
		Unit:                unit,
		MOQ:                 r.MOQ,
		PriceCents:          r.PriceCents,
		CompareAtPriceCents: r.CompareAtPriceCents,
		IsActive:            isActive,
		IsFeatured:          isFeatured,
		THCPercent:          r.THCPercent,
		CBDPercent:          r.CBDPercent,
		Inventory: productsvc.InventoryInput{
			AvailableQty: r.Inventory.AvailableQty,
			ReservedQty:  r.Inventory.ReservedQty,
		},
		MediaIDs:        mediaIDs,
		VolumeDiscounts: discounts,
	}, nil
}

func (r updateProductRequest) toUpdateInput() (productsvc.UpdateProductInput, error) {
	var input productsvc.UpdateProductInput

	if r.SKU != nil {
		trimmed := strings.TrimSpace(*r.SKU)
		if trimmed == "" {
			return input, pkgerrors.New(pkgerrors.CodeValidation, "sku cannot be empty")
		}
		input.SKU = &trimmed
	}
	if r.Title != nil {
		trimmed := strings.TrimSpace(*r.Title)
		if trimmed == "" {
			return input, pkgerrors.New(pkgerrors.CodeValidation, "title cannot be empty")
		}
		input.Title = &trimmed
	}
	if r.Subtitle != nil {
		input.Subtitle = r.Subtitle
	}
	if r.BodyHTML != nil {
		input.BodyHTML = r.BodyHTML
	}
	if r.Category != nil {
		parsed, err := enums.ParseProductCategory(strings.TrimSpace(*r.Category))
		if err != nil {
			return input, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid category")
		}
		input.Category = &parsed
	}
	if r.Feelings != nil {
		values, err := parseEnumStrings(*r.Feelings, enums.ParseProductFeeling)
		if err != nil {
			return input, err
		}
		input.Feelings = &values
	}
	if r.Flavors != nil {
		values, err := parseEnumStrings(*r.Flavors, enums.ParseProductFlavor)
		if err != nil {
			return input, err
		}
		input.Flavors = &values
	}
	if r.Usage != nil {
		values, err := parseEnumStrings(*r.Usage, enums.ParseProductUsage)
		if err != nil {
			return input, err
		}
		input.Usage = &values
	}
	if r.Strain != nil {
		input.Strain = r.Strain
	}
	if r.Classification != nil {
		parsed, err := enums.ParseProductClassification(strings.TrimSpace(*r.Classification))
		if err != nil {
			return input, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid classification")
		}
		input.Classification = &parsed
	}
	if r.Unit != nil {
		parsed, err := enums.ParseProductUnit(strings.TrimSpace(*r.Unit))
		if err != nil {
			return input, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid unit")
		}
		input.Unit = &parsed
	}
	if r.MOQ != nil {
		input.MOQ = r.MOQ
	}
	if r.PriceCents != nil {
		input.PriceCents = r.PriceCents
	}
	if r.CompareAtPriceCents != nil {
		input.CompareAtPriceCents = r.CompareAtPriceCents
	}
	if r.IsActive != nil {
		input.IsActive = r.IsActive
	}
	if r.IsFeatured != nil {
		input.IsFeatured = r.IsFeatured
	}
	if r.THCPercent != nil {
		input.THCPercent = r.THCPercent
	}
	if r.CBDPercent != nil {
		input.CBDPercent = r.CBDPercent
	}

	if r.Inventory != nil {
		if r.Inventory.AvailableQty == nil || r.Inventory.ReservedQty == nil {
			return input, pkgerrors.New(pkgerrors.CodeValidation, "inventory available_qty and reserved_qty are required")
		}
		input.Inventory = &productsvc.InventoryInput{
			AvailableQty: *r.Inventory.AvailableQty,
			ReservedQty:  *r.Inventory.ReservedQty,
		}
	}

	if r.MediaIDs != nil {
		ids, err := parseUUIDList(*r.MediaIDs)
		if err != nil {
			return input, err
		}
		input.MediaIDs = &ids
	}

	if r.VolumeDiscounts != nil {
		tiers := make([]productsvc.VolumeDiscountInput, len(*r.VolumeDiscounts))
		for i, tier := range *r.VolumeDiscounts {
			tiers[i] = productsvc.VolumeDiscountInput{
				MinQty:         tier.MinQty,
				UnitPriceCents: tier.UnitPriceCents,
			}
		}
		input.VolumeDiscounts = &tiers
	}

	return input, nil
}
func parseEnumStrings[T interface{ String() string }](values []string, parser func(string) (T, error)) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "enum values cannot be empty")
		}
		parsed, err := parser(trimmed)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid enum value")
		}
		result = append(result, parsed.String())
	}
	return result, nil
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	result := make([]uuid.UUID, 0, len(values))
	for _, raw := range values {
		if raw == "" {
			continue
		}
		parsed, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid media id")
		}
		result = append(result, parsed)
	}
	return result, nil
}
