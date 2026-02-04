package ledger

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// Service defines operations that record ledger events.
type Service interface {
	RecordEvent(ctx context.Context, input RecordLedgerEventInput) (*models.LedgerEvent, error)
	HasEvent(ctx context.Context, orderID uuid.UUID, eventType enums.LedgerEventType) (bool, error)
}

type service struct {
	repo Repository
}

// RecordLedgerEventInput captures the immutable data a ledger event requires.
type RecordLedgerEventInput struct {
	OrderID       uuid.UUID             `json:"order_id"`
	BuyerStoreID  uuid.UUID             `json:"buyer_store_id"`
	VendorStoreID uuid.UUID             `json:"vendor_store_id"`
	ActorUserID   uuid.UUID             `json:"actor_user_id"`
	Type          enums.LedgerEventType `json:"type"`
	AmountCents   int                   `json:"amount_cents"`
	Metadata      json.RawMessage       `json:"metadata"`
}

// NewService wires a ledger service with the provided repository.
func NewService(repo Repository) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("ledger repository required")
	}
	return &service{repo: repo}, nil
}

func (s *service) RecordEvent(ctx context.Context, input RecordLedgerEventInput) (*models.LedgerEvent, error) {
	if input.OrderID == uuid.Nil {
		return nil, fmt.Errorf("order id is required")
	}
	if input.BuyerStoreID == uuid.Nil {
		return nil, fmt.Errorf("buyer store id is required")
	}
	if input.VendorStoreID == uuid.Nil {
		return nil, fmt.Errorf("vendor store id is required")
	}
	if input.ActorUserID == uuid.Nil {
		return nil, fmt.Errorf("actor user id is required")
	}
	if !input.Type.IsValid() {
		return nil, fmt.Errorf("invalid ledger event type %q", input.Type)
	}

	event := &models.LedgerEvent{
		OrderID:       input.OrderID,
		BuyerStoreID:  input.BuyerStoreID,
		VendorStoreID: input.VendorStoreID,
		ActorUserID:   input.ActorUserID,
		Type:          input.Type,
		AmountCents:   input.AmountCents,
		Metadata:      input.Metadata,
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *service) HasEvent(ctx context.Context, orderID uuid.UUID, eventType enums.LedgerEventType) (bool, error) {
	if orderID == uuid.Nil {
		return false, fmt.Errorf("order id is required")
	}
	if !eventType.IsValid() {
		return false, fmt.Errorf("invalid ledger event type %q", eventType)
	}

	events, err := s.repo.ListByOrderID(ctx, orderID)
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Type == eventType {
			return true, nil
		}
	}
	return false, nil
}
