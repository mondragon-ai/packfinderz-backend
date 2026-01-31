package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"gorm.io/gorm"
)

func TestOutboxRetentionJobDeletesPublishedRows(t *testing.T) {
	now := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeOutboxRetentionRepo{}
	job := newOutboxRetentionJob(t, repo)
	job.now = func() time.Time { return now }

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	expectedCutoff := now.UTC().Add(-outboxRetentionDays * 24 * time.Hour)
	if !repo.lastCutoff.Equal(expectedCutoff) {
		t.Fatalf("expected cutoff %s, got %s", expectedCutoff, repo.lastCutoff)
	}
	if repo.minAttempts != outboxMinAttempts {
		t.Fatalf("expected min attempts %d, got %d", outboxMinAttempts, repo.minAttempts)
	}
	if repo.called != 1 {
		t.Fatalf("expected repo called once, got %d", repo.called)
	}
}

func TestOutboxRetentionJobPropagatesError(t *testing.T) {
	repo := &fakeOutboxRetentionRepo{err: errors.New("boom")}
	job := newOutboxRetentionJob(t, repo)

	if err := job.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func newOutboxRetentionJob(t *testing.T, repo *fakeOutboxRetentionRepo) *outboxRetentionJob {
	t.Helper()
	jobIface, err := NewOutboxRetentionJob(OutboxRetentionJobParams{
		Logger:     logger.New(logger.Options{ServiceName: "test"}),
		DB:         outboxRetentionTxRunner{},
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("NewOutboxRetentionJob: %v", err)
	}
	job, ok := jobIface.(*outboxRetentionJob)
	if !ok {
		t.Fatalf("expected outboxRetentionJob, got %T", jobIface)
	}
	return job
}

type fakeOutboxRetentionRepo struct {
	lastCutoff  time.Time
	minAttempts int
	called      int
	err         error
}

func (f *fakeOutboxRetentionRepo) DeletePublishedBefore(ctx context.Context, tx *gorm.DB, cutoff time.Time, minAttemptCount int) (int64, error) {
	f.called++
	f.lastCutoff = cutoff
	f.minAttempts = minAttemptCount
	if f.err != nil {
		return 0, f.err
	}
	return 7, nil
}

type outboxRetentionTxRunner struct{}

func (outboxRetentionTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}
