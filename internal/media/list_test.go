package media

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
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

func newServiceWithRepo(repo mediaRepository) (*service, *stubGCS) {
	gcs := &stubGCS{readURL: "https://download.example"}
	return &service{
		repo:        repo,
		gcs:         gcs,
		bucket:      "bucket",
		downloadTTL: time.Minute,
	}, gcs
}

func TestListMediaInvalidStore(t *testing.T) {
	svc, _ := newServiceWithRepo(&stubListRepo{})
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
		},
		{
			ID:        uuid.New(),
			CreatedAt: now.Add(-time.Minute),
			Status:    enums.MediaStatusUploaded,
			GCSKey:    "media/two",
		},
	}
	repo := &stubListRepo{listRows: rows}
	svc, gcs := newServiceWithRepo(repo)
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
	if res.Items[0].SignedURL != gcs.readURL {
		t.Fatalf("expected signed url %s got %s", gcs.readURL, res.Items[0].SignedURL)
	}
	if gcs.readCalls != 1 {
		t.Fatalf("expected one signed url request, got %d", gcs.readCalls)
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
	svc, _ := newServiceWithRepo(repo)
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
	svc, _ := newServiceWithRepo(repo)
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

func TestListMediaSignedURLForReadableStatus(t *testing.T) {
	repo := &stubListRepo{
		listRows: []models.Media{
			{
				ID:     uuid.New(),
				Status: enums.MediaStatusUploaded,
				GCSKey: "media/ready",
			},
		},
	}
	svc, gcs := newServiceWithRepo(repo)
	storeID := uuid.New()

	resp, err := svc.ListMedia(context.Background(), ListParams{StoreID: storeID})
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if resp.Items[0].SignedURL != gcs.readURL {
		t.Fatalf("expected signed url %s got %s", gcs.readURL, resp.Items[0].SignedURL)
	}
	if gcs.readCalls != 1 {
		t.Fatalf("expected one signed url request, got %d", gcs.readCalls)
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
	svc, gcs := newServiceWithRepo(repo)
	storeID := uuid.New()

	resp, err := svc.ListMedia(context.Background(), ListParams{StoreID: storeID})
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if resp.Items[0].SignedURL != "" {
		t.Fatalf("expected empty signed url for unreadable media, got %s", resp.Items[0].SignedURL)
	}
	if gcs.readCalls != 0 {
		t.Fatalf("expected no signed url requests, got %d", gcs.readCalls)
	}
}

func TestListMediaSignedURLOnlyForReturnedRows(t *testing.T) {
	rows := []models.Media{
		{
			ID:     uuid.New(),
			Status: enums.MediaStatusUploaded,
			GCSKey: "media/a",
		},
		{
			ID:     uuid.New(),
			Status: enums.MediaStatusUploaded,
			GCSKey: "media/b",
		},
	}
	repo := &stubListRepo{listRows: rows}
	svc, gcs := newServiceWithRepo(repo)
	storeID := uuid.New()

	resp, err := svc.ListMedia(context.Background(), ListParams{
		StoreID: storeID,
		Limit:   1,
	})
	if err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if gcs.readCalls != 1 {
		t.Fatalf("expected 1 signed url request, got %d", gcs.readCalls)
	}
}
