package webhooks

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"
)

type StripeWebhookService interface {
	HandleEvent(ctx context.Context, event *stripe.Event) error
}

type stripeWebhookGuard interface {
	CheckAndMark(ctx context.Context, eventID string) (bool, error)
	Delete(ctx context.Context, eventID string) error
}

type stripeClient interface {
	SigningSecret() string
}

// StripeWebhook handles Stripe subscription lifecycle events.
func StripeWebhook(svc StripeWebhookService, client stripeClient, guard stripeWebhookGuard, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "webhook service unavailable"))
			return
		}
		if client == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "stripe client unavailable"))
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

		sigHeader := r.Header.Get("Stripe-Signature")
		if sigHeader == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "stripe signature missing"))
			return
		}

		event, err := webhook.ConstructEvent(payload, sigHeader, client.SigningSecret())
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "verify signature"))
			return
		}

		alreadyProcessed, err := guard.CheckAndMark(ctx, event.ID)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check idempotency"))
			return
		}
		if alreadyProcessed {
			responses.WriteSuccess(w, nil)
			return
		}

		if err := svc.HandleEvent(ctx, &event); err != nil {
			_ = guard.Delete(ctx, event.ID)
			responses.WriteError(ctx, logg, w, err)
			return
		}

		if logg != nil {
			logg.Info(ctx, fmt.Sprintf("stripe event %s processed", event.ID))
		}
		responses.WriteSuccess(w, nil)
	}
}
