package controllers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type sessionTokenRotator interface {
	Rotate(ctx context.Context, oldAccessID, provided string) (string, string, error)
	Revoke(ctx context.Context, accessID string) error
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func parseBearerToken(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		return "", errors.New(errors.CodeUnauthorized, "missing credentials")
	}
	token := raw
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return "", errors.New(errors.CodeUnauthorized, "missing credentials")
	}
	return token, nil
}

// AuthLogout revokes the refresh mapping tied to the presented access token.
func AuthLogout(manager sessionTokenRotator, cfg config.JWTConfig, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeInternal, "session manager unavailable"))
			return
		}

		token, err := parseBearerToken(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		claims, err := pkgAuth.ParseAccessTokenAllowExpired(cfg, token)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeUnauthorized, err, "invalid token"))
			return
		}

		if claims.ID == "" {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeUnauthorized, "missing session id"))
			return
		}

		if err := manager.Revoke(r.Context(), claims.ID); err != nil {
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeInternal, err, "revoke session"))
			return
		}

		responses.WriteSuccess(w, map[string]string{"status": "logged_out"})
	}
}

// AuthRefresh rotates the refresh token and issues a new access token.
func AuthRefresh(manager sessionTokenRotator, cfg config.JWTConfig, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeInternal, "session manager unavailable"))
			return
		}

		var body refreshRequest
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		token, err := parseBearerToken(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		claims, err := pkgAuth.ParseAccessTokenAllowExpired(cfg, token)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeUnauthorized, err, "invalid token"))
			return
		}

		if claims.ID == "" {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeUnauthorized, "missing session id"))
			return
		}

		newAccessID, newRefreshToken, err := manager.Rotate(r.Context(), claims.ID, body.RefreshToken)
		if err != nil {
			if err == session.ErrInvalidRefreshToken {
				responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeUnauthorized, "invalid refresh token"))
				return
			}
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeInternal, err, "rotate session"))
			return
		}

		payload := pkgAuth.AccessTokenPayload{
			UserID:        claims.UserID,
			ActiveStoreID: claims.ActiveStoreID,
			Role:          claims.Role,
			StoreType:     claims.StoreType,
			KYCStatus:     claims.KYCStatus,
			JTI:           newAccessID,
		}

		now := time.Now().UTC()
		accessToken, err := pkgAuth.MintAccessToken(cfg, now, payload)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeInternal, err, "mint jwt"))
			return
		}

		w.Header().Set("X-PF-Token", accessToken)
		responses.WriteSuccess(w, refreshResponse{
			AccessToken:  accessToken,
			RefreshToken: newRefreshToken,
		})
	}
}
