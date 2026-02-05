package subscriptions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

// BuildSubscriptionFromSquare maps a Square subscription into the canonical model.
func BuildSubscriptionFromSquare(squareSub *SquareSubscription, storeID uuid.UUID, priceID, customerID, paymentMethodID string) (*models.Subscription, error) {
	if squareSub == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "square subscription is nil")
	}
	status, err := mapSquareStatus(squareSub.Status)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "invalid square subscription status")
	}

	metaExtras := map[string]string{}
	if customerID != "" {
		metaExtras["square_customer_id"] = customerID
	}
	if paymentMethodID != "" {
		metaExtras["square_payment_method_id"] = paymentMethodID
		metaExtras["square_card_id"] = paymentMethodID
	}

	metadata, err := mergeMetadata(squareSub.Metadata, metaExtras)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	startTS, endTS := periodFromSubscription(squareSub)
	var price *string
	if strings.TrimSpace(priceID) != "" {
		price = &priceID
	}

	return &models.Subscription{
		StoreID:              storeID,
		SquareSubscriptionID: squareSub.ID,
		Status:               status,
		PriceID:              price,
		BillingPlanID:        nil,
		SquareCustomerID:     trimmedPtr(customerID),
		SquareCardID:         trimmedPtr(paymentMethodID),
		PausedAt:             nil,
		CurrentPeriodStart:   toTimePtr(startTS),
		CurrentPeriodEnd:     toTime(endTS),
		CancelAtPeriodEnd:    squareSub.CancelAtPeriodEnd,
		CanceledAt:           toTimePtr(squareSub.CanceledAt),
		Metadata:             metadata,
	}, nil
}

// UpdateSubscriptionFromSquare mutates the provided subscription with new Square data.
func UpdateSubscriptionFromSquare(target *models.Subscription, squareSub *SquareSubscription, priceID *string) error {
	if target == nil {
		return pkgerrors.New(pkgerrors.CodeInternal, "target subscription is nil")
	}
	if squareSub == nil {
		return pkgerrors.New(pkgerrors.CodeDependency, "square subscription is nil")
	}
	status, err := mapSquareStatus(squareSub.Status)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "invalid square subscription status")
	}

	metadata, err := mergeMetadata(squareSub.Metadata, nil)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	target.SquareSubscriptionID = squareSub.ID
	target.Status = status
	if priceID != nil {
		target.PriceID = priceID
	}
	startTS, endTS := periodFromSubscription(squareSub)
	target.CurrentPeriodStart = toTimePtr(startTS)
	target.CurrentPeriodEnd = toTime(endTS)
	target.CancelAtPeriodEnd = squareSub.CancelAtPeriodEnd
	target.CanceledAt = toTimePtr(squareSub.CanceledAt)
	target.Metadata = metadata
	return nil
}

// StoreIDFromMetadata extracts the store ID that was attached to Square metadata.
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

func periodFromSubscription(sub *SquareSubscription) (int64, int64) {
	if sub == nil {
		return 0, 0
	}
	if sub.StartDate != 0 && sub.ChargedThroughDate != 0 {
		return sub.StartDate, sub.ChargedThroughDate
	}
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		return item.CurrentPeriodStart, item.CurrentPeriodEnd
	}
	return 0, 0
}

func trimmedPtr(value string) *string {
	if s := strings.TrimSpace(value); s != "" {
		return &s
	}
	return nil
}

func mapSquareStatus(raw string) (enums.SubscriptionStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "ACTIVE":
		return enums.SubscriptionStatusActive, nil
	case "PENDING":
		return enums.SubscriptionStatusTrialing, nil
	case "CANCELED", "DEACTIVATED", "PAUSED", "COMPLETED":
		return enums.SubscriptionStatusCanceled, nil
	case "PAST_DUE", "PAST-DUE", "PASTDUE":
		return enums.SubscriptionStatusPastDue, nil
	default:
		return "", fmt.Errorf("unknown square subscription status %q", raw)
	}
}
