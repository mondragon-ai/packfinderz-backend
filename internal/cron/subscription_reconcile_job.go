package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	"gorm.io/gorm"
)

const (
	defaultReconcileLimit    = 250
	defaultReconcileLookback = 7 * 24 * time.Hour
)

// SubscriptionReconcileJobParams configures the subscription sync cron job.
type SubscriptionReconcileJobParams struct {
	Logger       *logger.Logger
	DB           txRunner
	BillingRepo  billing.Repository
	StoreRepo    storesRepository
	SquareClient subscriptions.SquareSubscriptionClient
	Limit        int
	Lookback     time.Duration
	Now          func() time.Time
}

// NewSubscriptionReconcileJob builds a reconciliation cron job.
func NewSubscriptionReconcileJob(params SubscriptionReconcileJobParams) (Job, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db runner required")
	}
	if params.BillingRepo == nil {
		return nil, fmt.Errorf("billing repository required")
	}
	if params.StoreRepo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	if params.SquareClient == nil {
		return nil, fmt.Errorf("square client required")
	}
	now := params.Now
	if now == nil {
		now = time.Now
	}
	lookback := params.Lookback
	if lookback <= 0 {
		lookback = defaultReconcileLookback
	}
	limit := params.Limit
	if limit <= 0 {
		limit = defaultReconcileLimit
	}
	return &subscriptionReconcileJob{
		logg:        params.Logger,
		db:          params.DB,
		billingRepo: params.BillingRepo,
		storeRepo:   params.StoreRepo,
		square:      params.SquareClient,
		now:         now,
		limit:       limit,
		lookback:    lookback,
	}, nil
}

type storesRepository interface {
	UpdateSubscriptionActiveWithTx(tx *gorm.DB, storeID uuid.UUID, active bool) error
}

type subscriptionReconcileJob struct {
	logg        *logger.Logger
	db          txRunner
	billingRepo billing.Repository
	storeRepo   storesRepository
	square      subscriptions.SquareSubscriptionClient
	now         func() time.Time
	limit       int
	lookback    time.Duration
}

func (j *subscriptionReconcileJob) Name() string { return "subscription-reconcile" }

func (j *subscriptionReconcileJob) Run(ctx context.Context) error {
	logCtx := j.logg.WithField(ctx, "job", j.Name())
	logCtx = j.logg.WithField(logCtx, "event", "cron.job")
	snapshot, err := j.billingRepo.ListSubscriptionsForReconciliation(logCtx, j.limit, j.lookback)
	if err != nil {
		return fmt.Errorf("list subscriptions for reconciliation: %w", err)
	}
	var errs error
	scanned := len(snapshot)
	synced := 0
	for i := range snapshot {
		if err := j.reconcileSubscription(logCtx, &snapshot[i]); err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		synced++
	}
	reportCtx := j.logg.WithFields(logCtx, map[string]any{
		"candidates": scanned,
		"synced":     synced,
	})
	j.logg.Info(reportCtx, "subscription reconcile loop complete")
	return errs
}

func (j *subscriptionReconcileJob) reconcileSubscription(ctx context.Context, sub *models.Subscription) error {
	fields := map[string]any{
		"subscription_id":        sub.ID,
		"store_id":               sub.StoreID,
		"square_subscription_id": sub.SquareSubscriptionID,
	}
	logCtx := j.logg.WithFields(ctx, fields)
	if strings.TrimSpace(sub.SquareSubscriptionID) == "" {
		j.logg.Info(logCtx, "subscription missing square id; skipping")
		return nil
	}
	squareSub, err := j.square.Get(logCtx, sub.SquareSubscriptionID, &subscriptions.SquareSubscriptionParams{IncludeActions: true})
	if err != nil {
		return fmt.Errorf("fetch square subscription: %w", err)
	}
	if squareSub == nil {
		j.logg.Info(logCtx, "square subscription not found; skipping")
		return nil
	}
	if err := j.db.WithTx(logCtx, func(tx *gorm.DB) error {
		repo := j.billingRepo.WithTx(tx)
		stored, err := repo.FindSubscriptionBySquareID(logCtx, squareSub.ID)
		if err != nil {
			return err
		}
		if stored == nil {
			j.logg.Info(logCtx, "subscription removed from db; skipping")
			return nil
		}
		if err := subscriptions.UpdateSubscriptionFromSquare(stored, squareSub, stored.PriceID); err != nil {
			return err
		}
		applyPendingActions(stored, squareSub.Actions)
		if err := repo.UpdateSubscription(logCtx, stored); err != nil {
			return err
		}
		active := deriveStoreSubscriptionActive(j.now(), squareSub, stored)
		if err := j.storeRepo.UpdateSubscriptionActiveWithTx(tx, stored.StoreID, active); err != nil {
			return err
		}
		successCtx := j.logg.WithFields(logCtx, map[string]any{
			"square_status": squareSub.Status,
			"entitled":      active,
		})
		j.logg.Info(successCtx, "subscription reconciled")
		return nil
	}); err != nil {
		return fmt.Errorf("persist subscription reconciliation: %w", err)
	}
	return nil
}

func applyPendingActions(stored *models.Subscription, actions []*subscriptions.SquareSubscriptionAction) {
	if stored == nil {
		return
	}
	if cancel := pendingCancelAction(actions); cancel != nil {
		stored.CancelAtPeriodEnd = true
		if t, ok := actionTime(cancel); ok {
			stored.CanceledAt = &t
		}
	}
	if pause := pendingPauseAction(actions); pause != nil {
		if t, ok := actionTime(pause); ok {
			stored.PauseEffectiveAt = &t
		}
	}
}

func pendingPauseAction(actions []*subscriptions.SquareSubscriptionAction) *subscriptions.SquareSubscriptionAction {
	for _, action := range actions {
		if action == nil || strings.TrimSpace(action.ID) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(action.Type), "PAUSE") {
			return action
		}
	}
	return nil
}

func pendingCancelAction(actions []*subscriptions.SquareSubscriptionAction) *subscriptions.SquareSubscriptionAction {
	for _, action := range actions {
		if action == nil || strings.TrimSpace(action.ID) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(action.Type), "CANCEL") {
			return action
		}
	}
	return nil
}

func deriveStoreSubscriptionActive(now time.Time, square *subscriptions.SquareSubscription, stored *models.Subscription) bool {
	if square == nil {
		return false
	}
	if cancel := pendingCancelAction(square.Actions); cancel != nil {
		if t, ok := actionTime(cancel); ok && !now.Before(t) {
			return false
		}
	}
	if pause := pendingPauseAction(square.Actions); pause != nil {
		if t, ok := actionTime(pause); ok && !now.Before(t) {
			return false
		}
	}
	entitledUntil := timeFromUnix(square.ChargedThroughDate)
	if entitledUntil.IsZero() && stored != nil {
		entitledUntil = stored.CurrentPeriodEnd
	}
	if entitledUntil.IsZero() {
		return strings.EqualFold(square.Status, "ACTIVE")
	}
	return !now.After(entitledUntil)
}

func actionTime(action *subscriptions.SquareSubscriptionAction) (time.Time, bool) {
	if action == nil || action.EffectiveDate == 0 {
		return time.Time{}, false
	}
	return time.Unix(action.EffectiveDate, 0).UTC(), true
}

func timeFromUnix(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}
