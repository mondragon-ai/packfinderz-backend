package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	billingsvc "github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// BillingPlanService describes the billing plan methods used by the HTTP controllers.
type BillingPlanService interface {
	CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error
	UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error
	ListBillingPlans(ctx context.Context, params billingsvc.ListBillingPlansParams) ([]models.BillingPlan, error)
	FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error)
}

type billingPlanResponse struct {
	ID                        string          `json:"id"`
	Name                      string          `json:"name"`
	Status                    string          `json:"status"`
	SquareBillingPlanID       string          `json:"square_billing_plan_id"`
	Test                      bool            `json:"test"`
	IsDefault                 bool            `json:"is_default"`
	TrialDays                 int             `json:"trial_days"`
	TrialRequirePaymentMethod bool            `json:"trial_require_payment_method"`
	TrialStartOnActivation    bool            `json:"trial_start_on_activation"`
	Interval                  string          `json:"interval"`
	PriceAmount               string          `json:"price_amount"`
	PriceAmountCents          int64           `json:"price_amount_cents"`
	CurrencyCode              string          `json:"currency_code"`
	Features                  []string        `json:"features"`
	UI                        json.RawMessage `json:"ui,omitempty"`
	CreatedAt                 string          `json:"created_at"`
	UpdatedAt                 string          `json:"updated_at"`
}

type billingPlanListResponse struct {
	Plans []billingPlanResponse `json:"plans"`
}

type billingPlanUpsertRequest struct {
	ID                        string          `json:"id,omitempty"`
	Name                      string          `json:"name"`
	Status                    string          `json:"status"`
	SquareBillingPlanID       string          `json:"square_billing_plan_id"`
	Test                      *bool           `json:"test"`
	IsDefault                 *bool           `json:"is_default"`
	TrialDays                 *int            `json:"trial_days"`
	TrialRequirePaymentMethod *bool           `json:"trial_require_payment_method"`
	TrialStartOnActivation    *bool           `json:"trial_start_on_activation"`
	Interval                  string          `json:"interval"`
	PriceAmountCents          *int64          `json:"price_amount_cents"`
	CurrencyCode              string          `json:"currency_code"`
	Features                  []string        `json:"features"`
	UI                        json.RawMessage `json:"ui"`
}

func AdminBillingPlansList(svc BillingPlanService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing plan service unavailable"))
			return
		}

		statusParam := strings.TrimSpace(r.URL.Query().Get("status"))
		var status *enums.PlanStatus
		if statusParam != "" {
			parsed, err := enums.ParsePlanStatus(statusParam)
			if err != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid status"))
				return
			}
			status = &parsed
		}

		isDefaultParam := strings.TrimSpace(r.URL.Query().Get("is_default"))
		var isDefault *bool
		if isDefaultParam != "" {
			parsed, err := strconv.ParseBool(isDefaultParam)
			if err != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid is_default flag"))
				return
			}
			isDefault = &parsed
		}

		plans, err := svc.ListBillingPlans(ctx, billingsvc.ListBillingPlansParams{
			Status:    status,
			IsDefault: isDefault,
		})
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		response := billingPlanListResponse{
			Plans: plansToResponse(plans),
		}
		responses.WriteSuccess(w, response)
	}
}

func AdminBillingPlanCreate(svc BillingPlanService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing plan service unavailable"))
			return
		}

		var payload billingPlanUpsertRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		if strings.TrimSpace(payload.ID) == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "id is required"))
			return
		}

		plan, err := buildBillingPlanFromRequest(payload, strings.TrimSpace(payload.ID))
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		if err := svc.CreateBillingPlan(ctx, plan); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, billingPlanToResponse(plan))
	}
}

func AdminBillingPlanUpdate(svc BillingPlanService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing plan service unavailable"))
			return
		}

		planID := strings.TrimSpace(chi.URLParam(r, "planId"))
		if planID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "plan id is required"))
			return
		}

		existing, err := svc.FindBillingPlanByID(ctx, planID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		if existing == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "billing plan not found"))
			return
		}

		var payload billingPlanUpsertRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		plan, err := buildBillingPlanFromRequest(payload, planID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		plan.CreatedAt = existing.CreatedAt
		if err := svc.UpdateBillingPlan(ctx, plan); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		updatedPlan, err := svc.FindBillingPlanByID(ctx, plan.ID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		if updatedPlan == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "billing plan not found"))
			return
		}

		responses.WriteSuccess(w, billingPlanToResponse(updatedPlan))
	}
}

func AdminBillingPlanDelete(svc BillingPlanService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing plan service unavailable"))
			return
		}

		planID := strings.TrimSpace(chi.URLParam(r, "planId"))
		if planID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "plan id is required"))
			return
		}

		plan, err := svc.FindBillingPlanByID(ctx, planID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		if plan == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "billing plan not found"))
			return
		}

		plan.Status = enums.PlanStatusHidden
		plan.IsDefault = false
		if err := svc.UpdateBillingPlan(ctx, plan); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		updatedPlan, err := svc.FindBillingPlanByID(ctx, plan.ID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		if updatedPlan == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "billing plan not found"))
			return
		}

		responses.WriteSuccess(w, billingPlanToResponse(updatedPlan))
	}
}

func VendorBillingPlansList(svc BillingPlanService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing plan service unavailable"))
			return
		}

		_, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		active := enums.PlanStatusActive
		plans, err := svc.ListBillingPlans(ctx, billingsvc.ListBillingPlansParams{
			Status: &active,
		})
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, billingPlanListResponse{Plans: plansToResponse(plans)})
	}
}

func VendorBillingPlanDetail(svc BillingPlanService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing plan service unavailable"))
			return
		}

		_, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		planID := strings.TrimSpace(chi.URLParam(r, "planId"))
		if planID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "plan id is required"))
			return
		}

		plan, err := svc.FindBillingPlanByID(ctx, planID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		if plan == nil || plan.Status != enums.PlanStatusActive {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "billing plan not found"))
			return
		}

		responses.WriteSuccess(w, billingPlanToResponse(plan))
	}
}

func plansToResponse(plans []models.BillingPlan) []billingPlanResponse {
	result := make([]billingPlanResponse, 0, len(plans))
	for _, plan := range plans {
		result = append(result, billingPlanToResponse(&plan))
	}
	return result
}

func billingPlanToResponse(plan *models.BillingPlan) billingPlanResponse {
	priceAmount := plan.PriceAmount.StringFixed(2)
	priceCents := plan.PriceAmount.Shift(2).IntPart()

	features := make([]string, len(plan.Features))
	copy(features, plan.Features)

	return billingPlanResponse{
		ID:                        plan.ID,
		Name:                      plan.Name,
		Status:                    string(plan.Status),
		SquareBillingPlanID:       plan.SquareBillingPlanID,
		Test:                      plan.Test,
		IsDefault:                 plan.IsDefault,
		TrialDays:                 plan.TrialDays,
		TrialRequirePaymentMethod: plan.TrialRequirePaymentMethod,
		TrialStartOnActivation:    plan.TrialStartOnActivation,
		Interval:                  string(plan.Interval),
		PriceAmount:               priceAmount,
		PriceAmountCents:          priceCents,
		CurrencyCode:              plan.CurrencyCode,
		Features:                  features,
		UI:                        plan.UI,
		CreatedAt:                 plan.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                 plan.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func buildBillingPlanFromRequest(payload billingPlanUpsertRequest, id string) (*models.BillingPlan, error) {
	trimmedName := strings.TrimSpace(payload.Name)
	if trimmedName == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "name is required")
	}

	trimmedStatus := strings.TrimSpace(payload.Status)
	if trimmedStatus == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "status is required")
	}
	status, err := enums.ParsePlanStatus(trimmedStatus)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid status")
	}

	trimmedSquareID := strings.TrimSpace(payload.SquareBillingPlanID)
	if trimmedSquareID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "square_billing_plan_id is required")
	}

	trimmedInterval := strings.TrimSpace(payload.Interval)
	if trimmedInterval == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "interval is required")
	}
	interval, err := enums.ParseBillingInterval(trimmedInterval)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid interval")
	}

	if payload.PriceAmountCents == nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "price_amount_cents is required")
	}
	if *payload.PriceAmountCents < 0 {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "price amount must be non-negative")
	}
	priceAmount := decimal.NewFromInt(*payload.PriceAmountCents).Shift(-2)

	trimmedCurrency := strings.TrimSpace(payload.CurrencyCode)
	if trimmedCurrency == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "currency_code is required")
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "id is required")
	}

	return &models.BillingPlan{
		ID:                        trimmedID,
		Name:                      trimmedName,
		Status:                    status,
		SquareBillingPlanID:       trimmedSquareID,
		Test:                      boolValue(payload.Test, false),
		IsDefault:                 boolValue(payload.IsDefault, false),
		TrialDays:                 intValue(payload.TrialDays, 0),
		TrialRequirePaymentMethod: boolValue(payload.TrialRequirePaymentMethod, false),
		TrialStartOnActivation:    boolValue(payload.TrialStartOnActivation, true),
		Interval:                  interval,
		PriceAmount:               priceAmount,
		CurrencyCode:              trimmedCurrency,
		Features:                  pq.StringArray(payload.Features),
		UI:                        payload.UI,
	}, nil
}

func boolValue(ptr *bool, fallback bool) bool {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func intValue(ptr *int, fallback int) int {
	if ptr == nil {
		return fallback
	}
	return *ptr
}
