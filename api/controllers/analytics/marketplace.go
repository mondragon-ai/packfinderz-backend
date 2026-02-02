package analytics

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func MarketplaceAnalytics(service analytics.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		storeID := middleware.StoreIDFromContext(ctx)
		if storeID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context required"))
			return
		}

		storeType, ok := middleware.StoreTypeFromContext(ctx)
		if !ok || !storeType.IsValid() {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context required"))
			return
		}

		start, end, err := resolveAnalyticsRange(r, timeNowUTC())
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		req := types.MarketplaceQueryRequest{
			StoreID:   storeID,
			StoreType: storeType,
			Start:     start,
			End:       end,
		}

		result, err := service.Query(ctx, req)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, result)
	}
}
