package controllers

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/ads"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
)

var adTimeNowUTC = func() time.Time {
	return time.Now().UTC()
}

// VendorCreateAd handles POST /api/v1/vendors/ads.
func VendorCreateAd(svc ads.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "ads service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var payload createAdRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toCreateInput(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		ad, err := svc.CreateAd(r.Context(), input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, ad)
	}
}

// VendorListAds handles GET /api/v1/vendors/ads.
func VendorListAds(svc ads.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "ads service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		limit, err := validators.ParseQueryInt(r, "limit", pagination.DefaultLimit, 1, pagination.MaxLimit)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		page, err := validators.ParseQueryInt(r, "page", 1, 1, math.MaxInt32)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))

		filters, err := buildAdListFilters(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input := ads.ListAdsInput{
			StoreID: storeID,
			Filters: filters,
			Pagination: pagination.Params{
				Limit:  limit,
				Cursor: cursor,
			},
			Page: page,
		}

		result, err := svc.ListAds(r.Context(), input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, result)
	}
}

// VendorGetAdDetail handles GET /api/v1/vendors/ads/{adId}.
func VendorGetAdDetail(svc ads.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "ads service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		adIDParam := strings.TrimSpace(chi.URLParam(r, "adId"))
		adID, err := uuid.Parse(adIDParam)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid ad id"))
			return
		}

		start, end, err := resolveAdTimeframe(r, adTimeNowUTC())
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		detail, err := svc.GetAdDetail(r.Context(), ads.GetAdDetailInput{
			StoreID: storeID,
			AdID:    adID,
			Start:   start,
			End:     end,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, detail)
	}
}

func buildAdListFilters(r *http.Request) (ads.ListAdsFilters, error) {
	var filters ads.ListAdsFilters

	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		parsed, err := enums.ParseAdStatus(status)
		if err != nil {
			return filters, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid status")
		}
		filters.Status = &parsed
	}

	if placement := strings.TrimSpace(r.URL.Query().Get("placement")); placement != "" {
		parsed, err := enums.ParseAdPlacement(placement)
		if err != nil {
			return filters, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid placement")
		}
		filters.Placement = &parsed
	}

	if targetType := strings.TrimSpace(r.URL.Query().Get("target_type")); targetType != "" {
		parsed, err := enums.ParseAdTargetType(targetType)
		if err != nil {
			return filters, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid target type")
		}
		filters.TargetType = &parsed
	}

	if targetID := strings.TrimSpace(r.URL.Query().Get("target_id")); targetID != "" {
		parsed, err := uuid.Parse(targetID)
		if err != nil {
			return filters, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid target id")
		}
		filters.TargetID = &parsed
	}

	return filters, nil
}

func resolveAdTimeframe(r *http.Request, now time.Time) (time.Time, time.Time, error) {
	value := strings.TrimSpace(r.URL.Query().Get("timeframe"))
	if value == "" {
		value = "30d"
	}
	duration, ok := parseAdTimeframe(value)
	if !ok {
		return time.Time{}, time.Time{}, pkgerrors.New(pkgerrors.CodeValidation, "invalid timeframe")
	}

	end := now
	start := end.Add(-duration)
	return start, end, nil
}

func parseAdTimeframe(value string) (time.Duration, bool) {
	switch strings.ToLower(value) {
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	case "90d":
		return 90 * 24 * time.Hour, true
	case "1y":
		return 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

type createAdRequest struct {
	Status           string                    `json:"status" validate:"required"`
	Placement        string                    `json:"placement" validate:"required"`
	TargetType       string                    `json:"target_type" validate:"required"`
	TargetID         string                    `json:"target_id" validate:"required"`
	BidCents         int64                     `json:"bid_cents" validate:"required,min=0"`
	DailyBudgetCents int64                     `json:"daily_budget_cents" validate:"required,min=0"`
	StartsAt         *time.Time                `json:"starts_at,omitempty"`
	EndsAt           *time.Time                `json:"ends_at,omitempty"`
	Creatives        []createAdCreativeRequest `json:"creatives" validate:"required,min=1,dive"`
}

type createAdCreativeRequest struct {
	MediaID        *uuid.UUID `json:"media_id,omitempty"`
	DestinationURL string     `json:"destination_url" validate:"required,url"`
	Headline       *string    `json:"headline,omitempty"`
	Body           *string    `json:"body,omitempty"`
}

func (r *createAdRequest) toCreateInput(storeID uuid.UUID) (ads.CreateAdInput, error) {
	status, err := enums.ParseAdStatus(strings.TrimSpace(r.Status))
	if err != nil {
		return ads.CreateAdInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid status")
	}

	placement, err := enums.ParseAdPlacement(strings.TrimSpace(r.Placement))
	if err != nil {
		return ads.CreateAdInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid placement")
	}

	targetType, err := enums.ParseAdTargetType(strings.TrimSpace(r.TargetType))
	if err != nil {
		return ads.CreateAdInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid target type")
	}

	targetID, err := uuid.Parse(strings.TrimSpace(r.TargetID))
	if err != nil {
		return ads.CreateAdInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid target id")
	}

	creatives := make([]ads.AdCreativeInput, 0, len(r.Creatives))
	for _, creative := range r.Creatives {
		creatives = append(creatives, ads.AdCreativeInput{
			MediaID:        creative.MediaID,
			DestinationURL: strings.TrimSpace(creative.DestinationURL),
			Headline:       creative.Headline,
			Body:           creative.Body,
		})
	}

	return ads.CreateAdInput{
		StoreID:          storeID,
		Status:           status,
		Placement:        placement,
		TargetType:       targetType,
		TargetID:         targetID,
		BidCents:         r.BidCents,
		DailyBudgetCents: r.DailyBudgetCents,
		StartsAt:         r.StartsAt,
		EndsAt:           r.EndsAt,
		Creatives:        creatives,
	}, nil
}

func ServeAd(svc ads.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "ads service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromRequest(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		placementValue := strings.TrimSpace(r.URL.Query().Get("placement"))
		placement, err := enums.ParseAdPlacement(placementValue)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid placement"))
			return
		}

		requestID := uuid.NewString()
		result, err := svc.ServeAd(r.Context(), ads.ServeAdInput{
			Placement:    placement,
			BuyerStoreID: buyerStoreID,
			RequestID:    requestID,
			Now:          time.Now().UTC(),
		})
		if err != nil {
			if errors.Is(err, ads.ErrRedisUnavailable) {
				ctx := logg.WithFields(r.Context(), map[string]any{
					"placement":  placement,
					"request_id": requestID,
					"error":      err.Error(),
				})
				logg.Warn(ctx, "ads.serve.redis-unavailable")
				responses.WriteSuccess(w, nil)
				return
			}
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if result == nil {
			responses.WriteSuccess(w, nil)
			return
		}

		resp := serveAdResponse{
			AdID:       result.AdID,
			Placement:  result.Placement,
			TargetType: result.TargetType,
			TargetID:   result.TargetID,
			Creative:   result.Creative,
			RequestID:  result.RequestID,
			ViewToken:  result.ViewToken,
			ClickToken: result.ClickToken,
		}
		responses.WriteSuccess(w, resp)
	}
}

func TrackAdImpression(svc ads.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "ads service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromRequest(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var payload impressionRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input := ads.TrackImpressionInput{
			Token:        strings.TrimSpace(payload.ViewToken),
			RequestID:    strings.TrimSpace(payload.RequestID),
			BuyerStoreID: buyerStoreID,
			Now:          time.Now().UTC(),
		}

		if err := svc.TrackImpression(r.Context(), input); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func TrackAdClick(svc ads.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "ads service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromRequest(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		tokenValue := strings.TrimSpace(r.URL.Query().Get("token"))
		requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
		if tokenValue == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "token is required"))
			return
		}
		if requestID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "request_id is required"))
			return
		}

		input := ads.TrackClickInput{
			Token:        tokenValue,
			RequestID:    requestID,
			BuyerStoreID: buyerStoreID,
			Now:          time.Now().UTC(),
		}

		result, err := svc.TrackClick(r.Context(), input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if strings.TrimSpace(result.DestinationURL) == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "destination url missing"))
			return
		}
		http.Redirect(w, r, result.DestinationURL, http.StatusFound)
	}
}

type serveAdResponse struct {
	AdID       uuid.UUID          `json:"ad_id"`
	Placement  enums.AdPlacement  `json:"placement"`
	TargetType enums.AdTargetType `json:"target_type"`
	TargetID   uuid.UUID          `json:"target_id"`
	Creative   ads.AdCreativeDTO  `json:"creative"`
	RequestID  string             `json:"request_id"`
	ViewToken  string             `json:"view_token"`
	ClickToken string             `json:"click_token"`
}

type impressionRequest struct {
	ViewToken string `json:"view_token" validate:"required"`
	RequestID string `json:"request_id" validate:"required"`
}

func buyerStoreIDFromRequest(r *http.Request) (uuid.UUID, error) {
	storeType, ok := middleware.StoreTypeFromContext(r.Context())
	if !ok || storeType != enums.StoreTypeBuyer {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "buyer store context required")
	}
	storeID := middleware.StoreIDFromContext(r.Context())
	if storeID == "" {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}
	parsed, err := uuid.Parse(storeID)
	if err != nil {
		return uuid.Nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id")
	}
	return parsed, nil
}
