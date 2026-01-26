package orders

import (
	"context"
	"fmt"

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

// Service defines order-level operations beyond repository reads.
type Service interface {
	VendorDecision(ctx context.Context, input VendorDecisionInput) error
}

type service struct {
	repo   Repository
	tx     txRunner
	outbox outboxPublisher
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

// OrderDecisionEvent is emitted when a vendor decides an order.
type OrderDecisionEvent struct {
	OrderID         uuid.UUID               `json:"order_id"`
	CheckoutGroupID uuid.UUID               `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID               `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID               `json:"vendor_store_id"`
	Decision        VendorOrderDecision     `json:"decision"`
	Status          enums.VendorOrderStatus `json:"status"`
}

// NewService builds a vendor order service with the required dependencies.
func NewService(repo Repository, tx txRunner, outbox outboxPublisher) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("orders repository required")
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if outbox == nil {
		return nil, fmt.Errorf("outbox publisher required")
	}
	return &service{
		repo:   repo,
		tx:     tx,
		outbox: outbox,
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
			Actor:         buildActor(input),
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

func buildActor(input VendorDecisionInput) *outbox.ActorRef {
	storeID := input.ActorStoreID
	return &outbox.ActorRef{
		UserID:  input.ActorUserID,
		StoreID: &storeID,
		Role:    input.ActorRole,
	}
}
