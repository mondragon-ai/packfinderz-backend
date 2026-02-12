package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	sq "github.com/square/square-go-sdk"
	sqcore "github.com/square/square-go-sdk/core"
	"gorm.io/gorm"
)

type storeRepository interface {
	FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error)
	UpdateWithTx(tx *gorm.DB, store *models.Store) error
	UpdateSubscriptionActiveWithTx(tx *gorm.DB, storeID uuid.UUID, active bool) error
}

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

// Service defines the subscription lifecycle surface.
type Service interface {
	Create(ctx context.Context, storeID uuid.UUID, input CreateSubscriptionInput) (*models.Subscription, bool, error)
	Cancel(ctx context.Context, storeID uuid.UUID) error
	GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error)
	Pause(ctx context.Context, storeID uuid.UUID) error
	Resume(ctx context.Context, storeID uuid.UUID) error
}

// ServiceParams groups dependencies for the subscription service.
type ServiceParams struct {
	BillingRepo       billing.Repository
	StoreRepo         storeRepository
	SquareClient      SquareSubscriptionClient
	DefaultPriceID    string
	TransactionRunner txRunner
}

// CreateSubscriptionInput captures the data required to start a subscription.
type CreateSubscriptionInput struct {
	SquareCustomerID      string
	SquarePaymentMethodID string
	PriceID               string
}

type service struct {
	billingRepo billing.Repository
	storeRepo   storeRepository
	square      SquareSubscriptionClient
	priceID     string
	txRunner    txRunner
}

// NewService builds a subscription service with the required dependencies.
func NewService(params ServiceParams) (Service, error) {
	if params.BillingRepo == nil {
		return nil, fmt.Errorf("billing repo required")
	}
	if params.StoreRepo == nil {
		return nil, fmt.Errorf("store repo required")
	}
	if params.SquareClient == nil {
		return nil, fmt.Errorf("square client required")
	}
	if params.TransactionRunner == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if strings.TrimSpace(params.DefaultPriceID) == "" {
		return nil, fmt.Errorf("default price id required")
	}
	return &service{
		billingRepo: params.BillingRepo,
		storeRepo:   params.StoreRepo,
		square:      params.SquareClient,
		priceID:     strings.TrimSpace(params.DefaultPriceID),
		txRunner:    params.TransactionRunner,
	}, nil
}

// Create either returns the existing active subscription or creates a new one.
func (s *service) Create(ctx context.Context, storeID uuid.UUID, input CreateSubscriptionInput) (*models.Subscription, bool, error) {
	fmt.Printf("[subscriptions.Create] start storeID=%s\n", storeID)

	if storeID == uuid.Nil {
		fmt.Printf("[subscriptions.Create] FAIL storeID is nil\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	customerID := strings.TrimSpace(input.SquareCustomerID)
	fmt.Printf("[subscriptions.Create] customerID='%s'\n", customerID)
	if customerID == "" {
		fmt.Printf("[subscriptions.Create] FAIL missing customerID\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "square_customer_id is required")
	}

	paymentMethodID := strings.TrimSpace(input.SquarePaymentMethodID)
	fmt.Printf("[subscriptions.Create] paymentMethodID='%s'\n", paymentMethodID)
	if paymentMethodID == "" {
		fmt.Printf("[subscriptions.Create] FAIL missing paymentMethodID\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "square_payment_method_id is required")
	}

	priceID := strings.TrimSpace(input.PriceID)
	if priceID == "" {
		priceID = s.priceID
	}
	fmt.Printf("[subscriptions.Create] priceID='%s' (input=%t default=%t)\n", priceID, strings.TrimSpace(input.PriceID) != "", s.priceID != "")
	if priceID == "" {
		fmt.Printf("[subscriptions.Create] FAIL missing priceID\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "price_id is required")
	}

	fmt.Printf("[subscriptions.Create] findActive (pre)\n")
	if existing, err := s.findActive(ctx, storeID); err != nil {
		fmt.Printf("[subscriptions.Create] FAIL findActive err=%T %v\n", err, err)
		return nil, false, err
	} else if existing != nil {
		fmt.Printf("[subscriptions.Create] existing active subscription found id=%s status=%s\n", existing.ID, existing.Status)
		return existing, false, nil
	}

	params := &SquareSubscriptionParams{
		CustomerID:      customerID,
		PriceID:         priceID,
		PaymentMethodID: paymentMethodID,
		Metadata: map[string]string{
			"store_id": storeID.String(),
		},
	}

	fmt.Printf("[subscriptions.Create] square params=%+v\n", params)
	fmt.Printf("[subscriptions.Create] square.Create about to call Square\n")
	squareSub, err := s.square.Create(ctx, params)
	if err != nil {
		fmt.Printf("[subscriptions.Create] FAIL square.Create err=%T %v\n", err, err)
		// If Square SDK returns a structured error, this prints more detail:
		debugSquareErr(err)
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create square subscription")
	}
	fmt.Printf("[subscriptions.Create] square.Create OK squareSubID=%s status=%s\n", squareSub.ID, squareSub.Status)
	fmt.Printf("[subscriptions.Create] square response=%+v\n", squareSub)
	squareSubID := squareSub.ID
	refreshParams := squareSubscriptionParams(storeID, priceID, true)
	squareSub, err = s.square.Get(ctx, squareSubID, refreshParams)
	if err != nil {
		fmt.Printf("[subscriptions.Create] FAIL square.Get err=%T %v\n", err, err)
		if cancelErr := s.cancelSquare(ctx, squareSubID); cancelErr != nil {
			fmt.Printf("[subscriptions.Create] FAIL cancelSquare err=%T %v\n", cancelErr, cancelErr)
			return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription after get failure")
		}
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "get square subscription")
	}
	fmt.Printf("[subscriptions.Create] square.Get refreshed response=%+v\n", squareSub)

	var (
		createdSub    *models.Subscription
		existingAfter *models.Subscription
		skipped       bool
	)

	fmt.Printf("[subscriptions.Create] begin tx persist\n")
	err = s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		fmt.Printf("[subscriptions.Create.tx] start\n")
		txRepo := s.billingRepo.WithTx(tx)

		fmt.Printf("[subscriptions.Create.tx] findActiveWithTx\n")
		active, err := s.findActiveWithTx(ctx, txRepo, storeID)
		if err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL findActiveWithTx err=%T %v\n", err, err)
			return err
		}
		if active != nil {
			fmt.Printf("[subscriptions.Create.tx] active exists (race) subID=%s\n", active.ID)
			existingAfter = active
			skipped = true
			return nil
		}

		fmt.Printf("[subscriptions.Create.tx] BuildSubscriptionFromSquare\n")
		sub, err := BuildSubscriptionFromSquare(squareSub, storeID, priceID, customerID, paymentMethodID)
		if err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL BuildSubscriptionFromSquare err=%T %v\n", err, err)
			return err
		}

		fmt.Printf("[subscriptions.Create.tx] CreateSubscription\n")
		if err := txRepo.CreateSubscription(ctx, sub); err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL CreateSubscription err=%T %v\n", err, err)
			return err
		}

		fmt.Printf("[subscriptions.Create.tx] load store\n")
		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL FindByIDWithTx err=%T %v\n", err, err)
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}

		store.SubscriptionActive = IsActiveStatus(sub.Status)
		fmt.Printf("[subscriptions.Create.tx] Update store subscription flag => %t\n", store.SubscriptionActive)
		if err := s.storeRepo.UpdateSubscriptionActiveWithTx(tx, storeID, store.SubscriptionActive); err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL UpdateWithTx err=%T %v\n", err, err)
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}

		createdSub = sub
		fmt.Printf("[subscriptions.Create.tx] OK\n")
		return nil
	})

	if err != nil {
		fmt.Printf("[subscriptions.Create] FAIL persist err=%T %v skipped=%t\n", err, err, skipped)
		if !skipped {
			fmt.Printf("[subscriptions.Create] cancelSquare due to db error squareSubID=%s\n", squareSub.ID)
			if cancelErr := s.cancelSquare(ctx, squareSub.ID); cancelErr != nil {
				fmt.Printf("[subscriptions.Create] FAIL cancelSquare err=%T %v\n", cancelErr, cancelErr)
				return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription after db error")
			}
		}
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist subscription")
	}

	if skipped {
		fmt.Printf("[subscriptions.Create] skipped due to race, cancelSquare squareSubID=%s\n", squareSub.ID)
		if cancelErr := s.cancelSquare(ctx, squareSub.ID); cancelErr != nil {
			fmt.Printf("[subscriptions.Create] FAIL cancelSquare(race) err=%T %v\n", cancelErr, cancelErr)
			return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription due to race")
		}
		return existingAfter, false, nil
	}

	fmt.Printf("[subscriptions.Create] DONE createdSubID=%s\n", createdSub.ID)
	return createdSub, true, nil
}

// debugSquareErr tries to print useful details when Square SDK returns structured errors.
// Keep it best-effort: it should never panic.
func debugSquareErr(err error) {
	if err == nil {
		return
	}
	fmt.Printf("[square.err] raw=%T %v\n", err, err)

	// Many Square SDK errors implement these shapes; adapt as needed once you see output.
	type hasErrors interface {
		GetErrors() []sq.Error
	}
	if he, ok := err.(hasErrors); ok {
		es := he.GetErrors()
		fmt.Printf("[square.err] count=%d\n", len(es))
		for i, e := range es {
			detail := ""
			if e.Detail != nil {
				detail = *e.Detail
			}
			field := ""
			if e.Field != nil {
				field = *e.Field
			}
			fmt.Printf("[square.err] #%d category=%s code=%s detail=%s field=%s\n",
				i, e.Category, e.Code, detail, field)
		}
	}
}

// Cancel terminates the active subscription (if any) and flips the store flag.
func (s *service) Cancel(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	active, err := s.findActive(ctx, storeID)
	if err != nil {
		return err
	}
	if active == nil {
		return s.ensureStoreFlag(ctx, storeID, false)
	}

	squareSub, err := s.square.Cancel(ctx, active.SquareSubscriptionID, &SquareSubscriptionCancelParams{})
	if err != nil {
		if isSquareCancelAlreadyScheduled(err) {
			return s.syncSquareSubscription(ctx, storeID, active.SquareSubscriptionID, active.PriceID)
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "cancel square subscription")
	}
	fmt.Printf("[subscriptions.Cancel] square cancel response=%+v\n", squareSub)

	if err := s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.billingRepo.WithTx(tx)
		stored, err := s.findActiveWithTx(ctx, txRepo, storeID)
		if err != nil {
			return err
		}
		if stored == nil {
			return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
		}

		if err := UpdateSubscriptionFromSquare(stored, squareSub, stored.PriceID); err != nil {
			return err
		}
		if err := txRepo.UpdateSubscription(ctx, stored); err != nil {
			fmt.Printf("[subscriptions.persistSquareUpdate] FAIL UpdateSubscription err=%T %v\n", err, err)
			return err
		}

		// _, err = s.storeRepo.FindByIDWithTx(tx, storeID)
		// if err != nil {
		// 	if err == gorm.ErrRecordNotFound {
		// 		return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
		// 	}
		// 	return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		// }
		if err := s.storeRepo.UpdateSubscriptionActiveWithTx(tx, storeID, false); err != nil {
			fmt.Printf("[subscriptions.persistSquareUpdate] FAIL storeRepo.UpdateWithTx err=%T %v storeID=%s\n", err, err, storeID)
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}
		return nil
	}); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist cancellation")
	}

	return nil
}
func (s *service) Pause(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return err
	}
	if sub == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
	}
	if strings.TrimSpace(sub.SquareSubscriptionID) == "" {
		return pkgerrors.New(pkgerrors.CodeStateConflict, "square subscription id missing")
	}

	fmt.Printf("[subscriptions.Pause] start storeID=%s squareID=%s status=%s\n", storeID, sub.SquareSubscriptionID, sub.Status)

	live, err := s.square.Get(ctx, sub.SquareSubscriptionID, squareSubscriptionParams(storeID, resolvePriceID(sub.PriceID), true))
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "get square subscription")
	}
	if live == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "square subscription not found")
	}

	fmt.Printf("[subscriptions.Pause] square live state=%+v\n", live)

	if strings.ToUpper(live.Status) != "ACTIVE" {
		return pkgerrors.New(pkgerrors.CodeStateConflict,
			fmt.Sprintf("subscription not active in Square (status=%s)", live.Status),
		)
	}

	pauseParams := &SquareSubscriptionPauseParams{
		PriceID:            resolvePriceID(sub.PriceID),
		PauseEffectiveDate: pauseEffectiveDateFor(live, sub),
	}
	paused, err := s.square.Pause(ctx, sub.SquareSubscriptionID, pauseParams)
	if err != nil {
		if isSquarePauseAlreadyScheduled(err) {
			return s.syncSquareSubscription(ctx, storeID, sub.SquareSubscriptionID, sub.PriceID)
		}
		return pkgerrors.Wrap(pkgerrors.CodeStateConflict, err, "pause square subscription")
	}

	fmt.Printf("[subscriptions.Pause] square paused state=%+v\n", paused)

	return s.persistSquareUpdate(ctx, storeID, paused, func(stored *models.Subscription) {
		now := time.Now().UTC()
		stored.PausedAt = &now
	})
}

func (s *service) Resume(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return err
	}
	if sub == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
	}

	fmt.Printf("[subscriptions.Resume] start storeID=%s squareID=%s status=%s\n", storeID, sub.SquareSubscriptionID, sub.Status)

	live, err := s.square.Get(ctx, sub.SquareSubscriptionID, squareSubscriptionParams(storeID, resolvePriceID(sub.PriceID), true))
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "get square subscription before resume")
	}
	if live == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "square subscription not found")
	}
	if strings.EqualFold(live.Status, "ACTIVE") {
		if pause := pendingPauseAction(live.Actions); pause != nil {
			if _, err := s.square.DeleteAction(ctx, sub.SquareSubscriptionID, pause.ID); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeStateConflict, err, "delete scheduled pause")
			}
			return s.syncSquareSubscription(ctx, storeID, sub.SquareSubscriptionID, sub.PriceID)
		}
		return nil
	}

	squareSub, err := s.square.Resume(ctx, sub.SquareSubscriptionID, &SquareSubscriptionResumeParams{
		PriceID: resolvePriceID(sub.PriceID),
	})
	if err != nil {
		return wrapResumeError(err, "resume square subscription")
	}

	fmt.Printf("[subscriptions.Resume] square resumed state=%+v\n", squareSub)

	return s.persistSquareUpdate(ctx, storeID, squareSub, func(stored *models.Subscription) {
		stored.PausedAt = nil
	})
}

// GetActive returns the current active subscription if one exists.
func (s *service) GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	fmt.Printf("[subscriptions.GetActive] storeID=%s\n", storeID)
	sub, err := s.findActive(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, nil
	}
	if err := s.syncSquareSubscription(ctx, storeID, sub.SquareSubscriptionID, sub.PriceID); err != nil {
		return nil, err
	}
	return s.findActive(ctx, storeID)
}

func (s *service) findActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return nil, err
	}
	active := sub != nil && IsActiveStatus(sub.Status)
	fmt.Printf("[subscriptions.findActive] storeID=%s found=%t sub=%+v\n", storeID, active, sub)
	if !active {
		return nil, nil
	}
	return sub, nil
}

func (s *service) findActiveWithTx(ctx context.Context, repo billing.Repository, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.findSubscriptionWithTx(ctx, repo, storeID)
	if err != nil {
		return nil, err
	}
	if sub == nil || !IsActiveStatus(sub.Status) {
		return nil, nil
	}
	return sub, nil
}

func (s *service) persistSquareUpdate(ctx context.Context, storeID uuid.UUID, squareSub *SquareSubscription, modify func(*models.Subscription)) error {
	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.billingRepo.WithTx(tx)
		stored, err := s.findSubscriptionWithTx(ctx, txRepo, storeID)
		if err != nil {
			return err
		}
		if stored == nil {
			return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
		}

		if err := UpdateSubscriptionFromSquare(stored, squareSub, stored.PriceID); err != nil {
			return err
		}
		if modify != nil {
			modify(stored)
		}
		if err := txRepo.UpdateSubscription(ctx, stored); err != nil {
			fmt.Printf("[subscriptions.persistSquareUpdate] FAIL UpdateSubscription err=%T %v\n", err, err)
			return err
		}

		// store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		// if err != nil {
		// 	if err == gorm.ErrRecordNotFound {
		// 		return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
		// 	}
		// 	return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		// }
		active := IsActiveStatus(stored.Status)
		if err := s.storeRepo.UpdateSubscriptionActiveWithTx(tx, storeID, active); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}

		return nil
	})
}

func (s *service) syncSquareSubscription(ctx context.Context, storeID uuid.UUID, squareID string, priceID *string) error {
	if squareID == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "square subscription id is required")
	}
	squareSub, err := s.square.Get(ctx, squareID, squareSubscriptionParams(storeID, resolvePriceID(priceID), true))
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "get square subscription")
	}
	if squareSub == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "square subscription not found")
	}
	if err := s.persistSquareUpdate(ctx, storeID, squareSub, nil); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist square subscription")
	}
	return nil
}

func squareSubscriptionParams(storeID uuid.UUID, priceID string, includeActions bool) *SquareSubscriptionParams {
	return &SquareSubscriptionParams{
		PriceID:        priceID,
		Metadata:       map[string]string{"store_id": storeID.String()},
		IncludeActions: includeActions,
	}
}

func pendingPauseAction(actions []*SquareSubscriptionAction) *SquareSubscriptionAction {
	if len(actions) == 0 {
		return nil
	}
	for _, action := range actions {
		if action == nil || action.ID == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(action.Type), "PAUSE") {
			return action
		}
	}
	return nil
}

func wrapResumeError(err error, msg string) error {
	if err == nil {
		return nil
	}
	if pkgErr := pkgerrors.As(err); pkgErr != nil {
		return pkgerrors.Wrap(pkgErr.Code(), err, msg)
	}
	return pkgerrors.Wrap(pkgerrors.CodeDependency, err, msg)
}

func pauseEffectiveDateFor(live *SquareSubscription, stored *models.Subscription) string {
	if live != nil && live.ChargedThroughDate != 0 {
		return formatSquareDate(live.ChargedThroughDate)
	}
	if stored != nil && !stored.CurrentPeriodEnd.IsZero() {
		return stored.CurrentPeriodEnd.UTC().Format("2006-01-02")
	}
	return time.Now().UTC().Format("2006-01-02")
}

func formatSquareDate(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02")
}

func isSquarePauseAlreadyScheduled(err error) bool {
	var pkgErr *pkgerrors.Error
	if !errors.As(err, &pkgErr) {
		return false
	}
	apiErr := pkgErr.Unwrap()
	if apiErr == nil {
		return false
	}
	var squareAPIErr *sqcore.APIError
	if !errors.As(apiErr, &squareAPIErr) {
		return false
	}
	inner := squareAPIErr.Unwrap()
	if inner == nil {
		return false
	}
	raw := strings.TrimSpace(inner.Error())
	if raw == "" {
		return false
	}
	var payload struct {
		Errors []*sq.Error `json:"errors"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	for _, sqErr := range payload.Errors {
		if sqErr == nil {
			continue
		}
		detail := ""
		if sqErr.Detail != nil {
			detail = *sqErr.Detail
		}
		if strings.Contains(strings.ToLower(detail), "already has a pending pause date") {
			return true
		}
	}
	return false
}

func isSquareCancelAlreadyScheduled(err error) bool {
	var pkgErr *pkgerrors.Error
	if !errors.As(err, &pkgErr) {
		return false
	}
	apiErr := pkgErr.Unwrap()
	if apiErr == nil {
		return false
	}
	var squareAPIErr *sqcore.APIError
	if !errors.As(apiErr, &squareAPIErr) {
		return false
	}
	inner := squareAPIErr.Unwrap()
	if inner == nil {
		return false
	}
	raw := strings.TrimSpace(inner.Error())
	if raw == "" {
		return false
	}
	var payload struct {
		Errors []*sq.Error `json:"errors"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	for _, sqErr := range payload.Errors {
		if sqErr == nil {
			continue
		}
		detail := ""
		if sqErr.Detail != nil {
			detail = *sqErr.Detail
		}
		if strings.Contains(strings.ToLower(detail), "already has a pending cancel date") {
			return true
		}
	}
	return false
}

func (s *service) findSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.billingRepo.FindSubscription(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup subscription")
	}
	return sub, nil
}

func (s *service) findSubscriptionWithTx(ctx context.Context, repo billing.Repository, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := repo.FindSubscription(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup subscription")
	}
	return sub, nil
}

func resolvePriceID(price *string) string {
	if price == nil {
		return ""
	}
	return *price
}

func (s *service) ensureStoreFlag(ctx context.Context, storeID uuid.UUID, active bool) error {
	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}
		if store.SubscriptionActive == active {
			return nil
		}
		store.SubscriptionActive = active
		return s.storeRepo.UpdateWithTx(tx, store)
	})
}

func (s *service) cancelSquare(ctx context.Context, id string) error {
	_, err := s.square.Cancel(ctx, id, &SquareSubscriptionCancelParams{})
	return err
}
