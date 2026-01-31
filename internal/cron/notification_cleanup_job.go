package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"gorm.io/gorm"
)

const notificationRetentionDays = 30

type NotificationCleanupJobParams struct {
	Logger     *logger.Logger
	DB         txRunner
	Repository notificationsCleanupRepo
	Retention  int
}

type notificationsCleanupRepo interface {
	DeleteOlderThan(ctx context.Context, tx *gorm.DB, cutoff time.Time) (int64, error)
}

func NewNotificationCleanupJob(params NotificationCleanupJobParams) (Job, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db runner required")
	}
	if params.Repository == nil {
		return nil, fmt.Errorf("notifications repository required")
	}
	retention := params.Retention
	if retention <= 0 {
		retention = notificationRetentionDays
	}
	return &notificationCleanupJob{
		logg:      params.Logger,
		db:        params.DB,
		repo:      params.Repository,
		retention: retention,
		now:       time.Now,
	}, nil
}

type notificationCleanupJob struct {
	logg      *logger.Logger
	db        txRunner
	repo      notificationsCleanupRepo
	retention int
	now       func() time.Time
}

func (j *notificationCleanupJob) Name() string { return "notification-cleanup" }

func (j *notificationCleanupJob) Run(ctx context.Context) error {
	cutoff := j.now().UTC().Add(-time.Duration(j.retention) * 24 * time.Hour)
	var deleted int64
	err := j.db.WithTx(ctx, func(tx *gorm.DB) error {
		rows, err := j.repo.DeleteOlderThan(ctx, tx, cutoff)
		if err != nil {
			return err
		}
		deleted = rows
		return nil
	})
	if err != nil {
		return fmt.Errorf("notification cleanup: %w", err)
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{
		"cutoff":         cutoff,
		"retention_days": j.retention,
		"rows_deleted":   deleted,
	})
	j.logg.Info(logCtx, "notification cleanup complete")
	return nil
}
