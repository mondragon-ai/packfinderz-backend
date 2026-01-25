package controllers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartUpsert handles upsert of the buyer's active cart.
func CartUpsert(svc cartsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "cart service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		buyerStoreID, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid buyer store id"))
			return
		}

		var payload upsertCartRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toInput()
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		record, err := svc.UpsertCart(r.Context(), buyerStoreID, input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, newCartRecordResponse(record))
	}
}

type upsertCartRequest struct {
	SessionID          *string                   `json:"session_id"`
	ShippingAddress    *types.Address            `json:"shipping_address,omitempty"`
	TotalDiscount      int                       `json:"total_discount" validate:"required"`
	Fees               int                       `json:"fees"`
	SubtotalCents      int                       `json:"subtotal_cents" validate:"required"`
	TotalCents         int                       `json:"total_cents" validate:"required"`
	CartLevelDiscounts []types.CartLevelDiscount `json:"cart_level_discount"`
	Items              []cartItemPayload         `json:"items" validate:"required,dive"`
}

type cartItemPayload struct {
	ProductID                       uuid.UUID `json:"product_id" validate:"required"`
	VendorStoreID                   uuid.UUID `json:"vendor_store_id" validate:"required"`
	Qty                             int       `json:"qty" validate:"required,min=1"`
	ProductSKU                      string    `json:"product_sku" validate:"required"`
	Unit                            string    `json:"unit" validate:"required"`
	UnitPriceCents                  int       `json:"unit_price_cents" validate:"required"`
	CompareAtUnitPriceCents         *int      `json:"compare_at_unit_price_cents"`
	AppliedVolumeTierMinQty         *int      `json:"applied_volume_tier_min_qty"`
	AppliedVolumeTierUnitPriceCents *int      `json:"applied_volume_tier_unit_price_cents"`
	DiscountedPrice                 *int      `json:"discounted_price"`
	SubTotalPrice                   *int      `json:"sub_total_price" validate:"required"`
	FeaturedImage                   *string   `json:"featured_image"`
	THCPercent                      *float64  `json:"thc_percent"`
	CBDPercent                      *float64  `json:"cbd_percent"`
}

func (r upsertCartRequest) toInput() (cartsvc.UpsertCartInput, error) {
	items := make([]cartsvc.CartItemInput, len(r.Items))
	for i, payload := range r.Items {
		unit, err := enums.ParseProductUnit(payload.Unit)
		if err != nil {
			return cartsvc.UpsertCartInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid unit")
		}
		items[i] = cartsvc.CartItemInput{
			ProductID:                       payload.ProductID,
			VendorStoreID:                   payload.VendorStoreID,
			Qty:                             payload.Qty,
			ProductSKU:                      payload.ProductSKU,
			Unit:                            unit,
			UnitPriceCents:                  payload.UnitPriceCents,
			CompareAtUnitPriceCents:         payload.CompareAtUnitPriceCents,
			AppliedVolumeTierMinQty:         payload.AppliedVolumeTierMinQty,
			AppliedVolumeTierUnitPriceCents: payload.AppliedVolumeTierUnitPriceCents,
			DiscountedPrice:                 payload.DiscountedPrice,
			SubTotalPrice:                   payload.SubTotalPrice,
			FeaturedImage:                   payload.FeaturedImage,
			THCPercent:                      payload.THCPercent,
			CBDPercent:                      payload.CBDPercent,
		}
	}

	return cartsvc.UpsertCartInput{
		SessionID:          r.SessionID,
		ShippingAddress:    r.ShippingAddress,
		TotalDiscount:      r.TotalDiscount,
		Fees:               r.Fees,
		SubtotalCents:      r.SubtotalCents,
		TotalCents:         r.TotalCents,
		CartLevelDiscounts: types.CartLevelDiscounts(r.CartLevelDiscounts),
		Items:              items,
	}, nil
}

type cartRecordResponse struct {
	ID                 uuid.UUID                `json:"id"`
	BuyerStoreID       uuid.UUID                `json:"buyer_store_id"`
	SessionID          *string                  `json:"session_id,omitempty"`
	Status             string                   `json:"status"`
	ShippingAddress    *types.Address           `json:"shipping_address,omitempty"`
	TotalDiscount      int                      `json:"total_discount"`
	Fees               int                      `json:"fees"`
	SubtotalCents      int                      `json:"subtotal_cents"`
	TotalCents         int                      `json:"total_cents"`
	CartLevelDiscounts types.CartLevelDiscounts `json:"cart_level_discount,omitempty"`
	Items              []cartItemResponse       `json:"items"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
}

type cartItemResponse struct {
	ID                              uuid.UUID `json:"id"`
	ProductID                       uuid.UUID `json:"product_id"`
	VendorStoreID                   uuid.UUID `json:"vendor_store_id"`
	Qty                             int       `json:"qty"`
	ProductSKU                      string    `json:"product_sku"`
	Unit                            string    `json:"unit"`
	UnitPriceCents                  int       `json:"unit_price_cents"`
	CompareAtUnitPriceCents         *int      `json:"compare_at_unit_price_cents,omitempty"`
	AppliedVolumeTierMinQty         *int      `json:"applied_volume_tier_min_qty,omitempty"`
	AppliedVolumeTierUnitPriceCents *int      `json:"applied_volume_tier_unit_price_cents,omitempty"`
	DiscountedPrice                 *int      `json:"discounted_price,omitempty"`
	SubTotalPrice                   *int      `json:"sub_total_price,omitempty"`
	FeaturedImage                   *string   `json:"featured_image,omitempty"`
	MOQ                             *int      `json:"moq,omitempty"`
	THCPercent                      *float64  `json:"thc_percent,omitempty"`
	CBDPercent                      *float64  `json:"cbd_percent,omitempty"`
	CreatedAt                       time.Time `json:"created_at"`
	UpdatedAt                       time.Time `json:"updated_at"`
}

func newCartRecordResponse(record *models.CartRecord) cartRecordResponse {
	items := make([]cartItemResponse, 0, len(record.Items))
	for _, item := range record.Items {
		items = append(items, cartItemResponse{
			ID:                              item.ID,
			ProductID:                       item.ProductID,
			VendorStoreID:                   item.VendorStoreID,
			Qty:                             item.Qty,
			ProductSKU:                      item.ProductSKU,
			Unit:                            string(item.Unit),
			UnitPriceCents:                  item.UnitPriceCents,
			CompareAtUnitPriceCents:         item.CompareAtUnitPriceCents,
			AppliedVolumeTierMinQty:         item.AppliedVolumeTierMinQty,
			AppliedVolumeTierUnitPriceCents: item.AppliedVolumeTierUnitPriceCents,
			DiscountedPrice:                 item.DiscountedPrice,
			SubTotalPrice:                   item.SubTotalPrice,
			FeaturedImage:                   item.FeaturedImage,
			MOQ:                             item.MOQ,
			THCPercent:                      item.THCPercent,
			CBDPercent:                      item.CBDPercent,
			CreatedAt:                       item.CreatedAt,
			UpdatedAt:                       item.UpdatedAt,
		})
	}

	return cartRecordResponse{
		ID:                 record.ID,
		BuyerStoreID:       record.BuyerStoreID,
		SessionID:          record.SessionID,
		Status:             string(record.Status),
		ShippingAddress:    record.ShippingAddress,
		TotalDiscount:      record.TotalDiscount,
		Fees:               record.Fees,
		SubtotalCents:      record.SubtotalCents,
		TotalCents:         record.TotalCents,
		CartLevelDiscounts: record.CartLevelDiscounts,
		Items:              items,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}
