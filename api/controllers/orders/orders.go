package orders

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	internalorders "github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
)

// List returns buyer- or vendor-perspective order pages depending on the active store type.
func List(repo internalorders.Repository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store type missing"))
			return
		}

		limit, err := validators.ParseQueryInt(r, "limit", pagination.DefaultLimit, 1, pagination.MaxLimit)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))

		params := pagination.Params{
			Limit:  limit,
			Cursor: cursor,
		}

		switch storeType {
		case enums.StoreTypeBuyer:
			filters, err := buildBuyerFilters(r)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, err)
				return
			}
			list, err := repo.ListBuyerOrders(r.Context(), storeID, params, filters)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list buyer orders"))
				return
			}
			responses.WriteSuccess(w, list)
			return
		case enums.StoreTypeVendor:
			filters, err := buildVendorFilters(r)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, err)
				return
			}
			list, err := repo.ListVendorOrders(r.Context(), storeID, params, filters)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list vendor orders"))
				return
			}
			responses.WriteSuccess(w, list)
			return
		default:
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "unsupported store type"))
			return
		}
	}
}

// Detail returns the full order detail after ensuring the active store owns the order.
func Detail(repo internalorders.Repository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store type missing"))
			return
		}

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		detail, err := repo.FindOrderDetail(r.Context(), orderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "order not found"))
				return
			}
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "fetch order detail"))
			return
		}

		switch storeType {
		case enums.StoreTypeBuyer:
			if detail.BuyerStore.ID != storeID {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store"))
				return
			}
		case enums.StoreTypeVendor:
			if detail.VendorStore.ID != storeID {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store"))
				return
			}
		default:
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "unsupported store type"))
			return
		}

		responses.WriteSuccess(w, detail)
	}
}

func VendorOrderDecision(svc internalorders.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders service unavailable"))
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok || storeType != enums.StoreTypeVendor {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "vendor store context required"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		actorID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		role := middleware.RoleFromContext(r.Context())

		var payload vendorOrderDecisionRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		decision, err := parseVendorOrderDecision(payload.Decision)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		input := internalorders.VendorDecisionInput{
			OrderID:      orderID,
			Decision:     decision,
			ActorUserID:  actorID,
			ActorStoreID: storeID,
			ActorRole:    role,
		}

		if err := svc.VendorDecision(r.Context(), input); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, nil)
	}
}

func CancelOrder(svc internalorders.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders service unavailable"))
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok || storeType != enums.StoreTypeBuyer {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "buyer store context required"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		actorID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		role := middleware.RoleFromContext(r.Context())

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		input := internalorders.BuyerCancelInput{
			OrderID:      orderID,
			ActorUserID:  actorID,
			ActorStoreID: storeID,
			ActorRole:    role,
		}

		if err := svc.CancelOrder(r.Context(), input); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		responses.WriteSuccess(w, nil)
	}
}

func NudgeVendor(svc internalorders.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders service unavailable"))
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok || storeType != enums.StoreTypeBuyer {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "buyer store context required"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		actorID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		role := middleware.RoleFromContext(r.Context())

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		input := internalorders.BuyerNudgeInput{
			OrderID:      orderID,
			ActorUserID:  actorID,
			ActorStoreID: storeID,
			ActorRole:    role,
		}

		if err := svc.NudgeVendor(r.Context(), input); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		responses.WriteSuccessStatus(w, http.StatusAccepted, nil)
	}
}

func RetryOrder(svc internalorders.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders service unavailable"))
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok || storeType != enums.StoreTypeBuyer {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "buyer store context required"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		actorID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		role := middleware.RoleFromContext(r.Context())

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		input := internalorders.BuyerRetryInput{
			OrderID:      orderID,
			ActorUserID:  actorID,
			ActorStoreID: storeID,
			ActorRole:    role,
		}

		output, err := svc.RetryOrder(r.Context(), input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		responses.WriteSuccessStatus(w, http.StatusCreated, output)
	}
}

func VendorLineItemDecision(svc internalorders.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders service unavailable"))
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(r.Context())
		if !ok || storeType != enums.StoreTypeVendor {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "vendor store context required"))
			return
		}

		storeID, err := parseStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		actorID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		role := middleware.RoleFromContext(r.Context())

		var payload vendorLineItemDecisionRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		lineItemID, err := uuid.Parse(strings.TrimSpace(payload.LineItemID))
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid line item id"))
			return
		}

		decision, err := parseVendorLineItemDecision(payload.Decision)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		input := internalorders.LineItemDecisionInput{
			OrderID:      orderID,
			LineItemID:   lineItemID,
			Decision:     decision,
			Notes:        payload.Notes,
			ActorUserID:  actorID,
			ActorStoreID: storeID,
			ActorRole:    role,
		}

		if err := svc.LineItemDecision(r.Context(), input); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, nil)
	}
}

type vendorOrderDecisionRequest struct {
	Decision string `json:"decision" validate:"required"`
}

func parseVendorOrderDecision(raw string) (enums.VendorOrderDecision, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "accept":
		return enums.VendorOrderDecisionAccept, nil
	case "reject":
		return enums.VendorOrderDecisionReject, nil
	default:
		return "", pkgerrors.New(pkgerrors.CodeValidation, "decision must be accept or reject")
	}
}

type vendorLineItemDecisionRequest struct {
	LineItemID string  `json:"line_item_id" validate:"required,uuid4"`
	Decision   string  `json:"decision" validate:"required"`
	Notes      *string `json:"notes,omitempty"`
}

func parseVendorLineItemDecision(raw string) (internalorders.LineItemDecision, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fulfill":
		return internalorders.LineItemDecisionFulfill, nil
	case "reject":
		return internalorders.LineItemDecisionReject, nil
	default:
		return "", pkgerrors.New(pkgerrors.CodeValidation, "decision must be fulfill or reject")
	}
}

func parseStoreID(r *http.Request) (uuid.UUID, error) {
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

func buildBuyerFilters(r *http.Request) (internalorders.BuyerOrderFilters, error) {
	var filters internalorders.BuyerOrderFilters
	orderStatus, err := parseVendorOrderStatusParam(r.URL.Query().Get("order_status"))
	if err != nil {
		return filters, err
	}
	filters.OrderStatus = orderStatus

	fulfillmentStatus, err := parseFulfillmentStatusParam(r.URL.Query().Get("fulfillment_status"))
	if err != nil {
		return filters, err
	}
	filters.FulfillmentStatus = fulfillmentStatus

	shippingStatus, err := parseShippingStatusParam(r.URL.Query().Get("shipping_status"))
	if err != nil {
		return filters, err
	}
	filters.ShippingStatus = shippingStatus

	paymentStatus, err := parsePaymentStatusParam(r.URL.Query().Get("payment_status"))
	if err != nil {
		return filters, err
	}
	filters.PaymentStatus = paymentStatus

	dateFrom, err := parseDateParam(r.URL.Query().Get("date_from"), "date_from")
	if err != nil {
		return filters, err
	}
	filters.DateFrom = dateFrom

	dateTo, err := parseDateParam(r.URL.Query().Get("date_to"), "date_to")
	if err != nil {
		return filters, err
	}
	filters.DateTo = dateTo

	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		filters.Query = q
	}

	return filters, nil
}

func buildVendorFilters(r *http.Request) (internalorders.VendorOrderFilters, error) {
	var filters internalorders.VendorOrderFilters
	orderStatus, err := parseVendorOrderStatusParam(r.URL.Query().Get("order_status"))
	if err != nil {
		return filters, err
	}
	filters.OrderStatus = orderStatus

	fulfillmentStatus, err := parseFulfillmentStatusParam(r.URL.Query().Get("fulfillment_status"))
	if err != nil {
		return filters, err
	}
	filters.FulfillmentStatus = fulfillmentStatus

	shippingStatus, err := parseShippingStatusParam(r.URL.Query().Get("shipping_status"))
	if err != nil {
		return filters, err
	}
	filters.ShippingStatus = shippingStatus

	paymentStatus, err := parsePaymentStatusParam(r.URL.Query().Get("payment_status"))
	if err != nil {
		return filters, err
	}
	filters.PaymentStatus = paymentStatus

	dateFrom, err := parseDateParam(r.URL.Query().Get("date_from"), "date_from")
	if err != nil {
		return filters, err
	}
	filters.DateFrom = dateFrom

	dateTo, err := parseDateParam(r.URL.Query().Get("date_to"), "date_to")
	if err != nil {
		return filters, err
	}
	filters.DateTo = dateTo

	actionable, err := parseActionableStatuses(r)
	if err != nil {
		return filters, err
	}
	if len(actionable) > 0 {
		filters.ActionableStatuses = actionable
	}

	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		filters.Query = q
	}

	return filters, nil
}

func parseVendorOrderStatusParam(raw string) (*enums.VendorOrderStatus, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	status, err := enums.ParseVendorOrderStatus(raw)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, fmt.Sprintf("invalid order_status %q", raw))
	}
	return &status, nil
}

func parseFulfillmentStatusParam(raw string) (*enums.VendorOrderFulfillmentStatus, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	status, err := enums.ParseVendorOrderFulfillmentStatus(raw)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, fmt.Sprintf("invalid fulfillment_status %q", raw))
	}
	return &status, nil
}

func parseShippingStatusParam(raw string) (*enums.VendorOrderShippingStatus, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	status, err := enums.ParseVendorOrderShippingStatus(raw)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, fmt.Sprintf("invalid shipping_status %q", raw))
	}
	return &status, nil
}

func parsePaymentStatusParam(raw string) (*enums.PaymentStatus, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	status, err := enums.ParsePaymentStatus(raw)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, fmt.Sprintf("invalid payment_status %q", raw))
	}
	return &status, nil
}

func parseDateParam(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, fmt.Sprintf("invalid %s", field))
		}
	}
	return &t, nil
}

func parseActionableStatuses(r *http.Request) ([]enums.VendorOrderStatus, error) {
	var statuses []enums.VendorOrderStatus
	for _, raw := range r.URL.Query()["actionable_statuses"] {
		for _, token := range strings.Split(raw, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			status, err := enums.ParseVendorOrderStatus(token)
			if err != nil {
				return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, fmt.Sprintf("invalid actionable_statuses %q", token))
			}
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}
