package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type outboxPublisher interface {
	Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error
}

// InventoryReleaser returns reserved stock when a line item is rejected.
type InventoryReleaser interface {
	Release(ctx context.Context, tx *gorm.DB, productID uuid.UUID, qty int) error
}

type inventoryReserver interface {
	Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error)
}

// Service defines order-level operations beyond repository reads.
type Service interface {
	VendorDecision(ctx context.Context, input VendorDecisionInput) error
	LineItemDecision(ctx context.Context, input LineItemDecisionInput) error
	CancelOrder(ctx context.Context, input BuyerCancelInput) error
	NudgeVendor(ctx context.Context, input BuyerNudgeInput) error
	RetryOrder(ctx context.Context, input BuyerRetryInput) (*BuyerRetryResult, error)
	AgentPickup(ctx context.Context, input AgentPickupInput) error
}

type service struct {
	repo      Repository
	tx        txRunner
	outbox    outboxPublisher
	inventory InventoryReleaser
	reserver  inventoryReserver
}

// VendorOrderDecision represents the high-level decision a vendor can take.
type VendorOrderDecision string

const (
	VendorOrderDecisionAccept VendorOrderDecision = "accept"
	VendorOrderDecisionReject VendorOrderDecision = "reject"
)

// VendorDecisionInput captures the data required to change an order's decision state.
type VendorDecisionInput struct {
	OrderID      uuid.UUID
	Decision     VendorOrderDecision
	ActorUserID  uuid.UUID
	ActorStoreID uuid.UUID
	ActorRole    string
}

// LineItemDecision captures the actions vendors can take on a line item.
type LineItemDecision string

const (
	LineItemDecisionFulfill LineItemDecision = "fulfill"
	LineItemDecisionReject  LineItemDecision = "reject"
)

// LineItemDecisionInput carries the contextual metadata required to resolve a line item.
type LineItemDecisionInput struct {
	OrderID      uuid.UUID
	LineItemID   uuid.UUID
	Decision     LineItemDecision
	Notes        *string
	ActorUserID  uuid.UUID
	ActorStoreID uuid.UUID
	ActorRole    string
}

// BuyerCancelInput carries metadata for buyer-initiated cancels.
type BuyerCancelInput struct {
	OrderID      uuid.UUID
	ActorUserID  uuid.UUID
	ActorStoreID uuid.UUID
	ActorRole    string
}

// BuyerNudgeInput captures the buyer request used to prod the vendor.
type BuyerNudgeInput struct {
	OrderID      uuid.UUID
	ActorUserID  uuid.UUID
	ActorStoreID uuid.UUID
	ActorRole    string
}

// BuyerRetryInput reuses an expired order snapshot so the buyer can try again.
type BuyerRetryInput struct {
	OrderID      uuid.UUID
	ActorUserID  uuid.UUID
	ActorStoreID uuid.UUID
	ActorRole    string
}

// BuyerRetryResult surfaces the new order created during a retry.
type BuyerRetryResult struct {
	OrderID uuid.UUID `json:"order_id"`
}

// AgentPickupInput captures the agent and order for pickup confirmation.
type AgentPickupInput struct {
	OrderID     uuid.UUID
	AgentUserID uuid.UUID
}

// OrderDecisionEvent is emitted when a vendor decides an order.
type OrderDecisionEvent struct {
	OrderID         uuid.UUID               `json:"order_id"`
	CheckoutGroupID uuid.UUID               `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID               `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID               `json:"vendor_store_id"`
	Decision        VendorOrderDecision     `json:"decision"`
	Status          enums.VendorOrderStatus `json:"status"`
}

// OrderFulfilledEvent surfaces the aggregated fields when fulfillment completes.
type OrderFulfilledEvent struct {
	OrderID            uuid.UUID                          `json:"order_id"`
	CheckoutGroupID    uuid.UUID                          `json:"checkout_group_id"`
	BuyerStoreID       uuid.UUID                          `json:"buyer_store_id"`
	VendorStoreID      uuid.UUID                          `json:"vendor_store_id"`
	FulfillmentStatus  enums.VendorOrderFulfillmentStatus `json:"fulfillment_status"`
	ShippingStatus     enums.VendorOrderShippingStatus    `json:"shipping_status"`
	RejectedItemCount  int                                `json:"rejected_item_count"`
	ResolvedLineItemID uuid.UUID                          `json:"resolved_line_item_id"`
}

// OrderCanceledEvent is emitted whenever a buyer cancels a pre-transit order.
type OrderCanceledEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	CheckoutGroupID uuid.UUID `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
}

// NotificationRequestedEvent tells downstream systems to alert a vendor.
type NotificationRequestedEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	CheckoutGroupID uuid.UUID `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
	Type            string    `json:"type"`
}

// OrderRetriedEvent reports that an expired order was replayed.
type OrderRetriedEvent struct {
	OriginalOrderID uuid.UUID `json:"original_order_id"`
	OrderID         uuid.UUID `json:"order_id"`
	CheckoutGroupID uuid.UUID `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
}

// NewService builds a vendor order service with the required dependencies.
func NewService(repo Repository, tx txRunner, outbox outboxPublisher, inventory InventoryReleaser, reserver inventoryReserver) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("orders repository required")
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if outbox == nil {
		return nil, fmt.Errorf("outbox publisher required")
	}
	if inventory == nil {
		return nil, fmt.Errorf("inventory releaser required")
	}
	if reserver == nil {
		return nil, fmt.Errorf("inventory reserver required")
	}
	return &service{
		repo:      repo,
		tx:        tx,
		outbox:    outbox,
		inventory: inventory,
		reserver:  reserver,
	}, nil
}

func (s *service) VendorDecision(ctx context.Context, input VendorDecisionInput) error {
	if input.OrderID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "order id required")
	}
	if input.ActorUserID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeUnauthorized, "user identity missing")
	}
	if input.ActorStoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}

	targetStatus, err := mapDecisionToStatus(input.Decision)
	if err != nil {
		return err
	}

	return s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		order, err := repo.FindVendorOrder(ctx, input.OrderID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "order not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load vendor order")
		}
		if order.VendorStoreID != input.ActorStoreID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store")
		}
		if order.Status == targetStatus {
			return nil
		}
		if order.Status != enums.VendorOrderStatusCreatedPending {
			return pkgerrors.New(pkgerrors.CodeStateConflict, "vendor decision not allowed in current state")
		}

		if err := repo.UpdateVendorOrderStatus(ctx, order.ID, targetStatus); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update order status")
		}

		order.Status = targetStatus
		event := outbox.DomainEvent{
			EventType:     enums.EventOrderDecided,
			AggregateType: enums.AggregateVendorOrder,
			AggregateID:   order.ID,
			Version:       1,
			Actor:         buildActor(input.ActorUserID, input.ActorStoreID, input.ActorRole),
			Data: OrderDecisionEvent{
				OrderID:         order.ID,
				CheckoutGroupID: order.CheckoutGroupID,
				BuyerStoreID:    order.BuyerStoreID,
				VendorStoreID:   order.VendorStoreID,
				Decision:        input.Decision,
				Status:          targetStatus,
			},
		}
		return s.outbox.Emit(ctx, tx, event)
	})
}

func (s *service) LineItemDecision(ctx context.Context, input LineItemDecisionInput) error {
	if input.OrderID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "order id required")
	}
	if input.LineItemID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "line item id required")
	}
	if input.ActorUserID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeUnauthorized, "user identity missing")
	}
	if input.ActorStoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}

	targetStatus, err := mapLineItemDecision(input.Decision)
	if err != nil {
		return err
	}

	return s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)

		order, err := repo.FindVendorOrder(ctx, input.OrderID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "order not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load vendor order")
		}
		if order.VendorStoreID != input.ActorStoreID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store")
		}

		lineItem, err := repo.FindOrderLineItem(ctx, input.LineItemID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "line item not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load line item")
		}
		if lineItem.OrderID != order.ID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "line item does not belong to order")
		}

		if lineItem.Status == targetStatus {
			return nil
		}
		if !canTransitionLineItemStatus(lineItem.Status) {
			return pkgerrors.New(pkgerrors.CodeStateConflict, "line item cannot be updated in current state")
		}

		if targetStatus == enums.LineItemStatusRejected && lineItem.ProductID != nil && lineItem.Qty > 0 {
			if err := s.inventory.Release(ctx, tx, *lineItem.ProductID, lineItem.Qty); err != nil {
				return err
			}
		}

		if err := repo.UpdateOrderLineItemStatus(ctx, lineItem.ID, targetStatus, input.Notes); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update line item status")
		}

		items, err := repo.FindOrderLineItemsByOrder(ctx, order.ID)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "reload line items")
		}

		subtotal := 0
		pending := 0
		rejected := 0
		for _, item := range items {
			if item.Status == enums.LineItemStatusPending {
				pending++
			}
			if item.Status == enums.LineItemStatusRejected {
				rejected++
				continue
			}
			subtotal += item.TotalCents
		}

		diff := order.TotalCents - order.SubtotalCents
		if diff < 0 {
			diff = 0
		}
		total := subtotal + diff
		if total < 0 {
			total = 0
		}
		balance := total

		updates := map[string]any{
			"subtotal_cents":    subtotal,
			"total_cents":       total,
			"balance_due_cents": balance,
		}

		var fulfillment enums.VendorOrderFulfillmentStatus
		if pending == 0 {
			if rejected > 0 {
				fulfillment = enums.VendorOrderFulfillmentStatusPartial
			} else {
				fulfillment = enums.VendorOrderFulfillmentStatusFulfilled
			}
			updates["fulfillment_status"] = fulfillment
			updates["status"] = enums.VendorOrderStatusHold
		}

		if err := repo.UpdateVendorOrder(ctx, order.ID, updates); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update order totals")
		}

		order.SubtotalCents = subtotal
		order.TotalCents = total
		order.BalanceDueCents = balance
		if pending == 0 {
			order.FulfillmentStatus = fulfillment
			order.Status = enums.VendorOrderStatusHold
		}

		if pending == 0 {
			event := outbox.DomainEvent{
				EventType:     enums.EventOrderFulfilled,
				AggregateType: enums.AggregateVendorOrder,
				AggregateID:   order.ID,
				Version:       1,
				Actor:         buildActor(input.ActorUserID, input.ActorStoreID, input.ActorRole),
				Data: OrderFulfilledEvent{
					OrderID:            order.ID,
					CheckoutGroupID:    order.CheckoutGroupID,
					BuyerStoreID:       order.BuyerStoreID,
					VendorStoreID:      order.VendorStoreID,
					FulfillmentStatus:  order.FulfillmentStatus,
					ShippingStatus:     order.ShippingStatus,
					RejectedItemCount:  rejected,
					ResolvedLineItemID: lineItem.ID,
				},
			}
			return s.outbox.Emit(ctx, tx, event)
		}

		return nil
	})
}

func (s *service) CancelOrder(ctx context.Context, input BuyerCancelInput) error {
	if input.OrderID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "order id required")
	}
	if input.ActorUserID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeUnauthorized, "user identity missing")
	}
	if input.ActorStoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}

	return s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		order, err := repo.FindVendorOrder(ctx, input.OrderID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "order not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load vendor order")
		}
		if order.BuyerStoreID != input.ActorStoreID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store")
		}
		if !isCancelableStatus(order.Status) {
			return pkgerrors.New(pkgerrors.CodeStateConflict, "order cannot be canceled in current state")
		}

		items, err := repo.FindOrderLineItemsByOrder(ctx, order.ID)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load order line items")
		}

		for _, item := range items {
			if item.Status == enums.LineItemStatusFulfilled {
				continue
			}
			if err := releaseLineItem(item, s.inventory, ctx, tx); err != nil {
				return err
			}
			if item.Status != enums.LineItemStatusRejected {
				if err := repo.UpdateOrderLineItemStatus(ctx, item.ID, enums.LineItemStatusRejected, nil); err != nil {
					return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update line item status")
				}
			}
		}

		updates := map[string]any{
			"status":            enums.VendorOrderStatusCanceled,
			"balance_due_cents": 0,
			"canceled_at":       time.Now().UTC(),
		}
		if err := repo.UpdateVendorOrder(ctx, order.ID, updates); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update vendor order")
		}

		event := outbox.DomainEvent{
			EventType:     enums.EventOrderCanceled,
			AggregateType: enums.AggregateVendorOrder,
			AggregateID:   order.ID,
			Version:       1,
			Actor:         buildActor(input.ActorUserID, input.ActorStoreID, input.ActorRole),
			Data: OrderCanceledEvent{
				OrderID:         order.ID,
				CheckoutGroupID: order.CheckoutGroupID,
				BuyerStoreID:    order.BuyerStoreID,
				VendorStoreID:   order.VendorStoreID,
			},
		}
		return s.outbox.Emit(ctx, tx, event)
	})
}

func (s *service) NudgeVendor(ctx context.Context, input BuyerNudgeInput) error {
	if input.OrderID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "order id required")
	}
	if input.ActorUserID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeUnauthorized, "user identity missing")
	}
	if input.ActorStoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}

	return s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		order, err := repo.FindVendorOrder(ctx, input.OrderID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "order not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load vendor order")
		}
		if order.BuyerStoreID != input.ActorStoreID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store")
		}
		if isFinalOrderStatus(order.Status) {
			return pkgerrors.New(pkgerrors.CodeStateConflict, "order cannot be nudged in current state")
		}

		event := outbox.DomainEvent{
			EventType:     enums.EventNotificationRequested,
			AggregateType: enums.AggregateVendorOrder,
			AggregateID:   order.ID,
			Version:       1,
			Actor:         buildActor(input.ActorUserID, input.ActorStoreID, input.ActorRole),
			Data: NotificationRequestedEvent{
				OrderID:         order.ID,
				CheckoutGroupID: order.CheckoutGroupID,
				BuyerStoreID:    order.BuyerStoreID,
				VendorStoreID:   order.VendorStoreID,
				Type:            "order_nudge",
			},
		}
		return s.outbox.Emit(ctx, tx, event)
	})
}

func (s *service) RetryOrder(ctx context.Context, input BuyerRetryInput) (*BuyerRetryResult, error) {
	if input.OrderID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "order id required")
	}
	if input.ActorUserID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, "user identity missing")
	}
	if input.ActorStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}

	var result *BuyerRetryResult
	err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		order, err := repo.FindVendorOrder(ctx, input.OrderID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "order not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load vendor order")
		}
		if order.BuyerStoreID != input.ActorStoreID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order does not belong to store")
		}
		if order.Status != enums.VendorOrderStatusExpired {
			return pkgerrors.New(pkgerrors.CodeStateConflict, "order retry only allowed for expired orders")
		}

		items, err := repo.FindOrderLineItemsByOrder(ctx, order.ID)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load order line items")
		}
		requests := make([]reservation.InventoryReservationRequest, 0, len(items))
		for _, item := range items {
			if item.ProductID != nil && item.Qty > 0 {
				requests = append(requests, reservation.InventoryReservationRequest{
					CartItemID: item.ID,
					ProductID:  *item.ProductID,
					Qty:        item.Qty,
				})
			}
		}

		group, err := repo.CreateCheckoutGroup(ctx, &models.CheckoutGroup{
			BuyerStoreID: order.BuyerStoreID,
		})
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create checkout group")
		}

		newOrder := &models.VendorOrder{
			CheckoutGroupID:   group.ID,
			BuyerStoreID:      order.BuyerStoreID,
			VendorStoreID:     order.VendorStoreID,
			SubtotalCents:     order.SubtotalCents,
			DiscountCents:     order.DiscountCents,
			TaxCents:          order.TaxCents,
			TransportFeeCents: order.TransportFeeCents,
			TotalCents:        order.TotalCents,
			BalanceDueCents:   order.TotalCents,
			Status:            enums.VendorOrderStatusCreatedPending,
			FulfillmentStatus: enums.VendorOrderFulfillmentStatusPending,
			ShippingStatus:    enums.VendorOrderShippingStatusPending,
			RefundStatus:      enums.RefundStatusNone,
		}
		createdOrder, err := repo.CreateVendorOrder(ctx, newOrder)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create vendor order")
		}

		newItems := make([]models.OrderLineItem, 0, len(items))
		for _, item := range items {
			newItems = append(newItems, models.OrderLineItem{
				OrderID:        createdOrder.ID,
				ProductID:      item.ProductID,
				Name:           item.Name,
				Category:       item.Category,
				Strain:         item.Strain,
				Classification: item.Classification,
				Unit:           item.Unit,
				UnitPriceCents: item.UnitPriceCents,
				Qty:            item.Qty,
				DiscountCents:  item.DiscountCents,
				TotalCents:     item.TotalCents,
				Status:         enums.LineItemStatusPending,
			})
		}
		if err := repo.CreateOrderLineItems(ctx, newItems); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create order line items")
		}

		if len(requests) > 0 {
			reserved, err := s.reserver.Reserve(ctx, tx, requests)
			if err != nil {
				return err
			}
			for _, res := range reserved {
				if !res.Reserved {
					return pkgerrors.New(pkgerrors.CodeConflict, "insufficient inventory for retry")
				}
			}
		}

		method := enums.PaymentMethodCash
		if origIntent, err := repo.FindPaymentIntentByOrder(ctx, order.ID); err == nil && origIntent != nil {
			method = origIntent.Method
		}
		if _, err := repo.CreatePaymentIntent(ctx, &models.PaymentIntent{
			OrderID:     createdOrder.ID,
			Method:      method,
			Status:      enums.PaymentStatusUnpaid,
			AmountCents: createdOrder.TotalCents,
		}); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create payment intent")
		}

		event := outbox.DomainEvent{
			EventType:     enums.EventOrderRetried,
			AggregateType: enums.AggregateVendorOrder,
			AggregateID:   createdOrder.ID,
			Version:       1,
			Actor:         buildActor(input.ActorUserID, input.ActorStoreID, input.ActorRole),
			Data: OrderRetriedEvent{
				OriginalOrderID: order.ID,
				OrderID:         createdOrder.ID,
				CheckoutGroupID: createdOrder.CheckoutGroupID,
				BuyerStoreID:    createdOrder.BuyerStoreID,
				VendorStoreID:   createdOrder.VendorStoreID,
			},
		}
		if err := s.outbox.Emit(ctx, tx, event); err != nil {
			return err
		}

		result = &BuyerRetryResult{OrderID: createdOrder.ID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) AgentPickup(ctx context.Context, input AgentPickupInput) error {
	if input.OrderID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "order id required")
	}
	if input.AgentUserID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeUnauthorized, "agent identity missing")
	}

	return s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		detail, err := repo.FindOrderDetail(ctx, input.OrderID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "order not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load order detail")
		}
		if detail == nil || detail.ActiveAssignment == nil || detail.Order == nil {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order not assigned to agent")
		}
		if detail.ActiveAssignment.AgentUserID != input.AgentUserID {
			return pkgerrors.New(pkgerrors.CodeForbidden, "order not assigned to agent")
		}
		status := detail.Order.Status
		if status != enums.VendorOrderStatusHold &&
			status != enums.VendorOrderStatusHoldForPickup &&
			status != enums.VendorOrderStatusInTransit {
			return pkgerrors.New(pkgerrors.CodeStateConflict, "order cannot be picked up in current state")
		}

		now := time.Now().UTC()
		orderUpdates := map[string]any{}
		if status != enums.VendorOrderStatusInTransit {
			orderUpdates["status"] = enums.VendorOrderStatusInTransit
		}
		if detail.Order.ShippingStatus != enums.VendorOrderShippingStatusInTransit {
			orderUpdates["shipping_status"] = enums.VendorOrderShippingStatusInTransit
		}
		if len(orderUpdates) > 0 {
			if err := repo.UpdateVendorOrder(ctx, input.OrderID, orderUpdates); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update order status")
			}
		}

		assignUpdates := map[string]any{}
		if detail.ActiveAssignment.PickupTime == nil {
			assignUpdates["pickup_time"] = now
		}
		if len(assignUpdates) > 0 {
			if err := repo.UpdateOrderAssignment(ctx, detail.ActiveAssignment.ID, assignUpdates); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update order assignment")
			}
		}

		return nil
	})
}

func mapDecisionToStatus(decision VendorOrderDecision) (enums.VendorOrderStatus, error) {
	switch decision {
	case VendorOrderDecisionAccept:
		return enums.VendorOrderStatusAccepted, nil
	case VendorOrderDecisionReject:
		return enums.VendorOrderStatusRejected, nil
	default:
		return "", pkgerrors.New(pkgerrors.CodeValidation, "invalid decision")
	}
}

func mapLineItemDecision(decision LineItemDecision) (enums.LineItemStatus, error) {
	switch decision {
	case LineItemDecisionFulfill:
		return enums.LineItemStatusFulfilled, nil
	case LineItemDecisionReject:
		return enums.LineItemStatusRejected, nil
	default:
		return "", pkgerrors.New(pkgerrors.CodeValidation, "line item decision must be fulfill or reject")
	}
}

func canTransitionLineItemStatus(current enums.LineItemStatus) bool {
	switch current {
	case enums.LineItemStatusPending, enums.LineItemStatusAccepted, enums.LineItemStatusHold:
		return true
	default:
		return false
	}
}

func buildActor(userID, storeID uuid.UUID, role string) *outbox.ActorRef {
	store := storeID
	return &outbox.ActorRef{
		UserID:  userID,
		StoreID: &store,
		Role:    role,
	}
}

func releaseLineItem(item models.OrderLineItem, releaser InventoryReleaser, ctx context.Context, tx *gorm.DB) error {
	if item.ProductID == nil || item.Qty <= 0 {
		return nil
	}
	if err := releaser.Release(ctx, tx, *item.ProductID, item.Qty); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "release inventory")
	}
	return nil
}

func isCancelableStatus(status enums.VendorOrderStatus) bool {
	return !isFinalOrderStatus(status)
}

func isFinalOrderStatus(status enums.VendorOrderStatus) bool {
	switch status {
	case enums.VendorOrderStatusInTransit, enums.VendorOrderStatusDelivered, enums.VendorOrderStatusClosed, enums.VendorOrderStatusCanceled, enums.VendorOrderStatusExpired:
		return true
	default:
		return false
	}
}

type inventoryReleaserImpl struct{}

// NewInventoryReleaser exposes the default inventory release implementation.
func NewInventoryReleaser() InventoryReleaser {
	return inventoryReleaserImpl{}
}

func (inventoryReleaserImpl) Release(ctx context.Context, tx *gorm.DB, productID uuid.UUID, qty int) error {
	if qty <= 0 {
		return nil
	}
	if tx == nil {
		return pkgerrors.New(pkgerrors.CodeDependency, "transaction required for inventory release")
	}

	res := tx.WithContext(ctx).Exec(`
		UPDATE inventory_items
		SET available_qty = available_qty + ?,
			reserved_qty = reserved_qty - ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE product_id = ? AND reserved_qty >= ?
	`, qty, qty, productID, qty)
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, res.Error, "release inventory")
	}
	return nil
}

type inventoryReserverImpl struct{}

// NewInventoryReserver exposes the default inventory reservation helper.
func NewInventoryReserver() inventoryReserver {
	return inventoryReserverImpl{}
}

func (inventoryReserverImpl) Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	return reservation.ReserveInventory(ctx, tx, requests)
}
