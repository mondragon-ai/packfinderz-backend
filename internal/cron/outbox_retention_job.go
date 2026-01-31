package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"gorm.io/gorm"
)

const (
	outboxRetentionDays = 30
	outboxMinAttempts   = 5
)

type OutboxRetentionJobParams struct {
	Logger      *logger.Logger
	DB          txRunner
	Repository  outboxRetentionRepo
	Retention   int
	MinAttempts int
}

type outboxRetentionRepo interface {
	DeletePublishedBefore(ctx context.Context, tx *gorm.DB, cutoff time.Time, minAttemptCount int) (int64, error)
}

func NewOutboxRetentionJob(params OutboxRetentionJobParams) (Job, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db runner required")
	}
	if params.Repository == nil {
		return nil, fmt.Errorf("outbox repository required")
	}
	retention := params.Retention
	if retention <= 0 {
		retention = outboxRetentionDays
	}
	minAttempts := params.MinAttempts
	if minAttempts <= 0 {
		minAttempts = outboxMinAttempts
	}
	return &outboxRetentionJob{
		logg:        params.Logger,
		db:          params.DB,
		repo:        params.Repository,
		retention:   retention,
		minAttempts: minAttempts,
		now:         time.Now,
	}, nil
}

type outboxRetentionJob struct {
	logg        *logger.Logger
	db          txRunner
	repo        outboxRetentionRepo
	retention   int
	minAttempts int
	now         func() time.Time
}

func (j *outboxRetentionJob) Name() string { return "outbox-retention" }

func (j *outboxRetentionJob) Run(ctx context.Context) error {
	cutoff := j.now().UTC().Add(-time.Duration(j.retention) * 24 * time.Hour)
	var deleted int64
	err := j.db.WithTx(ctx, func(tx *gorm.DB) error {
		rows, err := j.repo.DeletePublishedBefore(ctx, tx, cutoff, j.minAttempts)
		if err != nil {
			return err
		}
		deleted = rows
		return nil
	})
	if err != nil {
		return fmt.Errorf("outbox retention: %w", err)
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{
		"cutoff":         cutoff,
		"retention_days": j.retention,
		"min_attempts":   j.minAttempts,
		"rows_deleted":   deleted,
	})
	j.logg.Info(logCtx, "outbox retention cleanup complete")
	return nil
}
