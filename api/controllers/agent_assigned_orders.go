package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	internalorders "github.com/angelmondragon/packfinderz-backend/internal/orders"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
)

// AgentAssignedOrders returns the paginated list of orders assigned to the agent.
func AgentAssignedOrders(repo internalorders.Repository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		agentID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
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

		list, err := repo.ListAssignedOrders(r.Context(), agentID, params)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list assigned orders"))
			return
		}
		responses.WriteSuccess(w, list)
	}
}

// AgentAssignedOrderDetail returns detailed info for an order assigned to the agent.
func AgentAssignedOrderDetail(repo internalorders.Repository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		agentID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
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

		order, err := repo.FindOrderDetail(r.Context(), orderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "order not found"))
				return
			}
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "fetch order detail"))
			return
		}

		if order.ActiveAssignment == nil || order.ActiveAssignment.AgentUserID != agentID {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "order not assigned to agent"))
			return
		}

		responses.WriteSuccess(w, order)
	}
}

func AgentPickupOrder(svc internalorders.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders service unavailable"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}
		agentID, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
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

		if err := svc.AgentPickup(r.Context(), internalorders.AgentPickupInput{
			OrderID:     orderID,
			AgentUserID: agentID,
		}); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, map[string]string{"status": "in_transit"})
	}
}
