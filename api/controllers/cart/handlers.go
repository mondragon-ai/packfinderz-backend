package cart

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// CartUpsert handles upsert of the buyer's active cart.
func CartUpsert(svc cartsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeInternal, "cart service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromContext(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
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

		responses.WriteSuccess(w, newCartQuote(record))
	}
}

// CartFetch exposes the active cart record for the buyer store.
func CartFetch(svc cartsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeInternal, "cart service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromContext(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		record, err := svc.GetActiveCart(r.Context(), buyerStoreID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, newCartQuote(record))
	}
}

func buyerStoreIDFromContext(r *http.Request) (uuid.UUID, error) {
	if r == nil {
		return uuid.Nil, errors.New(errors.CodeForbidden, "store context missing")
	}
	storeID := middleware.StoreIDFromContext(r.Context())
	if storeID == "" {
		return uuid.Nil, errors.New(errors.CodeForbidden, "store context missing")
	}
	buyerStoreID, err := uuid.Parse(storeID)
	if err != nil {
		return uuid.Nil, errors.Wrap(errors.CodeValidation, err, "invalid buyer store id")
	}
	return buyerStoreID, nil
}
