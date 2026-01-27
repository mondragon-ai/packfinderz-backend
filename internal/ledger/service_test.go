package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRepository struct {
	createFn func(ctx context.Context, event *models.LedgerEvent) error
}

func (f *fakeRepository) WithTx(tx *gorm.DB) Repository {
	return f
}

func (f *fakeRepository) Create(ctx context.Context, event *models.LedgerEvent) error {
	if f.createFn != nil {
		return f.createFn(ctx, event)
	}
	return nil
}

func (f *fakeRepository) ListByOrderID(ctx context.Context, orderID uuid.UUID) ([]models.LedgerEvent, error) {
	return nil, nil
}

func TestService_RecordEvent(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("unexpected service error: %v", err)
	}

	metadata := json.RawMessage(`{"note":"collected"}`)
	input := RecordLedgerEventInput{
		OrderID:       uuid.New(),
		BuyerStoreID:  uuid.New(),
		VendorStoreID: uuid.New(),
		ActorUserID:   uuid.New(),
		Type:          enums.LedgerEventTypeCashCollected,
		AmountCents:   425000,
		Metadata:      metadata,
	}

	var created *models.LedgerEvent
	repo.createFn = func(ctx context.Context, event *models.LedgerEvent) error {
		created = event
		return nil
	}

	got, err := svc.RecordEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("RecordEvent error: %v", err)
	}
	if created == nil {
		t.Fatal("expected ledger event to be created")
	}
	if created.OrderID != input.OrderID || created.Type != input.Type || created.AmountCents != input.AmountCents {
		t.Fatalf("unexpected ledger event data: %v", created)
	}
	if created.BuyerStoreID != input.BuyerStoreID || created.VendorStoreID != input.VendorStoreID || created.ActorUserID != input.ActorUserID {
		t.Fatalf("missing store/actor metadata: %+v", created)
	}
	if string(created.Metadata) != string(metadata) {
		t.Fatalf("metadata mismatch: %s", created.Metadata)
	}
	if got != created {
		t.Fatalf("service should return created event")
	}
}

func TestService_RecordEventValidation(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("unexpected service error: %v", err)
	}

	tests := []struct {
		name  string
		input RecordLedgerEventInput
	}{
		{
			name: "missing order id",
			input: RecordLedgerEventInput{
				OrderID:       uuid.Nil,
				BuyerStoreID:  uuid.New(),
				VendorStoreID: uuid.New(),
				ActorUserID:   uuid.New(),
				Type:          enums.LedgerEventTypeCashCollected,
			},
		},
		{
			name: "missing buyer store",
			input: RecordLedgerEventInput{
				OrderID:       uuid.New(),
				VendorStoreID: uuid.New(),
				ActorUserID:   uuid.New(),
				Type:          enums.LedgerEventTypeCashCollected,
			},
		},
		{
			name: "missing vendor store",
			input: RecordLedgerEventInput{
				OrderID:      uuid.New(),
				BuyerStoreID: uuid.New(),
				ActorUserID:  uuid.New(),
				Type:         enums.LedgerEventTypeCashCollected,
			},
		},
		{
			name: "missing actor",
			input: RecordLedgerEventInput{
				OrderID:       uuid.New(),
				BuyerStoreID:  uuid.New(),
				VendorStoreID: uuid.New(),
				Type:          enums.LedgerEventTypeCashCollected,
			},
		},
		{
			name: "invalid type",
			input: RecordLedgerEventInput{
				OrderID:       uuid.New(),
				BuyerStoreID:  uuid.New(),
				VendorStoreID: uuid.New(),
				ActorUserID:   uuid.New(),
				Type:          enums.LedgerEventType("not_real"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.RecordEvent(context.Background(), tc.input); err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestService_RecordEventRepoError(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("unexpected service error: %v", err)
	}

	expectedErr := errors.New("boom")
	repo.createFn = func(ctx context.Context, event *models.LedgerEvent) error {
		return expectedErr
	}

	if _, err := svc.RecordEvent(context.Background(), RecordLedgerEventInput{
		OrderID:       uuid.New(),
		BuyerStoreID:  uuid.New(),
		VendorStoreID: uuid.New(),
		ActorUserID:   uuid.New(),
		Type:          enums.LedgerEventTypeVendorPayout,
		AmountCents:   100,
	}); !errors.Is(err, expectedErr) {
		t.Fatalf("expected repo error to bubble up, got %v", err)
	}
}
