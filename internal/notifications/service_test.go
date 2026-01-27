package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	paginationpkg "github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRepository struct {
	listFn        func(ctx context.Context, params listNotificationsParams) ([]models.Notification, *paginationpkg.Cursor, error)
	markReadFn    func(ctx context.Context, storeID, notificationID uuid.UUID, now time.Time) (notificationMarkResult, error)
	markAllReadFn func(ctx context.Context, storeID uuid.UUID, now time.Time) (int64, error)
}

func (f *fakeRepository) WithTx(tx *gorm.DB) Repository {
	return f
}

func (f *fakeRepository) Create(ctx context.Context, notification *models.Notification) error {
	return nil
}

func (f *fakeRepository) List(ctx context.Context, params listNotificationsParams) ([]models.Notification, *paginationpkg.Cursor, error) {
	if f.listFn != nil {
		return f.listFn(ctx, params)
	}
	return nil, nil, nil
}

func (f *fakeRepository) MarkRead(ctx context.Context, storeID, notificationID uuid.UUID, now time.Time) (notificationMarkResult, error) {
	if f.markReadFn != nil {
		return f.markReadFn(ctx, storeID, notificationID, now)
	}
	return notificationMarkResult{}, nil
}

func (f *fakeRepository) MarkAllRead(ctx context.Context, storeID uuid.UUID, now time.Time) (int64, error) {
	if f.markAllReadFn != nil {
		return f.markAllReadFn(ctx, storeID, now)
	}
	return 0, nil
}

func newServiceWithRepo(repo Repository) Service {
	svc, _ := NewService(repo)
	return svc
}

func TestService_ListNotifications(t *testing.T) {
	first := models.Notification{ID: uuid.New(), CreatedAt: time.Now().Add(-time.Hour)}
	second := models.Notification{ID: uuid.New(), CreatedAt: time.Now()}

	repo := &fakeRepository{
		listFn: func(ctx context.Context, params listNotificationsParams) ([]models.Notification, *paginationpkg.Cursor, error) {
			if params.Limit != paginationpkg.LimitWithBuffer(1) {
				t.Fatalf("unexpected limit %d", params.Limit)
			}
			return []models.Notification{first}, &paginationpkg.Cursor{CreatedAt: second.CreatedAt, ID: second.ID}, nil
		},
	}

	svc := newServiceWithRepo(repo)
	result, err := svc.List(context.Background(), ListParams{StoreID: uuid.New(), Limit: 1})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(result.Items))
	}
	if result.Cursor == "" {
		t.Fatal("expected cursor for next page")
	}
	decoded, err := paginationpkg.ParseCursor(result.Cursor)
	if err != nil {
		t.Fatalf("invalid cursor %q: %v", result.Cursor, err)
	}
	if decoded.ID != second.ID {
		t.Fatalf("expected cursor id %s got %s", second.ID, decoded.ID)
	}
}

func TestService_ListNotificationsInvalidCursor(t *testing.T) {
	svc := newServiceWithRepo(&fakeRepository{})
	_, err := svc.List(context.Background(), ListParams{StoreID: uuid.New(), Cursor: "bad"})
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
	errCode := pkgerrors.As(err).Code()
	if errCode != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %s", errCode)
	}
}

func TestService_MarkRead(t *testing.T) {
	repo := &fakeRepository{
		markReadFn: func(ctx context.Context, storeID, notificationID uuid.UUID, now time.Time) (notificationMarkResult, error) {
			return notificationMarkResult{Found: true, Updated: true}, nil
		},
	}
	svc := newServiceWithRepo(repo)
	if err := svc.MarkRead(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected mark read error: %v", err)
	}
}

func TestService_MarkReadNotFound(t *testing.T) {
	repo := &fakeRepository{
		markReadFn: func(ctx context.Context, storeID, notificationID uuid.UUID, now time.Time) (notificationMarkResult, error) {
			return notificationMarkResult{Found: false}, nil
		},
	}
	svc := newServiceWithRepo(repo)
	if err := svc.MarkRead(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected not found error")
	} else if pkgerrors.As(err).Code() != pkgerrors.CodeNotFound {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestService_MarkAllRead(t *testing.T) {
	repo := &fakeRepository{
		markAllReadFn: func(ctx context.Context, storeID uuid.UUID, now time.Time) (int64, error) {
			return 3, nil
		},
	}
	svc := newServiceWithRepo(repo)
	count, err := svc.MarkAllRead(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected mark all read error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 updated rows, got %d", count)
	}
}

func TestService_MarkAllReadError(t *testing.T) {
	repo := &fakeRepository{
		markAllReadFn: func(ctx context.Context, storeID uuid.UUID, now time.Time) (int64, error) {
			return 0, errors.New("boom")
		},
	}
	svc := newServiceWithRepo(repo)
	if _, err := svc.MarkAllRead(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}
