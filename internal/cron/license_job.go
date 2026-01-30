package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	"gorm.io/gorm"
)

const (
	expiryWarningDays    = 14
	expirationWindowDays = 30
	deletionAgeDays      = 30
)

// LicenseLifecycleJobParams configures the scheduled license work.
type LicenseLifecycleJobParams struct {
	Logger         *logger.Logger
	DB             txRunner
	LicenseRepo    licensesRepository
	StoreRepo      storeRepository
	MediaRepo      mediaRepository
	AttachmentRepo attachmentRepository
	Outbox         outboxEmitter
	OutboxRepo     outboxExistenceChecker
	GCS            gcsClient
	GCSBucket      string
}

// NewLicenseLifecycleJob constructs the license lifecycle cron job.
func NewLicenseLifecycleJob(params LicenseLifecycleJobParams) (Job, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db runner required")
	}
	if params.LicenseRepo == nil {
		return nil, fmt.Errorf("license repository required")
	}
	if params.StoreRepo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	if params.MediaRepo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if params.AttachmentRepo == nil {
		return nil, fmt.Errorf("attachment repository required")
	}
	if params.Outbox == nil {
		return nil, fmt.Errorf("outbox service required")
	}
	if params.OutboxRepo == nil {
		return nil, fmt.Errorf("outbox repository required")
	}
	return &licenseLifecycleJob{
		logg:           params.Logger,
		db:             params.DB,
		licenseRepo:    params.LicenseRepo,
		storeRepo:      params.StoreRepo,
		mediaRepo:      params.MediaRepo,
		attachmentRepo: params.AttachmentRepo,
		outbox:         params.Outbox,
		outboxRepo:     params.OutboxRepo,
		gcs:            params.GCS,
		bucket:         params.GCSBucket,
		now:            time.Now,
	}, nil
}

type licensesRepository interface {
	FindExpiringBetween(ctx context.Context, from, to time.Time) ([]models.License, error)
	FindExpiredInRange(ctx context.Context, from, to time.Time) ([]models.License, error)
	FindExpiredBefore(ctx context.Context, cutoff time.Time) ([]models.License, error)
	UpdateStatusWithTx(tx *gorm.DB, id uuid.UUID, status enums.LicenseStatus) error
	ListStatusesWithTx(tx *gorm.DB, storeID uuid.UUID) ([]enums.LicenseStatus, error)
	DeleteWithTx(tx *gorm.DB, id uuid.UUID) error
}

type storeRepository interface {
	UpdateStatusWithTx(tx *gorm.DB, storeID uuid.UUID, newStatus enums.KYCStatus) error
}

type mediaRepository interface {
	DeleteWithTx(tx *gorm.DB, id uuid.UUID) error
}

type attachmentRepository interface {
	ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error)
	Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error
}

type outboxEmitter interface {
	Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error
}

type outboxExistenceChecker interface {
	Exists(ctx context.Context, eventType enums.OutboxEventType, aggregateType enums.OutboxAggregateType, aggregateID uuid.UUID) (bool, error)
}

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type gcsClient interface {
	DeleteObject(ctx context.Context, bucket, object string) error
}

type licenseLifecycleJob struct {
	logg           *logger.Logger
	db             txRunner
	licenseRepo    licensesRepository
	storeRepo      storeRepository
	mediaRepo      mediaRepository
	attachmentRepo attachmentRepository
	outbox         outboxEmitter
	outboxRepo     outboxExistenceChecker
	gcs            gcsClient
	bucket         string
	now            func() time.Time
}

func (j *licenseLifecycleJob) Name() string { return "license-lifecycle" }

func (j *licenseLifecycleJob) Run(ctx context.Context) error {
	var errs []error
	if err := j.warnExpiring(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := j.expireLicenses(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := j.deleteExpired(ctx); err != nil {
		errs = append(errs, err)
	}
	return multierr.Combine(errs...)
}

func (j *licenseLifecycleJob) warnExpiring(ctx context.Context) error {
	now := j.now().UTC()
	target := now.Add(expiryWarningDays * 24 * time.Hour)
	start := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	licenses, err := j.licenseRepo.FindExpiringBetween(ctx, start, end)
	if err != nil {
		return fmt.Errorf("query expiring licenses: %w", err)
	}
	count := 0
	for _, lic := range licenses {
		if lic.Status != enums.LicenseStatusVerified || lic.ExpirationDate == nil {
			continue
		}
		exists, err := j.outboxRepo.Exists(ctx, enums.EventLicenseExpiringSoon, enums.AggregateLicense, lic.ID)
		if err != nil {
			return fmt.Errorf("check existing warning event: %w", err)
		}
		if exists {
			continue
		}
		if err := j.db.WithTx(ctx, func(tx *gorm.DB) error {
			event := outbox.DomainEvent{
				EventType:     enums.EventLicenseExpiringSoon,
				AggregateType: enums.AggregateLicense,
				AggregateID:   lic.ID,
				Data: LicenseExpiringSoonEvent{
					LicenseID:           lic.ID,
					StoreID:             lic.StoreID,
					ExpirationDate:      *lic.ExpirationDate,
					DaysUntilExpiration: expiryWarningDays,
				},
				Version:    1,
				OccurredAt: j.now().UTC(),
			}
			return j.outbox.Emit(ctx, tx, event)
		}); err != nil {
			return fmt.Errorf("queue warning event: %w", err)
		}
		count++
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{"count": count})
	j.logg.Info(logCtx, "license warn loop complete")
	return nil
}

func (j *licenseLifecycleJob) expireLicenses(ctx context.Context) error {
	now := j.now().UTC()
	from := now.Add(-expirationWindowDays * 24 * time.Hour)
	to := now
	licenses, err := j.licenseRepo.FindExpiredInRange(ctx, from, to)
	if err != nil {
		return fmt.Errorf("query licenses for expiry: %w", err)
	}
	count := 0
	for _, lic := range licenses {
		if lic.Status != enums.LicenseStatusVerified || lic.ExpirationDate == nil {
			continue
		}
		if err := j.expireLicense(ctx, lic); err != nil {
			return err
		}
		count++
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{"count": count})
	j.logg.Info(logCtx, "license expiry loop complete")
	return nil
}

func (j *licenseLifecycleJob) expireLicense(ctx context.Context, lic models.License) error {
	return j.db.WithTx(ctx, func(tx *gorm.DB) error {
		if err := j.licenseRepo.UpdateStatusWithTx(tx, lic.ID, enums.LicenseStatusExpired); err != nil {
			return err
		}
		statuses, err := j.licenseRepo.ListStatusesWithTx(tx, lic.StoreID)
		if err != nil {
			return err
		}
		newStatus := licenses.DetermineStoreKYCStatus(statuses)
		if err := j.storeRepo.UpdateStatusWithTx(tx, lic.StoreID, newStatus); err != nil {
			return err
		}
		event := outbox.DomainEvent{
			EventType:     enums.EventLicenseExpired,
			AggregateType: enums.AggregateLicense,
			AggregateID:   lic.ID,
			Data: LicenseExpiredEvent{
				LicenseID:      lic.ID,
				StoreID:        lic.StoreID,
				ExpirationDate: *lic.ExpirationDate,
				ExpiredAt:      j.now().UTC(),
			},
			Version:    1,
			OccurredAt: j.now().UTC(),
		}
		return j.outbox.Emit(ctx, tx, event)
	})
}

func (j *licenseLifecycleJob) deleteExpired(ctx context.Context) error {
	cutoff := j.now().UTC().Add(-deletionAgeDays * 24 * time.Hour)
	licenses, err := j.licenseRepo.FindExpiredBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("query stale licenses: %w", err)
	}
	count := 0
	for _, lic := range licenses {
		if err := j.deleteLicense(ctx, lic); err != nil {
			return err
		}
		count++
	}
	logCtx := j.logg.WithFields(ctx, map[string]any{"count": count})
	j.logg.Info(logCtx, "license hard-delete loop complete")
	return nil
}

func (j *licenseLifecycleJob) deleteLicense(ctx context.Context, lic models.License) error {
	return j.db.WithTx(ctx, func(tx *gorm.DB) error {
		attachments, err := j.attachmentRepo.ListByMediaID(ctx, lic.MediaID)
		if err != nil {
			return fmt.Errorf("load attachments: %w", err)
		}
		for _, attachment := range attachments {
			if err := j.attachmentRepo.Delete(ctx, tx, attachment.EntityType, attachment.EntityID, attachment.MediaID); err != nil {
				return fmt.Errorf("delete attachment: %w", err)
			}
		}
		if j.gcs != nil && lic.GCSKey != "" {
			if err := j.gcs.DeleteObject(ctx, j.bucket, lic.GCSKey); err != nil {
				return fmt.Errorf("delete gcs object: %w", err)
			}
		}
		if err := j.licenseRepo.DeleteWithTx(tx, lic.ID); err != nil {
			return err
		}
		if err := j.mediaRepo.DeleteWithTx(tx, lic.MediaID); err != nil {
			return err
		}
		statuses, err := j.licenseRepo.ListStatusesWithTx(tx, lic.StoreID)
		if err != nil {
			return err
		}
		newStatus := licenses.DetermineStoreKYCStatus(statuses)
		if err := j.storeRepo.UpdateStatusWithTx(tx, lic.StoreID, newStatus); err != nil {
			return err
		}
		return nil
	})
}

// LicenseExpiringSoonEvent describes the payload for the warning.
type LicenseExpiringSoonEvent struct {
	LicenseID           uuid.UUID `json:"licenseId"`
	StoreID             uuid.UUID `json:"storeId"`
	ExpirationDate      time.Time `json:"expirationDate"`
	DaysUntilExpiration int       `json:"daysUntilExpiration"`
}

// LicenseExpiredEvent describes the payload for expired licenses.
type LicenseExpiredEvent struct {
	LicenseID      uuid.UUID `json:"licenseId"`
	StoreID        uuid.UUID `json:"storeId"`
	ExpirationDate time.Time `json:"expirationDate"`
	ExpiredAt      time.Time `json:"expiredAt"`
}
