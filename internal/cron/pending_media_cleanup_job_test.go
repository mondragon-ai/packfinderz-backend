package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestPendingMediaCleanupDeletesStaleRows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	rows := []models.Media{
		{ID: uuid.New()},
		{ID: uuid.New()},
	}
	repo := &fakePendingMediaRepo{rows: rows}
	attachmentRepo := &fakePendingAttachmentRepo{}
	job := newPendingMediaCleanupJob(t, repo, attachmentRepo)
	job.now = func() time.Time { return now }

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	expectedCutoff := now.UTC().Add(-pendingMediaRetentionDays * 24 * time.Hour)
	if !repo.lastCutoff.Equal(expectedCutoff) {
		t.Fatalf("expected cutoff %s got %s", expectedCutoff, repo.lastCutoff)
	}
	if len(repo.deletedIDs) != len(rows) {
		t.Fatalf("expected deleted media %d got %d", len(rows), len(repo.deletedIDs))
	}
	if len(attachmentRepo.deletedMediaIDs) != len(rows) {
		t.Fatalf("expected attachments cleaned for each media, got %d", len(attachmentRepo.deletedMediaIDs))
	}
}

func TestPendingMediaCleanupPropagatesErrors(t *testing.T) {
	t.Parallel()

	repo := &fakePendingMediaRepo{listErr: errors.New("list failure")}
	job := newPendingMediaCleanupJob(t, repo, &fakePendingAttachmentRepo{})

	if err := job.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func newPendingMediaCleanupJob(t *testing.T, repo *fakePendingMediaRepo, attachments *fakePendingAttachmentRepo) *pendingMediaCleanupJob {
	t.Helper()
	jobIface, err := NewPendingMediaCleanupJob(PendingMediaCleanupJobParams{
		Logger:         logger.New(logger.Options{ServiceName: "test"}),
		DB:             pendingMediaFakeTxRunner{},
		MediaRepo:      repo,
		AttachmentRepo: attachments,
	})
	if err != nil {
		t.Fatalf("NewPendingMediaCleanupJob: %v", err)
	}
	job, ok := jobIface.(*pendingMediaCleanupJob)
	if !ok {
		t.Fatalf("expected pendingMediaCleanupJob, got %T", jobIface)
	}
	return job
}

type fakePendingMediaRepo struct {
	rows       []models.Media
	listErr    error
	deleteErr  error
	lastCutoff time.Time
	deletedIDs []uuid.UUID
}

func (f *fakePendingMediaRepo) ListPendingBefore(ctx context.Context, cutoff time.Time) ([]models.Media, error) {
	f.lastCutoff = cutoff
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rows, nil
}

func (f *fakePendingMediaRepo) DeleteWithTx(tx *gorm.DB, id uuid.UUID) error {
	f.deletedIDs = append(f.deletedIDs, id)
	return f.deleteErr
}

type fakePendingAttachmentRepo struct {
	deletedMediaIDs []uuid.UUID
	err             error
}

func (f *fakePendingAttachmentRepo) DeleteByMediaID(ctx context.Context, tx *gorm.DB, mediaID uuid.UUID) (int64, error) {
	f.deletedMediaIDs = append(f.deletedMediaIDs, mediaID)
	if f.err != nil {
		return 0, f.err
	}
	return 1, nil
}

type pendingMediaFakeTxRunner struct{}

func (pendingMediaFakeTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}
