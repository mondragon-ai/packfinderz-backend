package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	squarewebhook "github.com/angelmondragon/packfinderz-backend/internal/webhooks/square"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type SquareWebhookService interface {
	HandleEvent(ctx context.Context, event *squarewebhook.SquareWebhookEvent) error
}

type squareWebhookGuard interface {
	CheckAndMark(ctx context.Context, eventID string) (bool, error)
	Delete(ctx context.Context, eventID string) error
}

type squareClient interface {
	SigningSecret() string
}

// SquareWebhook handles Square subscription lifecycle events.
func SquareWebhook(svc SquareWebhookService, client squareClient, guard squareWebhookGuard, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "webhook service unavailable"))
			return
		}
		if client == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "square client unavailable"))
			return
		}
		if guard == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "idempotency guard unavailable"))
			return
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "read request body"))
			return
		}

		sigHeader := r.Header.Get("Square-Signature")
		if sigHeader == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "square signature missing"))
			return
		}

		if !validateSquareSignature(payload, client.SigningSecret(), sigHeader) {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeDependency, "invalid square signature"))
			return
		}

		var event squarewebhook.SquareWebhookEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "decode event"))
			return
		}

		eventID := strings.TrimSpace(event.EventID)
		if eventID == "" {
			eventID = event.Data.ID
		}

		alreadyProcessed, err := guard.CheckAndMark(ctx, eventID)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check idempotency"))
			return
		}
		if alreadyProcessed {
			responses.WriteSuccess(w, nil)
			return
		}

		if err := svc.HandleEvent(ctx, &event); err != nil {
			_ = guard.Delete(ctx, eventID)
			responses.WriteError(ctx, logg, w, err)
			return
		}

		if logg != nil {
			logg.Info(ctx, fmt.Sprintf("square event %s processed", eventID))
		}
		responses.WriteSuccess(w, nil)
	}
}

func validateSquareSignature(payload []byte, secret, header string) bool {
	if header == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(header))
}
