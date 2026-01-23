package controllers

import (
	"net/http"
	"strings"

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
