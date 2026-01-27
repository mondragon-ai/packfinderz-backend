package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// ListNotifications returns paginated notifications for the active store.
func ListNotifications(svc notifications.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "notifications service unavailable"))
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

		params := notifications.ListParams{StoreID: sid}

		if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
			value, err := strconv.Atoi(limitStr)
			if err != nil || value <= 0 {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "limit must be a positive integer"))
				return
			}
			params.Limit = value
		}

		if cursor := strings.TrimSpace(r.URL.Query().Get("cursor")); cursor != "" {
			params.Cursor = cursor
		}

		if unread := strings.TrimSpace(r.URL.Query().Get("unreadOnly")); unread != "" {
			value, err := strconv.ParseBool(unread)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid unreadOnly value"))
				return
			}
			params.UnreadOnly = value
		}

		resp, err := svc.List(r.Context(), params)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		responses.WriteSuccess(w, resp)
	}
}

// MarkNotificationRead acknowledges a single notification.
func MarkNotificationRead(svc notifications.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "notifications service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		sid, parseErr := uuid.Parse(storeID)
		if parseErr != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, parseErr, "invalid store id"))
			return
		}

		raw := strings.TrimSpace(chi.URLParam(r, "notificationId"))
		if raw == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "notification id required"))
			return
		}

		nID, err := uuid.Parse(raw)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid notification id"))
			return
		}

		if err := svc.MarkRead(r.Context(), sid, nID); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		responses.WriteSuccess(w, map[string]bool{"read": true})
	}
}

// MarkAllNotificationsRead marks every unread notification for the active store.
func MarkAllNotificationsRead(svc notifications.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "notifications service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		sid, parseErr := uuid.Parse(storeID)
		if parseErr != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, parseErr, "invalid store id"))
			return
		}

		count, err := svc.MarkAllRead(r.Context(), sid)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		responses.WriteSuccess(w, map[string]int64{"updated": count})
	}
}
