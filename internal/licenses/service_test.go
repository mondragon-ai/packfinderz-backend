package licenses

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubLicenseRepo struct {
	created      *models.License
	listRows     []models.License
	listErr      error
	lastQuery    listQuery
	err          error
	findResult   *models.License
	findErr      error
	deleteErr    error
	validCount   int64
	countErr     error
	updateStatus enums.LicenseStatus
	updateErr    error
	statusRows   []enums.LicenseStatus
	statusErr    error
}

func (s *stubLicenseRepo) Create(ctx context.Context, license *models.License) (*models.License, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.created = license
	return license, nil
}

func (s *stubLicenseRepo) List(ctx context.Context, opts listQuery) ([]models.License, error) {
	s.lastQuery = opts
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listRows == nil {
		return nil, nil
	}
	return s.listRows, nil
}

func (s *stubLicenseRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.License, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.findResult == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.findResult, nil
}

func (s *stubLicenseRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteErr
}

func (s *stubLicenseRepo) CountValidLicenses(ctx context.Context, storeID uuid.UUID) (int64, error) {
	if s.countErr != nil {
		return 0, s.countErr
	}
	return s.validCount, nil
}

func (s *stubLicenseRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status enums.LicenseStatus) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updateStatus = status
	return nil
}

func (s *stubLicenseRepo) ListStatusesWithTx(_ *gorm.DB, storeID uuid.UUID) ([]enums.LicenseStatus, error) {
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	return s.statusRows, nil
}

func (s *stubLicenseRepo) CreateWithTx(_ *gorm.DB, license *models.License) (*models.License, error) {
	return s.Create(context.Background(), license)
}

func (s *stubLicenseRepo) FindByIDWithTx(_ *gorm.DB, id uuid.UUID) (*models.License, error) {
	return s.FindByID(context.Background(), id)
}

func (s *stubLicenseRepo) UpdateStatusWithTx(_ *gorm.DB, id uuid.UUID, status enums.LicenseStatus) error {
	return s.UpdateStatus(context.Background(), id, status)
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubOutboxPublisher struct {
	events []outbox.DomainEvent
	err    error
}

func (s *stubOutboxPublisher) Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, event)
	return nil
}

type stubMediaRepo struct {
	media *models.Media
	err   error
}

func (s *stubMediaRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.media == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.media, nil
}

type stubMemberships struct {
	ok    bool
	err   error
	roles []enums.MemberRole
}

func (s *stubMemberships) UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	s.roles = roles
	return s.ok, nil
}

type stubStoreRepo struct {
	store     *models.Store
	findErr   error
	updateErr error
	updated   *models.Store
}

// // UpdateStatusWithTx implements [storesRepository].
// func (s *stubStoreRepo) UpdateStatusWithTx(tx *gorm.DB, storeID uuid.UUID, newStatus enums.KYCStatus) error {
// 	panic("unimplemented")
// }

func (s *stubStoreRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.store == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.store, nil
}

func (s *stubStoreRepo) Update(ctx context.Context, store *models.Store) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = store
	return nil
}

func (s *stubStoreRepo) FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error) {
	return s.FindByID(context.Background(), id)
}

func (s *stubStoreRepo) UpdateWithTx(tx *gorm.DB, store *models.Store) error {
	return s.Update(context.Background(), store)
}

type stubGCS struct {
	url        string
	err        error
	lastBucket string
	lastObject string
	calls      int
}

func (s *stubGCS) SignedReadURL(bucket, object string, expires time.Duration) (string, error) {
	s.lastBucket = bucket
	s.lastObject = object
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	if s.url != "" {
		return s.url, nil
	}
	return "https://download.example", nil
}

func newServiceForTests(media *models.Media, members *stubMemberships, repo *stubLicenseRepo, storeRepo *stubStoreRepo) (Service, *stubLicenseRepo, *stubGCS, *stubStoreRepo, *stubOutboxPublisher) {
	if repo == nil {
		repo = &stubLicenseRepo{}
	}
	if media != nil && media.ID == uuid.Nil {
		media.ID = uuid.New()
	}
	if members == nil {
		members = &stubMemberships{ok: true}
	}
	mediaRepo := &stubMediaRepo{media: media}
	gcsStub := &stubGCS{}
	if storeRepo == nil {
		storeRepo = &stubStoreRepo{
			store: &models.Store{
				ID:        uuid.New(),
				KYCStatus: enums.KYCStatusVerified,
			},
		}
	}
	pub := &stubOutboxPublisher{}
	svc, err := NewService(repo, mediaRepo, members, gcsStub, "bucket", time.Minute, storeRepo, &stubTxRunner{}, pub)
	if err != nil {
		panic(err)
	}
	return svc, repo, gcsStub, storeRepo, pub
}

func TestCreateLicenseSuccess(t *testing.T) {
	now := time.Now()
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:        uuid.New(),
		StoreID:   storeID,
		Status:    enums.MediaStatusUploaded,
		GCSKey:    "media/license",
		MimeType:  "application/pdf",
		Kind:      enums.MediaKindLicenseDoc,
		CreatedAt: now,
	}

	members := &stubMemberships{ok: true}
	svc, repo, _, _, pub := newServiceForTests(mediaRow, members, nil, nil)

	input := CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "OK",
		IssueDate:    &now,
		Type:         enums.LicenseTypeProducer,
		Number:       " 123 ",
	}
	license, err := svc.CreateLicense(context.Background(), userID, storeID, input)
	if err != nil {
		t.Fatalf("CreateLicense returned error: %v", err)
	}
	if license.Number != "123" {
		t.Fatalf("expected trimmed number, got %q", license.Number)
	}
	if license.Status != enums.LicenseStatusPending {
		t.Fatalf("expected status pending, got %s", license.Status)
	}
	if repo.created == nil {
		t.Fatalf("expected license created")
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(pub.events))
	}
	event := pub.events[0]
	if event.EventType != enums.EventLicenseStatusChanged {
		t.Fatalf("unexpected event type %s", event.EventType)
	}
	payload, ok := event.Data.(payloads.LicenseStatusChangedEvent)
	if !ok {
		t.Fatalf("expected license status payload, got %T", event.Data)
	}
	if payload.LicenseID != license.ID {
		t.Fatalf("expected payload license id %s, got %s", license.ID, payload.LicenseID)
	}
	if payload.Status != enums.LicenseStatusPending {
		t.Fatalf("expected payload status pending, got %s", payload.Status)
	}
}

func TestCreateLicenseRequiresMembership(t *testing.T) {
	storeID := uuid.New()
	members := &stubMemberships{ok: false}
	svc, _, _, _, _ := newServiceForTests(&models.Media{StoreID: storeID, Status: enums.MediaStatusUploaded, Kind: enums.MediaKindLicenseDoc, MimeType: "application/pdf"}, members, nil, nil)

	if _, err := svc.CreateLicense(context.Background(), uuid.New(), storeID, CreateLicenseInput{
		MediaID:      uuid.New(),
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected forbidden error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestCreateLicenseMediaNotFound(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRepo := &stubMediaRepo{}
	repo := &stubLicenseRepo{}
	gcs := &stubGCS{}
	storeRepo := &stubStoreRepo{store: &models.Store{ID: storeID, KYCStatus: enums.KYCStatusVerified}}
	pub := &stubOutboxPublisher{}
	svc, err := NewService(repo, mediaRepo, &stubMemberships{ok: true}, gcs, "bucket", time.Minute, storeRepo, &stubTxRunner{}, pub)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      uuid.New(),
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected not found error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("expected not found code, got %v", err)
	}
}

func TestCreateLicenseMediaStoreMismatch(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  uuid.New(),
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/pdf",
	}
	svc, _, _, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil, nil)

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected forbidden error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestCreateLicenseInvalidStatus(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusPending,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/pdf",
	}
	svc, _, _, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil, nil)

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected conflict error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestCreateLicenseInvalidMime(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/octet-stream",
	}
	svc, _, _, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil, nil)

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected validation error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation code, got %v", err)
	}
}

func TestCreateLicenseInvalidKind(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindProduct,
		MimeType: "image/png",
	}
	svc, _, _, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil, nil)

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected validation error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation code, got %v", err)
	}
}

func TestCreateLicenseChecksAllMemberRoles(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/pdf",
	}
	members := &stubMemberships{ok: false}
	svc, _, _, _, _ := newServiceForTests(mediaRow, members, nil, nil)

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected forbidden error")
	}

	expectedRoles := []enums.MemberRole{
		enums.MemberRoleOwner,
		enums.MemberRoleAdmin,
		enums.MemberRoleManager,
		enums.MemberRoleStaff,
		enums.MemberRoleOps,
	}
	if len(members.roles) != len(expectedRoles) {
		t.Fatalf("expected %d roles, got %d", len(expectedRoles), len(members.roles))
	}
	for _, expected := range expectedRoles {
		if !containsRole(members.roles, expected) {
			t.Fatalf("expected role %s in membership check", expected)
		}
	}
}

func TestDeleteLicenseStatusRestriction(t *testing.T) {
	storeID := uuid.New()
	license := &models.License{
		ID:      uuid.New(),
		StoreID: storeID,
		Status:  enums.LicenseStatusPending,
	}
	repo := &stubLicenseRepo{findResult: license}
	storeRepo := &stubStoreRepo{store: &models.Store{ID: storeID, KYCStatus: enums.KYCStatusVerified}}
	svc, _, _, _, _ := newServiceForTests(nil, &stubMemberships{ok: true}, repo, storeRepo)

	if err := svc.DeleteLicense(context.Background(), uuid.New(), storeID, license.ID); err == nil {
		t.Fatal("expected conflict error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestDeleteLicenseDowngradesStoreWhenNoValidLicenses(t *testing.T) {
	storeID := uuid.New()
	license := &models.License{
		ID:      uuid.New(),
		StoreID: storeID,
		Status:  enums.LicenseStatusExpired,
	}
	repo := &stubLicenseRepo{
		findResult: license,
		validCount: 0,
	}
	storeRepo := &stubStoreRepo{store: &models.Store{ID: storeID, KYCStatus: enums.KYCStatusVerified}}
	svc, _, _, _, _ := newServiceForTests(nil, &stubMemberships{ok: true}, repo, storeRepo)

	if err := svc.DeleteLicense(context.Background(), uuid.New(), storeID, license.ID); err != nil {
		t.Fatalf("DeleteLicense returned error: %v", err)
	}
	if storeRepo.updated == nil {
		t.Fatal("expected store update")
	}
	if storeRepo.updated.KYCStatus != enums.KYCStatusPendingVerification {
		t.Fatalf("expected pending verification, got %s", storeRepo.updated.KYCStatus)
	}
}

func TestDeleteLicenseKeepsStoreWhenValidLicenseExists(t *testing.T) {
	storeID := uuid.New()
	license := &models.License{
		ID:      uuid.New(),
		StoreID: storeID,
		Status:  enums.LicenseStatusExpired,
	}
	repo := &stubLicenseRepo{
		findResult: license,
		validCount: 2,
	}
	storeRepo := &stubStoreRepo{store: &models.Store{ID: storeID, KYCStatus: enums.KYCStatusVerified}}
	svc, _, _, _, _ := newServiceForTests(nil, &stubMemberships{ok: true}, repo, storeRepo)

	if err := svc.DeleteLicense(context.Background(), uuid.New(), storeID, license.ID); err != nil {
		t.Fatalf("DeleteLicense returned error: %v", err)
	}
	if storeRepo.updated != nil {
		t.Fatalf("expected no store update, got %v", storeRepo.updated)
	}
}

// UpdateStatusWithTx implements [storesRepository].
func (s *stubStoreRepo) UpdateStatusWithTx(tx *gorm.DB, storeID uuid.UUID, newStatus enums.KYCStatus) error {
	if s.updateErr != nil {
		return s.updateErr
	}

	// mimic persistence + allow assertions
	if s.store == nil {
		s.store = &models.Store{ID: storeID}
	}
	s.store.ID = storeID
	s.store.KYCStatus = newStatus

	// keep existing test expectations that check storeRepo.updated
	s.updated = s.store
	return nil
}

func TestVerifyLicenseSuccess(t *testing.T) {
	license := &models.License{
		ID:      uuid.New(),
		Status:  enums.LicenseStatusPending,
		StoreID: uuid.New(),
	}
	repo := &stubLicenseRepo{
		findResult: license,
	}
	storeRepo := &stubStoreRepo{store: &models.Store{ID: license.StoreID, KYCStatus: enums.KYCStatusPendingVerification}}
	repo.statusRows = []enums.LicenseStatus{enums.LicenseStatusVerified}
	svc, _, _, storeRepo, pub := newServiceForTests(nil, nil, repo, storeRepo)

	updated, err := svc.VerifyLicense(context.Background(), license.ID, enums.LicenseStatusVerified, "approved")
	if err != nil {
		t.Fatalf("VerifyLicense returned error: %v", err)
	}
	if updated.Status != enums.LicenseStatusVerified {
		t.Fatalf("expected verified status, got %s", updated.Status)
	}
	if repo.updateStatus != enums.LicenseStatusVerified {
		t.Fatalf("expected repo update to verified, got %s", repo.updateStatus)
	}
	if storeRepo.updated == nil || storeRepo.updated.KYCStatus != enums.KYCStatusVerified {
		t.Fatalf("expected store kyc verified, got %v", storeRepo.updated)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(pub.events))
	}
	if event := pub.events[0]; event.EventType != enums.EventLicenseStatusChanged {
		t.Fatalf("unexpected event type %s", event.EventType)
	} else if payload, ok := event.Data.(payloads.LicenseStatusChangedEvent); !ok {
		t.Fatalf("unexpected payload type %T", event.Data)
	} else {
		if payload.Status != enums.LicenseStatusVerified {
			t.Fatalf("expected payload status verified, got %s", payload.Status)
		}
		if payload.Reason != "approved" {
			t.Fatalf("expected reason approved, got %q", payload.Reason)
		}
		if payload.LicenseID != license.ID {
			t.Fatalf("unexpected license id %s", payload.LicenseID)
		}
	}
}

func TestVerifyLicenseConflict(t *testing.T) {
	license := &models.License{
		ID:     uuid.New(),
		Status: enums.LicenseStatusVerified,
	}
	repo := &stubLicenseRepo{findResult: license}
	svc, _, _, _, _ := newServiceForTests(nil, nil, repo, nil)

	if _, err := svc.VerifyLicense(context.Background(), license.ID, enums.LicenseStatusRejected, "reject"); err == nil {
		t.Fatal("expected conflict error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestVerifyLicenseInvalidDecision(t *testing.T) {
	license := &models.License{
		ID:     uuid.New(),
		Status: enums.LicenseStatusPending,
	}
	repo := &stubLicenseRepo{findResult: license}
	svc, _, _, _, _ := newServiceForTests(nil, nil, repo, nil)

	if _, err := svc.VerifyLicense(context.Background(), license.ID, enums.LicenseStatusExpired, ""); err == nil {
		t.Fatal("expected validation error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation code, got %v", err)
	}
}

func TestVerifyLicenseStoreKYCRejected(t *testing.T) {
	license := &models.License{
		ID:      uuid.New(),
		Status:  enums.LicenseStatusPending,
		StoreID: uuid.New(),
	}
	storeRepo := &stubStoreRepo{store: &models.Store{ID: license.StoreID, KYCStatus: enums.KYCStatusVerified}}
	repo := &stubLicenseRepo{
		findResult: license,
		statusRows: []enums.LicenseStatus{enums.LicenseStatusRejected},
	}
	svc, _, _, storeRepo, _ := newServiceForTests(nil, nil, repo, storeRepo)

	if _, err := svc.VerifyLicense(context.Background(), license.ID, enums.LicenseStatusRejected, "denied"); err != nil {
		t.Fatalf("VerifyLicense returned error: %v", err)
	}
	if storeRepo.updated == nil || storeRepo.updated.KYCStatus != enums.KYCStatusRejected {
		t.Fatalf("expected store kyc rejected, got %v", storeRepo.updated)
	}
}

func containsRole(list []enums.MemberRole, target enums.MemberRole) bool {
	for _, candidate := range list {
		if candidate == target {
			return true
		}
	}
	return false
}

func TestListLicensesPagination(t *testing.T) {
	storeID := uuid.New()
	now := time.Now()
	rows := []models.License{
		{
			ID:             uuid.New(),
			StoreID:        storeID,
			UserID:         uuid.New(),
			Status:         enums.LicenseStatusPending,
			MediaID:        uuid.New(),
			GCSKey:         "media/a",
			IssuingState:   "OK",
			Number:         "123",
			Type:           enums.LicenseTypeProducer,
			CreatedAt:      now,
			UpdatedAt:      now,
			ExpirationDate: ptrTime(now.Add(24 * time.Hour)),
		},
		{
			ID:           uuid.New(),
			StoreID:      storeID,
			UserID:       uuid.New(),
			Status:       enums.LicenseStatusPending,
			MediaID:      uuid.New(),
			GCSKey:       "media/b",
			IssuingState: "OK",
			Number:       "456",
			Type:         enums.LicenseTypeProducer,
			CreatedAt:    now.Add(-time.Hour),
			UpdatedAt:    now.Add(-time.Hour),
		},
	}
	repo := &stubLicenseRepo{listRows: rows}
	svc, _, gcs, _, _ := newServiceForTests(nil, nil, repo, nil)
	gcs.url = "https://signed.example"

	resp, err := svc.ListLicenses(context.Background(), ListParams{
		StoreID: storeID,
		Params:  pagination.Params{Limit: 1},
	})
	if err != nil {
		t.Fatalf("ListLicenses returned error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Cursor == "" {
		t.Fatal("expected cursor for next page")
	}
	if resp.Items[0].SignedURL != "https://signed.example" {
		t.Fatalf("unexpected signed url %s", resp.Items[0].SignedURL)
	}
	if gcs.calls != 1 {
		t.Fatalf("expected gcs signed url called once, got %d", gcs.calls)
	}
	if repo.lastQuery.limit != 2 {
		t.Fatalf("expected query limit 2, got %d", repo.lastQuery.limit)
	}
}

func TestListLicensesInvalidCursor(t *testing.T) {
	storeID := uuid.New()
	svc, _, _, _, _ := newServiceForTests(nil, nil, &stubLicenseRepo{}, nil)

	if _, err := svc.ListLicenses(context.Background(), ListParams{
		StoreID: storeID,
		Params:  pagination.Params{Cursor: "badcursor"},
	}); err == nil {
		t.Fatal("expected validation error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
