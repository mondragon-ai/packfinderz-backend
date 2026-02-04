package subscriptions

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SquareSubscriptionClient defines the subset of Square interactions that our subscription service relies on.
type SquareSubscriptionClient interface {
	Create(ctx context.Context, params *SquareSubscriptionParams) (*SquareSubscription, error)
	Cancel(ctx context.Context, id string, params *SquareSubscriptionCancelParams) (*SquareSubscription, error)
	Get(ctx context.Context, id string, params *SquareSubscriptionParams) (*SquareSubscription, error)
}

// NewSquareClient returns a placeholder implementation until the real Square API wiring is ready.
func NewSquareClient() SquareSubscriptionClient {
	return &squareClientWrapper{
		planWindow: 30 * 24 * time.Hour,
	}
}

type squareClientWrapper struct {
	planWindow time.Duration
}

func (w *squareClientWrapper) Create(ctx context.Context, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	now := time.Now().UTC()
	return &SquareSubscription{
		ID:                 uuid.NewString(),
		Status:             "ACTIVE",
		Metadata:           params.Metadata,
		CancelAtPeriodEnd:  false,
		StartDate:          now.Unix(),
		ChargedThroughDate: now.Add(w.planWindow).Unix(),
		Items: &SquareSubscriptionItemList{
			Data: []*SquareSubscriptionItem{
				{
					CurrentPeriodStart: now.Unix(),
					CurrentPeriodEnd:   now.Add(w.planWindow).Unix(),
					Price: &SquareSubscriptionPrice{
						ID: params.PriceID,
					},
				},
			},
		},
	}, nil
}

func (w *squareClientWrapper) Cancel(ctx context.Context, id string, params *SquareSubscriptionCancelParams) (*SquareSubscription, error) {
	now := time.Now().UTC()
	return &SquareSubscription{
		ID:                 id,
		Status:             "CANCELED",
		CancelAtPeriodEnd:  true,
		CanceledAt:         now.Unix(),
		ChargedThroughDate: now.Unix(),
	}, nil
}

func (w *squareClientWrapper) Get(ctx context.Context, id string, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	now := time.Now().UTC()
	return &SquareSubscription{
		ID:                 id,
		Status:             "ACTIVE",
		StartDate:          now.Unix(),
		ChargedThroughDate: now.Add(w.planWindow).Unix(),
	}, nil
}
