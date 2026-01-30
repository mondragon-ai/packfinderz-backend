package cron

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestLicenseLifecycleJob_warnExpiring(t *testing.T) {
	helper := createLicenseJobTest(t)
	now := time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC)
	helper.job.now = func() time.Time { return now }
	license := models.License{
		ID:             uuid.New(),
		StoreID:        uuid.New(),
		Status:         enums.LicenseStatusVerified,
		ExpirationDate: ptrTime(now.Add(expiryWarningDays * 24 * time.Hour)),
	}
	helper.licenseRepo.expiring = []models.License{license}
	helper.outboxRepo.exists = false
	if err := helper.job.warnExpiring(context.Background()); err != nil {
		t.Fatalf("warnExpiring: %v", err)
	}
	if len(helper.outboxSvc.events) != 1 {
		t.Fatalf("expected 1 warning event, got %d", len(helper.outboxSvc.events))
	}
	event := helper.outboxSvc.events[0]
	if event.EventType != enums.EventLicenseExpiringSoon {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestLicenseLifecycleJob_expireLicensesUpdatesStoreAndEmitsEvent(t *testing.T) {
	helper := createLicenseJobTest(t)
	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	helper.job.now = func() time.Time { return now }
	license := models.License{
		ID:             uuid.New(),
		StoreID:        uuid.New(),
		Status:         enums.LicenseStatusVerified,
		ExpirationDate: ptrTime(now.Add(-1 * 24 * time.Hour)),
	}
	helper.licenseRepo.expiredRange = []models.License{license}
	helper.licenseRepo.listStatuses = []enums.LicenseStatus{enums.LicenseStatusExpired}
	if err := helper.job.expireLicenses(context.Background()); err != nil {
		t.Fatalf("expireLicenses: %v", err)
	}
	if len(helper.outboxSvc.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(helper.outboxSvc.events))
	}
	if helper.storeRepo.updatedStatus != enums.KYCStatusExpired {
		t.Fatalf("expected store status expired, got %s", helper.storeRepo.updatedStatus)
	}
	if len(helper.licenseRepo.updateCalls) != 1 {
		t.Fatalf("expected 1 license update, got %d", len(helper.licenseRepo.updateCalls))
	}
}

func TestLicenseLifecycleJob_deleteExpiredRemovesAttachmentsAndMedia(t *testing.T) {
	helper := createLicenseJobTest(t)
	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	helper.job.now = func() time.Time { return now }
	license := models.License{
		ID:             uuid.New(),
		StoreID:        uuid.New(),
		Status:         enums.LicenseStatusExpired,
		MediaID:        uuid.New(),
		ExpirationDate: ptrTime(now.Add(-40 * 24 * time.Hour)),
		GCSKey:         "license/key",
	}
	helper.licenseRepo.expiredBefore = []models.License{license}
	helper.attachmentRepo.attachments = []models.MediaAttachment{{
		MediaID:    license.MediaID,
		EntityType: "license",
		EntityID:   license.ID,
	}}
	helper.job.bucket = "bucket"
	if err := helper.job.deleteExpired(context.Background()); err != nil {
		t.Fatalf("deleteExpired: %v", err)
	}
	if helper.gcs.deletedKey != license.GCSKey {
		t.Fatalf("expected gcs key deleted, got %s", helper.gcs.deletedKey)
	}
	if len(helper.licenseRepo.deleteCalls) != 1 {
		t.Fatalf("expected license deleted, got %d", len(helper.licenseRepo.deleteCalls))
	}
	if len(helper.mediaRepo.deleteCalls) != 1 {
		t.Fatalf("expected media deleted, got %d", len(helper.mediaRepo.deleteCalls))
	}
}

type licenseJobTestHelper struct {
	job            *licenseLifecycleJob
	licenseRepo    *fakeLicenseRepo
	storeRepo      *fakeStoreRepo
	mediaRepo      *fakeMediaRepo
	attachmentRepo *fakeAttachmentRepo
	outboxRepo     *fakeOutboxRepo
	outboxSvc      *fakeOutboxService
	gcs            *fakeGCS
}

func createLicenseJobTest(t *testing.T) *licenseJobTestHelper {
	t.Helper()
	licenseRepo := &fakeLicenseRepo{}
	storeRepo := &fakeStoreRepo{}
	mediaRepo := &fakeMediaRepo{}
	attachmentRepo := &fakeAttachmentRepo{}
	outboxRepo := &fakeOutboxRepo{}
	outboxSvc := &fakeOutboxService{}
	gcsClient := &fakeGCS{}
	jobIface, err := NewLicenseLifecycleJob(LicenseLifecycleJobParams{
		Logger:         logger.New(logger.Options{ServiceName: "test"}),
		DB:             fakeTxRunner{},
		LicenseRepo:    licenseRepo,
		StoreRepo:      storeRepo,
		MediaRepo:      mediaRepo,
		AttachmentRepo: attachmentRepo,
		Outbox:         outboxSvc,
		OutboxRepo:     outboxRepo,
		GCS:            gcsClient,
		GCSBucket:      "bucket",
	})
	if err != nil {
		t.Fatalf("NewLicenseLifecycleJob: %v", err)
	}
	job, ok := jobIface.(*licenseLifecycleJob)
	if !ok {
		t.Fatalf("expected licenseLifecycleJob, got %T", jobIface)
	}
	return &licenseJobTestHelper{
		job:            job,
		licenseRepo:    licenseRepo,
		storeRepo:      storeRepo,
		mediaRepo:      mediaRepo,
		attachmentRepo: attachmentRepo,
		outboxRepo:     outboxRepo,
		outboxSvc:      outboxSvc,
		gcs:            gcsClient,
	}
}

func ptrTime(v time.Time) *time.Time { return &v }

type fakeLicenseRepo struct {
	expiring      []models.License
	expiredRange  []models.License
	expiredBefore []models.License
	listStatuses  []enums.LicenseStatus
	updateCalls   []licenseUpdateCall
	deleteCalls   []uuid.UUID
}

type licenseUpdateCall struct {
	id     uuid.UUID
	status enums.LicenseStatus
}

func (f *fakeLicenseRepo) FindExpiringBetween(ctx context.Context, from, to time.Time) ([]models.License, error) {
	return f.expiring, nil
}

func (f *fakeLicenseRepo) FindExpiredInRange(ctx context.Context, from, to time.Time) ([]models.License, error) {
	return f.expiredRange, nil
}

func (f *fakeLicenseRepo) FindExpiredBefore(ctx context.Context, cutoff time.Time) ([]models.License, error) {
	return f.expiredBefore, nil
}

func (f *fakeLicenseRepo) UpdateStatusWithTx(tx *gorm.DB, id uuid.UUID, status enums.LicenseStatus) error {
	f.updateCalls = append(f.updateCalls, licenseUpdateCall{id: id, status: status})
	return nil
}

func (f *fakeLicenseRepo) ListStatusesWithTx(tx *gorm.DB, storeID uuid.UUID) ([]enums.LicenseStatus, error) {
	return f.listStatuses, nil
}

func (f *fakeLicenseRepo) DeleteWithTx(tx *gorm.DB, id uuid.UUID) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return nil
}

type fakeStoreRepo struct {
	updatedStatus enums.KYCStatus
}

func (f *fakeStoreRepo) UpdateStatusWithTx(tx *gorm.DB, storeID uuid.UUID, newStatus enums.KYCStatus) error {
	f.updatedStatus = newStatus
	return nil
}

type fakeMediaRepo struct {
	deleteCalls []uuid.UUID
}

func (f *fakeMediaRepo) DeleteWithTx(tx *gorm.DB, id uuid.UUID) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return nil
}

type fakeAttachmentRepo struct {
	attachments []models.MediaAttachment
	deleted     []models.MediaAttachment
}

func (f *fakeAttachmentRepo) ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error) {
	return f.attachments, nil
}

func (f *fakeAttachmentRepo) Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error {
	f.deleted = append(f.deleted, models.MediaAttachment{EntityType: entityType, EntityID: entityID, MediaID: mediaID})
	return nil
}

type fakeOutboxRepo struct {
	exists bool
}

func (f *fakeOutboxRepo) Exists(ctx context.Context, eventType enums.OutboxEventType, aggregateType enums.OutboxAggregateType, aggregateID uuid.UUID) (bool, error) {
	return f.exists, nil
}

type fakeOutboxService struct {
	events []outbox.DomainEvent
}

func (f *fakeOutboxService) Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error {
	f.events = append(f.events, event)
	return nil
}

type fakeGCS struct {
	deletedKey string
}

func (f *fakeGCS) DeleteObject(ctx context.Context, bucket, object string) error {
	f.deletedKey = object
	return nil
}

type fakeTxRunner struct{}

func (fakeTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}
