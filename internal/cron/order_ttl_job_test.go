package cron

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestOrderTTLJob_nudgeEmitsPendingEvent(t *testing.T) {
	now := time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC)
	order := models.VendorOrder{
		ID:              uuid.New(),
		CheckoutGroupID: uuid.New(),
		BuyerStoreID:    uuid.New(),
		VendorStoreID:   uuid.New(),
		Status:          enums.VendorOrderStatusCreatedPending,
	}
	reader := &fakePendingReader{
		nudgeCutoff:      now.Add(-pendingNudgeDays * 24 * time.Hour),
		expireCutoff:     now.Add(-orderExpirationDays * 24 * time.Hour),
		nudgeOrders:      []models.VendorOrder{order},
		expirationOrders: nil,
	}
	helper := newOrderTTLJobTest(t, reader)
	helper.job.now = func() time.Time { return now }
	helper.outboxRepo.exists = false

	if err := helper.job.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(helper.outboxSvc.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(helper.outboxSvc.events))
	}
	event := helper.outboxSvc.events[0]
	if event.EventType != enums.EventOrderPendingNudge {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	payload, ok := event.Data.(payloads.OrderPendingNudgeEvent)
	if !ok {
		t.Fatal("expected pending nudge event payload")
	}
	if payload.OrderID != order.ID {
		t.Fatalf("unexpected order id: %s", payload.OrderID)
	}
	if payload.PendingDays != pendingNudgeDays {
		t.Fatalf("unexpected pending days: %d", payload.PendingDays)
	}
}

func TestOrderTTLJob_expireReleasesInventoryAndEmitsEvent(t *testing.T) {
	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	order := models.VendorOrder{
		ID:              uuid.New(),
		CheckoutGroupID: uuid.New(),
		BuyerStoreID:    uuid.New(),
		VendorStoreID:   uuid.New(),
		Status:          enums.VendorOrderStatusCreatedPending,
	}
	lineItem := models.OrderLineItem{
		ID:        uuid.New(),
		OrderID:   order.ID,
		ProductID: ptrUUID(uuid.New()),
		Qty:       3,
		Status:    enums.LineItemStatusPending,
	}
	reader := &fakePendingReader{
		nudgeCutoff:      now.Add(-pendingNudgeDays * 24 * time.Hour),
		expireCutoff:     now.Add(-orderExpirationDays * 24 * time.Hour),
		nudgeOrders:      nil,
		expirationOrders: []models.VendorOrder{order},
	}
	helper := newOrderTTLJobTest(t, reader)
	helper.job.now = func() time.Time { return now }
	fakeRepo := &fakeTransactionalRepo{
		order: &order,
		items: []models.OrderLineItem{lineItem},
	}
	helper.job.repoFactory = func(tx *gorm.DB) transactionalOrderRepo {
		return fakeRepo
	}

	if err := helper.job.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(helper.inventory.calls) != 1 {
		t.Fatalf("expected inventory release, got %d", len(helper.inventory.calls))
	}
	if len(helper.outboxSvc.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(helper.outboxSvc.events))
	}
	event := helper.outboxSvc.events[0]
	if event.EventType != enums.EventOrderExpired {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	payload, ok := event.Data.(payloads.OrderExpiredEvent)
	if !ok {
		t.Fatal("expected expiration event payload")
	}
	if payload.OrderID != order.ID {
		t.Fatalf("unexpected order id: %s", payload.OrderID)
	}
	if len(fakeRepo.orderUpdates) != 1 {
		t.Fatalf("expected order update, got %d", len(fakeRepo.orderUpdates))
	}
	update := fakeRepo.orderUpdates[0]
	if update.status != enums.VendorOrderStatusExpired {
		t.Fatalf("expected status expired, got %s", update.status)
	}
	if update.expiredAt.IsZero() {
		t.Fatal("expected expired timestamp to be set")
	}
	if len(fakeRepo.lineItemUpdates) != 1 {
		t.Fatalf("expected line item update, got %d", len(fakeRepo.lineItemUpdates))
	}
	lineUpdate := fakeRepo.lineItemUpdates[0]
	if lineUpdate.status != enums.LineItemStatusRejected {
		t.Fatalf("expected line item rejected, got %s", lineUpdate.status)
	}
}

type orderTTLJobTestHelper struct {
	job        *orderTTLJob
	outboxSvc  *fakeOutboxService
	outboxRepo *fakeOutboxRepo
	inventory  *fakeInventoryReleaser
}

func newOrderTTLJobTest(t *testing.T, reader pendingOrderReader) *orderTTLJobTestHelper {
	t.Helper()
	outboxSvc := &fakeOutboxService{}
	outboxRepo := &fakeOutboxRepo{}
	inventory := &fakeInventoryReleaser{}
	jobIface, err := NewOrderTTLJob(OrderTTLJobParams{
		Logger:        logger.New(logger.Options{ServiceName: "test"}),
		DB:            fakeTxRunner{},
		PendingReader: reader,
		Inventory:     inventory,
		Outbox:        outboxSvc,
		OutboxRepo:    outboxRepo,
	})
	if err != nil {
		t.Fatalf("NewOrderTTLJob: %v", err)
	}
	job, ok := jobIface.(*orderTTLJob)
	if !ok {
		t.Fatalf("expected orderTTLJob, got %T", jobIface)
	}
	return &orderTTLJobTestHelper{
		job:        job,
		outboxSvc:  outboxSvc,
		outboxRepo: outboxRepo,
		inventory:  inventory,
	}
}

type fakePendingReader struct {
	nudgeCutoff      time.Time
	expireCutoff     time.Time
	nudgeOrders      []models.VendorOrder
	expirationOrders []models.VendorOrder
}

func (f *fakePendingReader) FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error) {
	switch {
	case cutoff.Equal(f.nudgeCutoff):
		return f.nudgeOrders, nil
	case cutoff.Equal(f.expireCutoff):
		return f.expirationOrders, nil
	default:
		return nil, fmt.Errorf("unexpected cutoff: %s", cutoff)
	}
}

type fakeInventoryReleaser struct {
	calls []inventoryReleaseCall
}

type inventoryReleaseCall struct {
	productID uuid.UUID
	qty       int
}

func (f *fakeInventoryReleaser) Release(ctx context.Context, tx *gorm.DB, productID uuid.UUID, qty int) error {
	f.calls = append(f.calls, inventoryReleaseCall{productID: productID, qty: qty})
	return nil
}

type fakeTransactionalRepo struct {
	order           *models.VendorOrder
	items           []models.OrderLineItem
	orderUpdates    []orderUpdateCall
	lineItemUpdates []lineItemUpdateCall
}

type orderUpdateCall struct {
	orderID   uuid.UUID
	status    enums.VendorOrderStatus
	expiredAt time.Time
}

type lineItemUpdateCall struct {
	lineItemID uuid.UUID
	status     enums.LineItemStatus
}

func (f *fakeTransactionalRepo) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	return f.order, nil
}

func (f *fakeTransactionalRepo) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	return f.items, nil
}

func (f *fakeTransactionalRepo) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	status, _ := updates["status"].(enums.VendorOrderStatus)
	expiredAt, _ := updates["expired_at"].(time.Time)
	f.orderUpdates = append(f.orderUpdates, orderUpdateCall{
		orderID:   orderID,
		status:    status,
		expiredAt: expiredAt,
	})
	return nil
}

func (f *fakeTransactionalRepo) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	f.lineItemUpdates = append(f.lineItemUpdates, lineItemUpdateCall{lineItemID: lineItemID, status: status})
	return nil
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
