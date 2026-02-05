package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/squarecustomers"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

type adminSquareCustomerRequest struct {
	StoreID     uuid.UUID     `json:"store_id" validate:"required"`
	FirstName   string        `json:"first_name" validate:"required"`
	LastName    string        `json:"last_name" validate:"required"`
	Email       string        `json:"email" validate:"required,email"`
	Phone       *string       `json:"phone,omitempty"`
	CompanyName string        `json:"company_name" validate:"required"`
	Address     types.Address `json:"address" validate:"required"`
}

// AdminSquareCustomerEnsure creates or reuses a Square customer and persists the identifier on the store.
func AdminSquareCustomerEnsure(service squarecustomers.Service, store stores.SquareCustomerUpdater, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil || store == nil {
			err := pkgerrors.New(pkgerrors.CodeInternal, "square customer handler unavailable")
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var req adminSquareCustomerRequest
		if err := validators.DecodeJSONBody(r, &req); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		customerID, err := service.EnsureCustomer(r.Context(), squarecustomers.Input{
			ReferenceID: "",
			FirstName:   req.FirstName,
			LastName:    req.LastName,
			Email:       req.Email,
			Phone:       req.Phone,
			CompanyName: req.CompanyName,
			Address:     req.Address,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if err := store.UpdateSquareCustomerID(r.Context(), req.StoreID, &customerID); err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist square customer id"))
			return
		}

		responses.WriteSuccess(w, map[string]string{"square_customer_id": customerID})
	}
}
