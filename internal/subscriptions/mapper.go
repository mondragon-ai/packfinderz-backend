package subscriptions

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
)

// BuildSubscriptionFromStripe maps a Stripe subscription to the canonical model.
func BuildSubscriptionFromStripe(stripeSub *stripe.Subscription, storeID uuid.UUID, priceID, customerID, paymentMethodID string) (*models.Subscription, error) {
	if stripeSub == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "stripe subscription is nil")
	}
	status := enums.SubscriptionStatus(stripeSub.Status)
	if !status.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "invalid stripe subscription status")
	}

	metaExtras := map[string]string{}
	if customerID != "" {
		metaExtras["stripe_customer_id"] = customerID
	}
	if paymentMethodID != "" {
		metaExtras["stripe_payment_method_id"] = paymentMethodID
	}

	metadata, err := mergeMetadata(stripeSub.Metadata, metaExtras)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	startTS, endTS := periodFromSubscription(stripeSub)
	var price *string
	if strings.TrimSpace(priceID) != "" {
		price = &priceID
	}
	return &models.Subscription{
		StoreID:              storeID,
		StripeSubscriptionID: stripeSub.ID,
		Status:               status,
		PriceID:              price,
		CurrentPeriodStart:   toTimePtr(startTS),
		CurrentPeriodEnd:     toTime(endTS),
		CancelAtPeriodEnd:    stripeSub.CancelAtPeriodEnd,
		CanceledAt:           toTimePtr(stripeSub.CanceledAt),
		Metadata:             metadata,
	}, nil
}

// UpdateSubscriptionFromStripe mutates the provided subscription with new Stripe data.
func UpdateSubscriptionFromStripe(target *models.Subscription, stripeSub *stripe.Subscription, priceID *string) error {
	if target == nil {
		return pkgerrors.New(pkgerrors.CodeInternal, "target subscription is nil")
	}
	if stripeSub == nil {
		return pkgerrors.New(pkgerrors.CodeDependency, "stripe subscription is nil")
	}
	status := enums.SubscriptionStatus(stripeSub.Status)
	if !status.IsValid() {
		return pkgerrors.New(pkgerrors.CodeDependency, "invalid stripe subscription status")
	}

	metadata, err := mergeMetadata(stripeSub.Metadata, nil)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	target.StripeSubscriptionID = stripeSub.ID
	target.Status = status
	if priceID != nil {
		target.PriceID = priceID
	}
	startTS, endTS := periodFromSubscription(stripeSub)
	target.CurrentPeriodStart = toTimePtr(startTS)
	target.CurrentPeriodEnd = toTime(endTS)
	target.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd
	target.CanceledAt = toTimePtr(stripeSub.CanceledAt)
	target.Metadata = metadata
	return nil
}

// StoreIDFromMetadata extracts the store ID that was attached to Stripe metadata.
func StoreIDFromMetadata(metadata map[string]string) (uuid.UUID, error) {
	if metadata == nil {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeValidation, "subscription metadata is required")
	}
	storeID, ok := metadata["store_id"]
	if !ok || strings.TrimSpace(storeID) == "" {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeValidation, "store_id missing from metadata")
	}
	id, err := uuid.Parse(strings.TrimSpace(storeID))
	if err != nil {
		return uuid.Nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store_id metadata")
	}
	return id, nil
}

// IsActiveStatus reports whether the provided status keeps the subscription active.
func IsActiveStatus(status enums.SubscriptionStatus) bool {
	return status != enums.SubscriptionStatusCanceled
}

func mergeMetadata(base map[string]string, extras map[string]string) (json.RawMessage, error) {
	if len(base) == 0 && len(extras) == 0 {
		return json.RawMessage("{}"), nil
	}
	merged := make(map[string]string, len(base)+len(extras))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extras {
		if v == "" {
			continue
		}
		merged[k] = v
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func toTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}

func toTimePtr(ts int64) *time.Time {
	if ts == 0 {
		return nil
	}
	t := time.Unix(ts, 0).UTC()
	return &t
}

func periodFromSubscription(sub *stripe.Subscription) (int64, int64) {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 {
		return 0, 0
	}
	item := sub.Items.Data[0]
	return item.CurrentPeriodStart, item.CurrentPeriodEnd
}
