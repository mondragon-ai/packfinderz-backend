package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	"gorm.io/gorm"
)

const (
	pendingNudgeDays    = 5
	orderExpirationDays = 10
)

// OrderTTLJobParams configure the pending order scheduler.
type OrderTTLJobParams struct {
	Logger                   *logger.Logger
	DB                       txRunner
	PendingReader            pendingOrderReader
	Inventory                orders.InventoryReleaser
	Outbox                   outboxEmitter
	OutboxRepo               outboxExistenceChecker
	TransactionalRepoFactory transactionalRepoFactory
}

type pendingOrderReader interface {
	FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error)
}

type transactionalOrderRepo interface {
	FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error)
	FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error)
	UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error
	UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error
}

type transactionalRepoFactory func(tx *gorm.DB) transactionalOrderRepo

func defaultTransactionalRepo(tx *gorm.DB) transactionalOrderRepo {
	return orders.NewRepository(tx)
}

// NewOrderTTLJob builds the cron job that nudges and expires stale orders.
func NewOrderTTLJob(params OrderTTLJobParams) (Job, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db runner required")
	}
	if params.PendingReader == nil {
		return nil, fmt.Errorf("pending orders reader required")
	}
	if params.Inventory == nil {
		return nil, fmt.Errorf("inventory releaser required")
	}
	if params.Outbox == nil {
		return nil, fmt.Errorf("outbox service required")
	}
	if params.OutboxRepo == nil {
		return nil, fmt.Errorf("outbox repository required")
	}
	repoFactory := params.TransactionalRepoFactory
	if repoFactory == nil {
		repoFactory = defaultTransactionalRepo
	}
	return &orderTTLJob{
		logg:          params.Logger,
		db:            params.DB,
		pendingReader: params.PendingReader,
		inventory:     params.Inventory,
		outbox:        params.Outbox,
		outboxRepo:    params.OutboxRepo,
		repoFactory:   repoFactory,
		now:           time.Now,
	}, nil
}

type orderTTLJob struct {
	logg          *logger.Logger
	db            txRunner
	pendingReader pendingOrderReader
	inventory     orders.InventoryReleaser
	outbox        outboxEmitter
	outboxRepo    outboxExistenceChecker
	repoFactory   transactionalRepoFactory
	now           func() time.Time
}

func (j *orderTTLJob) Name() string { return "order-ttl" }

func (j *orderTTLJob) Run(ctx context.Context) error {
	var errs []error
	if err := j.nudgePendingOrders(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := j.expirePendingOrders(ctx); err != nil {
		errs = append(errs, err)
	}
	return multierr.Combine(errs...)
}

func (j *orderTTLJob) nudgePendingOrders(ctx context.Context) error {
	cutoff := j.now().UTC().Add(-pendingNudgeDays * 24 * time.Hour)
	orders, err := j.pendingReader.FindPendingOrdersBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("query pending orders for nudge: %w", err)
	}
	count := 0
	for _, order := range orders {
		if err := j.emitPendingNudge(ctx, order); err != nil {
			return err
		}
		count++
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{"count": count})
	j.logg.Info(logCtx, "order pending nudge loop complete")
	return nil
}

func (j *orderTTLJob) emitPendingNudge(ctx context.Context, order models.VendorOrder) error {
	exists, err := j.outboxRepo.Exists(ctx, enums.EventOrderPendingNudge, enums.AggregateVendorOrder, order.ID)
	if err != nil {
		return fmt.Errorf("check pending nudge existence: %w", err)
	}
	if exists {
		return nil
	}
	return j.db.WithTx(ctx, func(tx *gorm.DB) error {
		event := outbox.DomainEvent{
			EventType:     enums.EventOrderPendingNudge,
			AggregateType: enums.AggregateVendorOrder,
			AggregateID:   order.ID,
			Version:       1,
			OccurredAt:    j.now().UTC(),
			Data: OrderPendingNudgeEvent{
				OrderID:         order.ID,
				CheckoutGroupID: order.CheckoutGroupID,
				BuyerStoreID:    order.BuyerStoreID,
				VendorStoreID:   order.VendorStoreID,
				PendingDays:     pendingNudgeDays,
			},
		}
		return j.outbox.Emit(ctx, tx, event)
	})
}

func (j *orderTTLJob) expirePendingOrders(ctx context.Context) error {
	cutoff := j.now().UTC().Add(-orderExpirationDays * 24 * time.Hour)
	orders, err := j.pendingReader.FindPendingOrdersBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("query pending orders for expiration: %w", err)
	}
	count := 0
	for _, order := range orders {
		if err := j.expireOrder(ctx, order); err != nil {
			return err
		}
		count++
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{"count": count})
	j.logg.Info(logCtx, "order expiration loop complete")
	return nil
}

func (j *orderTTLJob) expireOrder(ctx context.Context, order models.VendorOrder) error {
	return j.db.WithTx(ctx, func(tx *gorm.DB) error {
		repo := j.repoFactory(tx)
		current, err := repo.FindVendorOrder(ctx, order.ID)
		if err != nil {
			return err
		}
		if current.Status != enums.VendorOrderStatusCreatedPending {
			return nil
		}
		items, err := repo.FindOrderLineItemsByOrder(ctx, order.ID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Status == enums.LineItemStatusFulfilled || item.Status == enums.LineItemStatusRejected {
				continue
			}
			if err := orders.ReleaseLineItemInventory(ctx, tx, item, j.inventory); err != nil {
				return err
			}
			if item.Status != enums.LineItemStatusRejected {
				if err := repo.UpdateOrderLineItemStatus(ctx, item.ID, enums.LineItemStatusRejected, nil); err != nil {
					return err
				}
			}
		}
		now := j.now().UTC()
		updates := map[string]any{
			"status":            enums.VendorOrderStatusExpired,
			"balance_due_cents": 0,
			"expired_at":        now,
		}
		if err := repo.UpdateVendorOrder(ctx, order.ID, updates); err != nil {
			return err
		}
		event := outbox.DomainEvent{
			EventType:     enums.EventOrderExpired,
			AggregateType: enums.AggregateVendorOrder,
			AggregateID:   order.ID,
			Version:       1,
			OccurredAt:    now,
			Data: OrderExpiredEvent{
				OrderID:         order.ID,
				CheckoutGroupID: order.CheckoutGroupID,
				BuyerStoreID:    order.BuyerStoreID,
				VendorStoreID:   order.VendorStoreID,
				ExpiredAt:       now,
			},
		}
		return j.outbox.Emit(ctx, tx, event)
	})
}

// OrderPendingNudgeEvent carries the payload for nudges.
type OrderPendingNudgeEvent struct {
	OrderID         uuid.UUID `json:"orderId"`
	CheckoutGroupID uuid.UUID `json:"checkoutGroupId"`
	BuyerStoreID    uuid.UUID `json:"buyerStoreId"`
	VendorStoreID   uuid.UUID `json:"vendorStoreId"`
	PendingDays     int       `json:"pendingDays"`
}

// OrderExpiredEvent describes the payload when orders expire.
type OrderExpiredEvent struct {
	OrderID         uuid.UUID `json:"orderId"`
	CheckoutGroupID uuid.UUID `json:"checkoutGroupId"`
	BuyerStoreID    uuid.UUID `json:"buyerStoreId"`
	VendorStoreID   uuid.UUID `json:"vendorStoreId"`
	ExpiredAt       time.Time `json:"expiredAt"`
}
