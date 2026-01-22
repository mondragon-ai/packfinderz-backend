package media

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

type stubListRepo struct {
	stubMediaRepo
	listRows  []models.Media
	listErr   error
	lastQuery listQuery
}

func (s *stubListRepo) List(ctx context.Context, opts listQuery) ([]models.Media, error) {
	s.lastQuery = opts
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listRows, nil
}

func newServiceWithRepo(repo mediaRepository) *service {
	return &service{repo: repo}
}

func TestListMediaInvalidStore(t *testing.T) {
	svc := newServiceWithRepo(&stubListRepo{})
	if _, err := svc.ListMedia(context.Background(), ListParams{}); err == nil {
		t.Fatal("expected error when store id missing")
	}
}

func TestListMediaCursorPagination(t *testing.T) {
	now := time.Now()
	rows := []models.Media{
		{
			ID:        uuid.New(),
			CreatedAt: now,
		},
		{
			ID:        uuid.New(),
			CreatedAt: now.Add(-time.Minute),
		},
	}
	repo := &stubListRepo{listRows: rows}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	params := ListParams{
		StoreID: storeID,
		Limit:   1,
	}
	res, err := svc.ListMedia(context.Background(), params)
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Cursor == "" {
		t.Fatal("expected cursor for next page")
	}
	expected := encodeListCursor(&rows[1])
	if res.Cursor != expected {
		t.Fatalf("expected cursor %s got %s", expected, res.Cursor)
	}
}

func TestListMediaLimitClamped(t *testing.T) {
	repo := &stubListRepo{}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	if _, err := svc.ListMedia(context.Background(), ListParams{
		StoreID: storeID,
		Limit:   maxMediaListLimit + 50,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastQuery.limit != maxMediaListLimit+1 {
		t.Fatalf("expected limit %d got %d", maxMediaListLimit+1, repo.lastQuery.limit)
	}
}

func TestListMediaInvalidCursor(t *testing.T) {
	repo := &stubListRepo{}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	if _, err := svc.ListMedia(context.Background(), ListParams{
		StoreID: storeID,
		Cursor:  "badcursor",
	}); err == nil {
		t.Fatal("expected error for invalid cursor")
	} else if pkgerrors.As(err).Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}
