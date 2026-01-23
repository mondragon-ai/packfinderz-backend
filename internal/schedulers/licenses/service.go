package licenses

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	licdomain "github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
)

const (
	expiryWarningDays = 14
	schedulerInterval = 24 * time.Hour
	expiryWarningType = "expiry_warning"
)

type schedulerRepo interface {
	FindExpiringBetween(ctx context.Context, from, to time.Time) ([]models.License, error)
	FindExpiredByDate(ctx context.Context, day time.Time) ([]models.License, error)
	FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.License, error)
	UpdateStatusWithTx(tx *gorm.DB, id uuid.UUID, status enums.LicenseStatus) error
	ListStatusesWithTx(tx *gorm.DB, storeID uuid.UUID) ([]enums.LicenseStatus, error)
}

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

// Service emits expiry warnings and expires licenses each day.
type Service struct {
	logg      *logger.Logger
	db        txRunner
	repo      schedulerRepo
	storeRepo *stores.Repository
	outbox    *outbox.Service
	interval  time.Duration
}

type ServiceParams struct {
	Logger    *logger.Logger
	DB        txRunner
	Repo      *licdomain.Repository
	StoreRepo *stores.Repository
	Outbox    *outbox.Service
}

// NewService builds the licence expiry scheduler.
func NewService(params ServiceParams) (*Service, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if params.Repo == nil {
		return nil, fmt.Errorf("license repository required")
	}
	if params.StoreRepo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	if params.Outbox == nil {
		return nil, fmt.Errorf("outbox service required")
	}
	return &Service{
		logg:      params.Logger,
		db:        params.DB,
		repo:      params.Repo,
		storeRepo: params.StoreRepo,
		outbox:    params.Outbox,
		interval:  schedulerInterval,
	}, nil
}

// Run executes the scheduler loop until the context is canceled.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.process(ctx); err != nil {
		s.logg.Error(ctx, "license scheduler run failed", err)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logg.Info(ctx, "license scheduler context canceled")
			return ctx.Err()
		case <-ticker.C:
			if err := s.process(ctx); err != nil {
				s.logg.Error(ctx, "license scheduler run failed", err)
			}
		}
	}
}

func (s *Service) process(ctx context.Context) error {
	var errs []error
	if err := s.warnExpiring(ctx); err != nil {
		errs = append(errs, fmt.Errorf("warn expiring: %w", err))
	}
	if err := s.expireLicenses(ctx); err != nil {
		errs = append(errs, fmt.Errorf("expire licenses: %w", err))
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("license scheduler errors: %v", errs)
}

func (s *Service) warnExpiring(ctx context.Context) error {
	target := time.Now().UTC().AddDate(0, 0, expiryWarningDays)
	start := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	licenses, err := s.repo.FindExpiringBetween(ctx, start, end)
	if err != nil {
		return err
	}
	for _, license := range licenses {
		if err := s.emitWarning(ctx, license); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) emitWarning(ctx context.Context, license models.License) error {
	return s.db.WithTx(ctx, func(tx *gorm.DB) error {
		event := outbox.DomainEvent{
			EventType:     enums.EventLicenseStatusChanged,
			AggregateType: enums.AggregateLicense,
			AggregateID:   license.ID,
			Data: licdomain.LicenseStatusChangedEvent{
				LicenseID:   license.ID,
				StoreID:     license.StoreID,
				Status:      license.Status,
				Reason:      fmt.Sprintf("expires on %s", license.ExpirationDate.Format("2006-01-02")),
				WarningType: expiryWarningType,
			},
			Version:    1,
			OccurredAt: time.Now().UTC(),
		}
		return s.outbox.Emit(ctx, tx, event)
	})
}

func (s *Service) expireLicenses(ctx context.Context) error {
	today := time.Now().UTC()
	licenses, err := s.repo.FindExpiredByDate(ctx, today)
	if err != nil {
		return err
	}
	for _, license := range licenses {
		if err := s.expireLicense(ctx, license); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) expireLicense(ctx context.Context, license models.License) error {
	return s.db.WithTx(ctx, func(tx *gorm.DB) error {
		current, err := s.repo.FindByIDWithTx(tx, license.ID)
		if err != nil {
			return err
		}
		if current.Status == enums.LicenseStatusExpired {
			return nil
		}
		if err := s.repo.UpdateStatusWithTx(tx, license.ID, enums.LicenseStatusExpired); err != nil {
			return err
		}
		if err := s.reconcileKYC(ctx, tx, current.StoreID); err != nil {
			return err
		}
		event := outbox.DomainEvent{
			EventType:     enums.EventLicenseStatusChanged,
			AggregateType: enums.AggregateLicense,
			AggregateID:   license.ID,
			Data: licdomain.LicenseStatusChangedEvent{
				LicenseID: license.ID,
				StoreID:   license.StoreID,
				Status:    enums.LicenseStatusExpired,
				Reason:    "expired by scheduler",
			},
			Version:    1,
			OccurredAt: time.Now().UTC(),
		}
		return s.outbox.Emit(ctx, tx, event)
	})
}

func (s *Service) reconcileKYC(ctx context.Context, tx *gorm.DB, storeID uuid.UUID) error {
	statuses, err := s.repo.ListStatusesWithTx(tx, storeID)
	if err != nil {
		return err
	}
	newStatus := licdomain.DetermineStoreKYCStatus(statuses)
	if newStatus == enums.KYCStatusPendingVerification {
		return nil
	}
	store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
	if err != nil {
		return err
	}
	if store.KYCStatus == newStatus {
		return nil
	}
	store.KYCStatus = newStatus
	return s.storeRepo.UpdateWithTx(tx, store)
}
