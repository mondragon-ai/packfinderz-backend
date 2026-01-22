package media

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubMediaRepo struct {
	created     *models.Media
	deleteID    uuid.UUID
	createErr   error
	deleteErr   error
	findMedia   *models.Media
	findErr     error
	markDeleted bool
	deletedAt   time.Time
	markErr     error
}

func (s *stubMediaRepo) Create(ctx context.Context, media *models.Media) (*models.Media, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.created = media
	return media, nil
}

func (s *stubMediaRepo) Delete(ctx context.Context, id uuid.UUID) error {
	s.deleteID = id
	return s.deleteErr
}

func (s *stubMediaRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.findMedia == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.findMedia, nil
}

func (s *stubMediaRepo) List(ctx context.Context, opts listQuery) ([]models.Media, error) {
	return nil, nil
}

func (s *stubMediaRepo) MarkDeleted(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	s.markDeleted = true
	s.deletedAt = deletedAt
	if s.markErr != nil {
		return s.markErr
	}
	return nil
}

type stubMemberships struct {
	ok  bool
	err error
}

func (s stubMemberships) UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.ok, nil
}

type stubGCS struct {
	url          string
	err          error
	lastBucket   string
	lastObject   string
	lastMimeType string
	readURL      string
	readErr      error
	readCalls    int
	deleteCalled bool
	deleteErr    error
}

func (s *stubGCS) SignedURL(bucket, object, contentType string, expires time.Duration) (string, error) {
	s.lastBucket = bucket
	s.lastObject = object
	s.lastMimeType = contentType
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func (s *stubGCS) SignedReadURL(bucket, object string, expires time.Duration) (string, error) {
	s.readCalls++
	if s.readErr != nil {
		return "", s.readErr
	}
	s.lastBucket = bucket
	s.lastObject = object
	if s.readURL == "" {
		return "https://download.example", nil
	}
	return s.readURL, nil
}

func (s *stubGCS) DeleteObject(ctx context.Context, bucket, object string) error {
	s.deleteCalled = true
	s.lastBucket = bucket
	s.lastObject = object
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return nil
}

func TestMediaServicePresignSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{url: "https://signed.example"}

	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	userID := uuid.New()
	storeID := uuid.New()
	input := PresignInput{
		Kind:      enums.MediaKindProduct,
		MimeType:  "image/png",
		FileName:  "photo.png",
		SizeBytes: 1024,
	}

	res, err := svc.PresignUpload(context.Background(), userID, storeID, input)
	if err != nil {
		t.Fatalf("PresignUpload returned error: %v", err)
	}
	if res.SignedPUTURL != gcs.url {
		t.Fatalf("unexpected signed url %s", res.SignedPUTURL)
	}
	if res.ContentType != "image/png" {
		t.Fatalf("unexpected content type %s", res.ContentType)
	}
	if repo.created == nil {
		t.Fatal("expected media created")
	}
	if res.MediaID != repo.created.ID {
		t.Fatalf("expected media id %s got %s", repo.created.ID, res.MediaID)
	}
	if !contains(res.GCSKey, res.MediaID.String()) {
		t.Fatalf("gcs key %s missing media id", res.GCSKey)
	}
	if gcs.lastBucket != "bucket" || gcs.lastObject != res.GCSKey || gcs.lastMimeType != "image/png" {
		t.Fatalf("unexpected gcs call %v", gcs)
	}
}

func TestMediaServicePresignValidation(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{url: "ok"}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	cases := []struct {
		name  string
		input PresignInput
	}{
		{
			name: "size too large",
			input: PresignInput{
				Kind:      enums.MediaKindProduct,
				MimeType:  "image/png",
				FileName:  "file.png",
				SizeBytes: maxUploadBytes + 1,
			},
		},
		{
			name: "invalid mime for kind",
			input: PresignInput{
				Kind:      enums.MediaKindPDF,
				MimeType:  "image/png",
				FileName:  "doc.pdf",
				SizeBytes: 1024,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.PresignUpload(context.Background(), uuid.New(), uuid.New(), tc.input)
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
			if pkgerrors.As(err).Code() != pkgerrors.CodeValidation {
				t.Fatalf("expected validation code got %v", pkgerrors.As(err).Code())
			}
		})
	}
}

func TestMediaServicePresignForbidden(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{}
	members := stubMemberships{ok: false}
	gcs := &stubGCS{}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.PresignUpload(context.Background(), uuid.New(), uuid.New(), PresignInput{
		Kind:      enums.MediaKindProduct,
		MimeType:  "image/png",
		FileName:  "x.png",
		SizeBytes: 100,
	})
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	if pkgerrors.As(err).Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code got %v", pkgerrors.As(err).Code())
	}
}

func TestMediaServiceGenerateReadURLSuccess(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	mediaID := uuid.New()
	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      mediaID,
			StoreID: storeID,
			Status:  enums.MediaStatusUploaded,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{readURL: "https://download.example"}

	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := svc.GenerateReadURL(context.Background(), ReadURLParams{
		StoreID: storeID,
		MediaID: mediaID,
	})
	if err != nil {
		t.Fatalf("GenerateReadURL returned error: %v", err)
	}
	if resp.URL != "https://download.example" {
		t.Fatalf("unexpected url %s", resp.URL)
	}
	if resp.ExpiresAt.Before(time.Now().Add(15*time.Minute - time.Second)) {
		t.Fatalf("expires at too soon: %s", resp.ExpiresAt)
	}
	if gcs.lastBucket != "bucket" || gcs.lastObject != "media/key" {
		t.Fatalf("unexpected gcs call %v", gcs)
	}
}

func TestMediaServiceGenerateReadURLStoreMismatch(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      uuid.New(),
			StoreID: uuid.New(),
			Status:  enums.MediaStatusUploaded,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := svc.GenerateReadURL(context.Background(), ReadURLParams{
		StoreID: uuid.New(),
		MediaID: repo.findMedia.ID,
	}); err == nil {
		t.Fatal("expected store mismatch error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestMediaServiceGenerateReadURLInvalidStatus(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      uuid.New(),
			StoreID: uuid.New(),
			Status:  enums.MediaStatusPending,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := svc.GenerateReadURL(context.Background(), ReadURLParams{
		StoreID: repo.findMedia.StoreID,
		MediaID: repo.findMedia.ID,
	}); err == nil {
		t.Fatal("expected conflict error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestMediaServiceGenerateReadURLDependencyError(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      uuid.New(),
			StoreID: uuid.New(),
			Status:  enums.MediaStatusUploaded,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{readErr: errors.New("boom")}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := svc.GenerateReadURL(context.Background(), ReadURLParams{
		StoreID: repo.findMedia.StoreID,
		MediaID: repo.findMedia.ID,
	}); err == nil {
		t.Fatal("expected dependency error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeDependency {
		t.Fatalf("expected dependency code, got %v", err)
	}
}

func TestMediaServiceDeleteSuccess(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	mediaID := uuid.New()
	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      mediaID,
			StoreID: storeID,
			Status:  enums.MediaStatusUploaded,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{}

	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := svc.DeleteMedia(context.Background(), DeleteMediaParams{
		StoreID: storeID,
		MediaID: mediaID,
	}); err != nil {
		t.Fatalf("DeleteMedia returned error: %v", err)
	}
	if !gcs.deleteCalled {
		t.Fatal("expected gcs delete called")
	}
	if !repo.markDeleted {
		t.Fatal("expected repo.MarkDeleted called")
	}
}

func TestMediaServiceDeleteStoreMismatch(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      uuid.New(),
			StoreID: uuid.New(),
			Status:  enums.MediaStatusUploaded,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := svc.DeleteMedia(context.Background(), DeleteMediaParams{
		StoreID: uuid.New(),
		MediaID: repo.findMedia.ID,
	}); err == nil {
		t.Fatal("expected forbidden error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestMediaServiceDeleteInvalidStatus(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      uuid.New(),
			StoreID: storeID,
			Status:  enums.MediaStatusPending,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := svc.DeleteMedia(context.Background(), DeleteMediaParams{
		StoreID: storeID,
		MediaID: repo.findMedia.ID,
	}); err == nil {
		t.Fatal("expected conflict error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestMediaServiceDeleteDependencyError(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	mediaID := uuid.New()
	repo := &stubMediaRepo{
		findMedia: &models.Media{
			ID:      mediaID,
			StoreID: storeID,
			Status:  enums.MediaStatusUploaded,
			GCSKey:  "media/key",
		},
	}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{deleteErr: errors.New("boom")}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := svc.DeleteMedia(context.Background(), DeleteMediaParams{
		StoreID: storeID,
		MediaID: mediaID,
	}); err == nil {
		t.Fatal("expected dependency error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeDependency {
		t.Fatalf("expected dependency code, got %v", err)
	}
}

func TestMediaServicePresignGcsErrorCleansUp(t *testing.T) {
	t.Parallel()

	repo := &stubMediaRepo{}
	members := stubMemberships{ok: true}
	gcs := &stubGCS{err: errTest}
	svc, err := NewService(repo, members, gcs, "bucket", time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	userID := uuid.New()
	storeID := uuid.New()
	_, err = svc.PresignUpload(context.Background(), userID, storeID, PresignInput{
		Kind:      enums.MediaKindProduct,
		MimeType:  "image/png",
		FileName:  "x.png",
		SizeBytes: 100,
	})
	if err == nil {
		t.Fatal("expected error from gcs")
	}
	if repo.deleteID != repo.created.ID {
		t.Fatalf("expected delete called for %s got %s", repo.created.ID, repo.deleteID)
	}
}

var errTest = fmt.Errorf("boom")

func contains(value, substring string) bool {
	return strings.Contains(value, substring)
}
