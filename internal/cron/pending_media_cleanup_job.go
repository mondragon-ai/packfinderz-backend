package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const pendingMediaRetentionDays = 7

type PendingMediaCleanupJobParams struct {
	Logger         *logger.Logger
	DB             txRunner
	MediaRepo      pendingMediaCleanupRepo
	AttachmentRepo pendingAttachmentRepo
	RetentionDays  int
}

type pendingMediaCleanupRepo interface {
	ListPendingBefore(ctx context.Context, cutoff time.Time) ([]models.Media, error)
	DeleteWithTx(tx *gorm.DB, id uuid.UUID) error
}

type pendingAttachmentRepo interface {
	DeleteByMediaID(ctx context.Context, tx *gorm.DB, mediaID uuid.UUID) (int64, error)
}

func NewPendingMediaCleanupJob(params PendingMediaCleanupJobParams) (Job, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db runner required")
	}
	if params.MediaRepo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if params.AttachmentRepo == nil {
		return nil, fmt.Errorf("attachment repository required")
	}
	retention := params.RetentionDays
	if retention <= 0 {
		retention = pendingMediaRetentionDays
	}
	return &pendingMediaCleanupJob{
		logg:          params.Logger,
		db:            params.DB,
		repo:          params.MediaRepo,
		attachments:   params.AttachmentRepo,
		retentionDays: retention,
		now:           time.Now,
	}, nil
}

type pendingMediaCleanupJob struct {
	logg          *logger.Logger
	db            txRunner
	repo          pendingMediaCleanupRepo
	attachments   pendingAttachmentRepo
	retentionDays int
	now           func() time.Time
}

func (j *pendingMediaCleanupJob) Name() string { return "pending-media-cleanup" }

func (j *pendingMediaCleanupJob) Run(ctx context.Context) error {
	cutoff := j.now().UTC().Add(-time.Duration(j.retentionDays) * 24 * time.Hour)
	var (
		deletedMedia       int64
		deletedAttachments int64
		mediaCandidates    int
	)
	err := j.db.WithTx(ctx, func(tx *gorm.DB) error {
		rows, err := j.repo.ListPendingBefore(ctx, cutoff)
		if err != nil {
			return fmt.Errorf("query pending media: %w", err)
		}
		mediaCandidates = len(rows)
		for _, mediaRow := range rows {
			attachmentsDeleted, err := j.attachments.DeleteByMediaID(ctx, tx, mediaRow.ID)
			if err != nil {
				return fmt.Errorf("delete media attachments: %w", err)
			}
			deletedAttachments += attachmentsDeleted

			if err := j.repo.DeleteWithTx(tx, mediaRow.ID); err != nil {
				return fmt.Errorf("delete media row: %w", err)
			}
			deletedMedia++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pending media cleanup: %w", err)
	}

	logCtx := j.logg.WithFields(ctx, map[string]any{
		"cutoff":              cutoff,
		"retention_days":      j.retentionDays,
		"media_candidates":    mediaCandidates,
		"media_deleted":       deletedMedia,
		"attachments_deleted": deletedAttachments,
	})
	j.logg.Info(logCtx, "pending media cleanup complete")
	return nil
}
