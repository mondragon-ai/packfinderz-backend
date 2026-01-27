package controllers

import (
	"net/http"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	internalorders "github.com/angelmondragon/packfinderz-backend/internal/orders"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
)

// AgentOrderQueue returns the paginated list of unassigned hold orders for agents.
func AgentOrderQueue(repo internalorders.Repository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
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

		list, err := repo.ListUnassignedHoldOrders(r.Context(), params)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list agent queue"))
			return
		}

		responses.WriteSuccess(w, list)
	}
}
