package subscriptions

import (
	"context"

	pkgstripe "github.com/angelmondragon/packfinderz-backend/pkg/stripe"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/subscription"
)

// StripeSubscriptionClient exposes the subset of Stripe operations required by the subscription service.
type StripeSubscriptionClient interface {
	Create(ctx context.Context, params *stripe.SubscriptionParams) (*stripe.Subscription, error)
	Cancel(ctx context.Context, id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error)
}

type stripeClientWrapper struct{}

// NewStripeClient wraps the provided Stripe client so the subscription service can be tested.
func NewStripeClient(api *pkgstripe.Client) StripeSubscriptionClient {
	if api == nil {
		return nil
	}
	return &stripeClientWrapper{}
}

func (w *stripeClientWrapper) Create(ctx context.Context, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	if params != nil {
		params.Context = ctx
	}
	return subscription.New(params)
}

func (w *stripeClientWrapper) Cancel(ctx context.Context, id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error) {
	if params != nil {
		params.Context = ctx
	}
	return subscription.Cancel(id, params)
}
