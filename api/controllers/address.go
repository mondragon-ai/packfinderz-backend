package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/internal/address"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type resolveAddressPayload struct {
	PlaceID string `json:"place_id"`
}

// AddressSuggest returns autocomplete suggestions for the frontend.
func AddressSuggest(svc address.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "address service unavailable"))
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("query"))
		country := strings.TrimSpace(r.URL.Query().Get("country"))
		language := strings.TrimSpace(r.URL.Query().Get("language"))

		req := address.SuggestRequest{
			Query:    query,
			Country:  country,
			Language: language,
		}

		resp, err := svc.Suggest(ctx, req)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, map[string]any{"suggestions": resp})
	}
}

// AddressResolve resolves a place ID into a canonical address.
func AddressResolve(svc address.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "address service unavailable"))
			return
		}

		var payload resolveAddressPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid json payload"))
			return
		}

		addr, err := svc.Resolve(ctx, address.ResolveRequest{
			PlaceID: payload.PlaceID,
		})
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, addr)
	}
}
