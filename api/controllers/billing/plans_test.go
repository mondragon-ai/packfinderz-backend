package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	billingsvc "github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type stubBillingPlanService struct {
	created    *models.BillingPlan
	updated    *models.BillingPlan
	listParams billingsvc.ListBillingPlansParams
	plans      []models.BillingPlan
	found      *models.BillingPlan
}

func (s *stubBillingPlanService) CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	s.created = plan
	return nil
}

func (s *stubBillingPlanService) UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	s.updated = plan
	return nil
}

func (s *stubBillingPlanService) ListBillingPlans(ctx context.Context, params billingsvc.ListBillingPlansParams) ([]models.BillingPlan, error) {
	s.listParams = params
	return s.plans, nil
}

func (s *stubBillingPlanService) FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error) {
	return s.found, nil
}

func TestVendorBillingPlansListRequiresVendorContext(t *testing.T) {
	handler := VendorBillingPlansList(&stubBillingPlanService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/billing/plans", nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when vendor context missing, got %d", resp.Code)
	}
}

func TestVendorBillingPlansListFiltersActive(t *testing.T) {
	service := &stubBillingPlanService{
		plans: []models.BillingPlan{
			{
				ID:                  "starter_v1",
				Name:                "Starter",
				Status:              enums.PlanStatusActive,
				SquareBillingPlanID: "square-plan-1",
				Interval:            enums.BillingIntervalEvery30Days,
				PriceAmount:         decimal.NewFromInt(1500).Shift(-2),
				CurrencyCode:        "usd",
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/billing/plans", nil)
	ctx := middleware.WithStoreID(req.Context(), uuid.New().String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	handler := VendorBillingPlansList(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	if service.listParams.Status == nil || *service.listParams.Status != enums.PlanStatusActive {
		t.Fatalf("expected active status filter, got %v", service.listParams.Status)
	}

	var envelope struct {
		Data billingPlanListResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(envelope.Data.Plans))
	}
}

func TestVendorBillingPlanDetailRequiresActiveStatus(t *testing.T) {
	service := &stubBillingPlanService{
		found: &models.BillingPlan{
			ID:     "disabled",
			Status: enums.PlanStatusHidden,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/billing/plans/disabled", nil)
	ctx := middleware.WithStoreID(req.Context(), uuid.New().String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("planId", "disabled")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
	req = req.WithContext(ctx)

	handler := VendorBillingPlanDetail(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for hidden plan, got %d", resp.Code)
	}
}

func TestAdminBillingPlanCreateParsesPayload(t *testing.T) {
	service := &stubBillingPlanService{}
	payload := `{
		"id":"starter_v1",
		"name":"Starter",
		"status":"active",
		"square_billing_plan_id":"square-plan-1",
		"interval":"EVERY_30_DAYS",
		"price_amount_cents":1500,
		"currency_code":"usd",
		"test":true,
		"is_default":true,
		"trial_days":7,
		"trial_require_payment_method":true,
		"trial_start_on_activation":false,
		"features":["feature-a"],
		"ui":{"badge":"popular"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/billing/plans", strings.NewReader(payload))
	resp := httptest.NewRecorder()
	handler := AdminBillingPlanCreate(service, nil)
	handler(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if service.created == nil {
		t.Fatal("expected plan creation")
	}
	if service.created.Name != "Starter" {
		t.Fatalf("unexpected name %s", service.created.Name)
	}
	if service.created.Status != enums.PlanStatusActive {
		t.Fatalf("unexpected status %s", service.created.Status)
	}
	if !service.created.IsDefault {
		t.Fatal("expected is_default flag set")
	}
	if !service.created.Test {
		t.Fatal("expected test flag set")
	}
	if service.created.TrialDays != 7 {
		t.Fatalf("unexpected trial days %d", service.created.TrialDays)
	}
	if !service.created.TrialRequirePaymentMethod {
		t.Fatal("expected trial require payment method")
	}
	if service.created.TrialStartOnActivation {
		t.Fatal("expected trial start on activation false")
	}
	if len(service.created.Features) != 1 || service.created.Features[0] != "feature-a" {
		t.Fatalf("unexpected features %v", service.created.Features)
	}
	if service.created.PriceAmount.StringFixed(2) != decimal.NewFromInt(1500).Shift(-2).StringFixed(2) {
		t.Fatalf("unexpected price %s", service.created.PriceAmount)
	}
}

func TestAdminBillingPlansListParsesFilters(t *testing.T) {
	service := &stubBillingPlanService{
		plans: []models.BillingPlan{
			{ID: "starter_v1", Status: enums.PlanStatusActive},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/billing/plans?status=active&is_default=true", nil)
	resp := httptest.NewRecorder()
	handler := AdminBillingPlansList(service, nil)
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if service.listParams.Status == nil || *service.listParams.Status != enums.PlanStatusActive {
		t.Fatalf("status filter missing: %v", service.listParams.Status)
	}
	if service.listParams.IsDefault == nil || !*service.listParams.IsDefault {
		t.Fatalf("is_default filter missing or false")
	}
}

func TestAdminBillingPlanDeleteHidesPlan(t *testing.T) {
	plan := &models.BillingPlan{
		ID:     "starter_v1",
		Status: enums.PlanStatusActive,
	}
	service := &stubBillingPlanService{
		found: plan,
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/v1/billing/plans/starter_v1", nil)
	resp := httptest.NewRecorder()
	handler := AdminBillingPlanDelete(service, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("planId", "starter_v1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if service.updated == nil {
		t.Fatal("expected plan update")
	}
	if service.updated.Status != enums.PlanStatusHidden {
		t.Fatalf("expected hidden status, got %s", service.updated.Status)
	}
	if service.updated.IsDefault {
		t.Fatal("expected is_default to be false")
	}
}
