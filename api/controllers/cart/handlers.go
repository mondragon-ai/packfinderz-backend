package cart

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/cart/dto"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// CartUpsert handles upsert of the buyer's active cart.
func CartUpsert(svc cartsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "cart service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromContext(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var payload cartdto.QuoteCartRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if payload.BuyerStoreID != uuid.Nil && payload.BuyerStoreID != buyerStoreID {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id mismatch"))
			return
		}
		payload.BuyerStoreID = buyerStoreID

		input := toQuoteCartInput(payload)

		record, err := svc.QuoteCart(r.Context(), buyerStoreID, input)
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
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "cart service unavailable"))
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
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}
	storeID := middleware.StoreIDFromContext(r.Context())
	if storeID == "" {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}
	buyerStoreID, err := uuid.Parse(storeID)
	if err != nil {
		return uuid.Nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid buyer store id")
	}
	return buyerStoreID, nil
}
