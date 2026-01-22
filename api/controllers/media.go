package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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

// MediaDelete deletes a media row if it belongs to the active store and is unreferenced.
func MediaDelete(svc media.Service, logg *logger.Logger) http.HandlerFunc {
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

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		mediaIDParam := chi.URLParam(r, "mediaId")
		mediaID, err := uuid.Parse(strings.TrimSpace(mediaIDParam))
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid media id"))
			return
		}

		err = svc.DeleteMedia(r.Context(), media.DeleteMediaParams{
			StoreID: sid,
			MediaID: mediaID,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// MediaList handles listing store-scoped media metadata.
func MediaList(svc media.Service, logg *logger.Logger) http.HandlerFunc {
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

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		q := r.URL.Query()
		params := media.ListParams{
			StoreID:  sid,
			MimeType: strings.TrimSpace(q.Get("mime_type")),
			Search:   strings.TrimSpace(q.Get("search")),
		}

		if limit := strings.TrimSpace(q.Get("limit")); limit != "" {
			value, err := strconv.Atoi(limit)
			if err != nil || value <= 0 {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "limit must be a positive integer"))
				return
			}
			params.Limit = value
		}

		if kind := strings.TrimSpace(q.Get("kind")); kind != "" {
			parsed, err := enums.ParseMediaKind(kind)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "invalid kind filter"))
				return
			}
			params.Kind = parsed
			params.HasKind = true
		}

		if status := strings.TrimSpace(q.Get("status")); status != "" {
			parsed, err := enums.ParseMediaStatus(status)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "invalid status filter"))
				return
			}
			params.Status = parsed
			params.HasStatus = true
		}

		resp, err := svc.ListMedia(r.Context(), params)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, resp)
	}
}
