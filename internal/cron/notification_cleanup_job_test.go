package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"gorm.io/gorm"
)

func TestNotificationCleanupJobDeletesExpiredNotifications(t *testing.T) {
	now := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	repo := &fakeNotificationRepo{deletedRows: 42}
	job := newNotificationCleanupJob(t, repo)
	job.now = func() time.Time { return now }

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	expectedCutoff := now.UTC().Add(-notificationRetentionDays * 24 * time.Hour)
	if !repo.lastCutoff.Equal(expectedCutoff) {
		t.Fatalf("expected cutoff %s, got %s", expectedCutoff, repo.lastCutoff)
	}
	if repo.called != 1 {
		t.Fatalf("expected repo called once, got %d", repo.called)
	}
}

func TestNotificationCleanupJobPropagatesErrors(t *testing.T) {
	repo := &fakeNotificationRepo{err: errors.New("boom")}
	job := newNotificationCleanupJob(t, repo)

	if err := job.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func newNotificationCleanupJob(t *testing.T, repo *fakeNotificationRepo) *notificationCleanupJob {
	t.Helper()
	jobIface, err := NewNotificationCleanupJob(NotificationCleanupJobParams{
		Logger:     logger.New(logger.Options{ServiceName: "test"}),
		DB:         notificationFakeTxRunner{},
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("NewNotificationCleanupJob: %v", err)
	}
	job, ok := jobIface.(*notificationCleanupJob)
	if !ok {
		t.Fatalf("expected notificationCleanupJob, got %T", jobIface)
	}
	return job
}

type fakeNotificationRepo struct {
	lastCutoff  time.Time
	deletedRows int64
	err         error
	called      int
}

func (f *fakeNotificationRepo) DeleteOlderThan(ctx context.Context, tx *gorm.DB, cutoff time.Time) (int64, error) {
	f.called++
	f.lastCutoff = cutoff
	if f.err != nil {
		return 0, f.err
	}
	return f.deletedRows, nil
}

type notificationFakeTxRunner struct{}

func (notificationFakeTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}
