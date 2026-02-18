package media

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
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
	return &service{
		repo:      repo,
		bucket:    "bucket",
		uploadTTL: time.Minute,
	}
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
			Status:    enums.MediaStatusUploaded,
			GCSKey:    "media/one",
			PublicURL: "https://public.example/one",
		},
		{
			ID:        uuid.New(),
			CreatedAt: now.Add(-time.Minute),
			Status:    enums.MediaStatusUploaded,
			GCSKey:    "media/two",
			PublicURL: "https://public.example/two",
		},
	}
	repo := &stubListRepo{listRows: rows}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	params := ListParams{
		StoreID: storeID,
		Params:  pagination.Params{Limit: 1},
	}
	res, err := svc.ListMedia(context.Background(), params)
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].SignedURL == nil || *res.Items[0].SignedURL != rows[0].PublicURL {
		t.Fatalf("expected signed url %s got %v", rows[0].PublicURL, res.Items[0].SignedURL)
	}
	if res.Cursor == "" {
		t.Fatal("expected cursor for next page")
	}
	expected := pagination.EncodeCursor(pagination.Cursor{
		CreatedAt: rows[1].CreatedAt,
		ID:        rows[1].ID,
	})
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
		Params:  pagination.Params{Limit: pagination.MaxLimit + 50},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastQuery.limit != pagination.MaxLimit+1 {
		t.Fatalf("expected limit %d got %d", pagination.MaxLimit+1, repo.lastQuery.limit)
	}
}

func TestListMediaInvalidCursor(t *testing.T) {
	repo := &stubListRepo{}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	if _, err := svc.ListMedia(context.Background(), ListParams{
		StoreID: storeID,
		Params:  pagination.Params{Cursor: "badcursor"},
	}); err == nil {
		t.Fatal("expected error for invalid cursor")
	} else if pkgerrors.As(err).Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestListMediaSignedURLForReadableStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status enums.MediaStatus
	}{
		{name: "uploaded", status: enums.MediaStatusUploaded},
		{name: "ready", status: enums.MediaStatusReady},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &stubListRepo{
				listRows: []models.Media{
					{
						ID:        uuid.New(),
						Status:    tc.status,
						GCSKey:    "media/readable",
						PublicURL: "https://public.example/read",
					},
				},
			}
			svc := newServiceWithRepo(repo)
			storeID := uuid.New()

			resp, err := svc.ListMedia(context.Background(), ListParams{StoreID: storeID})
			if err != nil {
				t.Fatalf("ListMedia returned error: %v", err)
			}
			if resp.Items[0].SignedURL == nil || *resp.Items[0].SignedURL != repo.listRows[0].PublicURL {
				t.Fatalf("expected signed url %s got %v", repo.listRows[0].PublicURL, resp.Items[0].SignedURL)
			}
		})
	}
}

func TestListMediaSkipsSignedURLForUnreadableStatus(t *testing.T) {
	repo := &stubListRepo{
		listRows: []models.Media{
			{
				ID:     uuid.New(),
				Status: enums.MediaStatusPending,
				GCSKey: "media/pending",
			},
		},
	}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	resp, err := svc.ListMedia(context.Background(), ListParams{StoreID: storeID})
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if resp.Items[0].SignedURL != nil {
		t.Fatalf("expected no signed url for unreadable media, got %v", resp.Items[0].SignedURL)
	}
}

func TestListMediaSignedURLOnlyForReturnedRows(t *testing.T) {
	rows := []models.Media{
		{
			ID:        uuid.New(),
			Status:    enums.MediaStatusUploaded,
			GCSKey:    "media/a",
			PublicURL: "https://public.example/a",
		},
		{
			ID:        uuid.New(),
			Status:    enums.MediaStatusUploaded,
			GCSKey:    "media/b",
			PublicURL: "https://public.example/b",
		},
	}
	repo := &stubListRepo{listRows: rows}
	svc := newServiceWithRepo(repo)
	storeID := uuid.New()

	resp, err := svc.ListMedia(context.Background(), ListParams{
		StoreID: storeID,
		Params:  pagination.Params{Limit: 1},
	})
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].SignedURL == nil || *resp.Items[0].SignedURL != rows[0].PublicURL {
		t.Fatalf("expected signed url %s got %v", rows[0].PublicURL, resp.Items[0].SignedURL)
	}
}
