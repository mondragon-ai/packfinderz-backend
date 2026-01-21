package controllers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type mediaPresignRequest struct {
	MediaKind string `json:"media_kind" validate:"required"`
	MimeType  string `json:"mime_type" validate:"required"`
	FileName  string `json:"file_name" validate:"required"`
	SizeBytes int64  `json:"size_bytes" validate:"required,min=1"`
}

func (r mediaPresignRequest) toInput() (media.PresignInput, error) {
	kind, err := enums.ParseMediaKind(strings.TrimSpace(r.MediaKind))
	if err != nil {
		return media.PresignInput{}, pkgerrors.New(pkgerrors.CodeValidation, "invalid media_kind")
	}
	return media.PresignInput{
		Kind:      kind,
		MimeType:  r.MimeType,
		FileName:  r.FileName,
		SizeBytes: r.SizeBytes,
	}, nil
}

// MediaPresign handles creating a media record and returning a signed PUT URL.
func MediaPresign(svc media.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "media service unavailable"))
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

		var payload mediaPresignRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toInput()
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		resp, err := svc.PresignUpload(r.Context(), uid, sid, input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, resp)
	}
}
