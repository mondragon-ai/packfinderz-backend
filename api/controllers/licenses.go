package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type licenseCreateRequest struct {
	MediaID        string     `json:"media_id" validate:"required"`
	IssuingState   string     `json:"issuing_state" validate:"required"`
	IssueDate      *time.Time `json:"issue_date"`
	ExpirationDate *time.Time `json:"expiration_date"`
	Type           string     `json:"type" validate:"required"`
	Number         string     `json:"number" validate:"required"`
}

func (r licenseCreateRequest) toInput() (licenses.CreateLicenseInput, error) {
	mediaID, err := uuid.Parse(strings.TrimSpace(r.MediaID))
	if err != nil {
		return licenses.CreateLicenseInput{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid media_id")
	}

	licenseType, err := enums.ParseLicenseType(strings.TrimSpace(r.Type))
	if err != nil {
		return licenses.CreateLicenseInput{}, pkgerrors.New(pkgerrors.CodeValidation, "invalid license type")
	}

	return licenses.CreateLicenseInput{
		MediaID:        mediaID,
		IssuingState:   strings.TrimSpace(r.IssuingState),
		IssueDate:      r.IssueDate,
		ExpirationDate: r.ExpirationDate,
		Type:           licenseType,
		Number:         strings.TrimSpace(r.Number),
	}, nil
}

// LicenseCreate handles creating store-scoped license metadata.
func LicenseCreate(svc licenses.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "license service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		var payload licenseCreateRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toInput()
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		created, err := svc.CreateLicense(r.Context(), uid, sid, input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, licenseResponseFromModel(created))
	}
}

type licenseResponse struct {
	ID             uuid.UUID           `json:"id"`
	StoreID        uuid.UUID           `json:"store_id"`
	UserID         uuid.UUID           `json:"user_id"`
	Status         enums.LicenseStatus `json:"status"`
	MediaID        uuid.UUID           `json:"media_id"`
	IssuingState   string              `json:"issuing_state"`
	IssueDate      *time.Time          `json:"issue_date"`
	ExpirationDate *time.Time          `json:"expiration_date"`
	Type           enums.LicenseType   `json:"type"`
	Number         string              `json:"number"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

func licenseResponseFromModel(m *models.License) licenseResponse {
	return licenseResponse{
		ID:             m.ID,
		StoreID:        m.StoreID,
		UserID:         m.UserID,
		Status:         m.Status,
		MediaID:        m.MediaID,
		IssuingState:   m.IssuingState,
		IssueDate:      m.IssueDate,
		ExpirationDate: m.ExpirationDate,
		Type:           m.Type,
		Number:         m.Number,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
