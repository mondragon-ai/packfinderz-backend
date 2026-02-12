package subscriptions

import (
	"encoding/json"
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
	pauseAt, resumeAt := scheduleFromActions(squareSub.Actions)
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
		PauseEffectiveAt:     pauseAt,
		ResumeEffectiveAt:    resumeAt,
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
	pauseAt, resumeAt := scheduleFromActions(squareSub.Actions)
	target.PauseEffectiveAt = pauseAt
	target.ResumeEffectiveAt = resumeAt
	if target.Status == enums.SubscriptionStatusPaused {
		now := time.Now().UTC()
		target.PausedAt = &now
	} else {
		target.PausedAt = nil
	}
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
	return status != enums.SubscriptionStatusCanceled && status != enums.SubscriptionStatusPaused
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

func scheduleFromActions(actions []*SquareSubscriptionAction) (*time.Time, *time.Time) {
	if len(actions) == 0 {
		return nil, nil
	}
	var pause, resume *time.Time
	for _, action := range actions {
		if action == nil || action.EffectiveDate == 0 {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(action.Type)) {
		case "PAUSE":
			pause = toTimePtr(action.EffectiveDate)
		case "RESUME":
			resume = toTimePtr(action.EffectiveDate)
		}
	}
	return pause, resume
}

func trimmedPtr(value string) *string {
	if s := strings.TrimSpace(value); s != "" {
		return &s
	}
	return nil
}

func mapSquareStatus(raw string) (enums.SubscriptionStatus, error) {
	normalized := normalizeSquareStatus(raw)
	if normalized == "" {
		return enums.SubscriptionStatusActive, nil
	}
	if mapped, ok := squareStatusAliases[normalized]; ok {
		return mapped, nil
	}
	if parsed, err := enums.ParseSubscriptionStatus(strings.ToLower(normalized)); err == nil {
		return parsed, nil
	}
	return enums.SubscriptionStatusActive, nil
}

func normalizeSquareStatus(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.ToUpper(normalized)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

var squareStatusAliases = map[string]enums.SubscriptionStatus{
	"PENDING":     enums.SubscriptionStatusTrialing,
	"TRIAL":       enums.SubscriptionStatusTrialing,
	"DEACTIVATED": enums.SubscriptionStatusCanceled,
	"COMPLETED":   enums.SubscriptionStatusCanceled,
	"CANCELING":   enums.SubscriptionStatusCanceled,
	"CANCELLING":  enums.SubscriptionStatusCanceled,
	"CANCELLED":   enums.SubscriptionStatusCanceled,
	"SUSPENDED":   enums.SubscriptionStatusPaused,
}
