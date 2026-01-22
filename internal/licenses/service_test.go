package licenses

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubLicenseRepo struct {
	created   *models.License
	listRows  []models.License
	listErr   error
	lastQuery listQuery
	err       error
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

func newServiceForTests(media *models.Media, members *stubMemberships, repo *stubLicenseRepo) (Service, *stubLicenseRepo, *stubGCS) {
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
	svc, err := NewService(repo, mediaRepo, members, gcsStub, "bucket", time.Minute)
	if err != nil {
		panic(err)
	}
	return svc, repo, gcsStub
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
	svc, repo, _ := newServiceForTests(mediaRow, members, nil)

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
}

func TestCreateLicenseRequiresMembership(t *testing.T) {
	storeID := uuid.New()
	members := &stubMemberships{ok: false}
	svc, _, _ := newServiceForTests(&models.Media{StoreID: storeID, Status: enums.MediaStatusUploaded, Kind: enums.MediaKindLicenseDoc, MimeType: "application/pdf"}, members, nil)

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
	svc, err := NewService(repo, mediaRepo, &stubMemberships{ok: true}, gcs, "bucket", time.Minute)
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
	svc, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil)

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
	svc, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil)

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
	svc, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil)

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
	svc, _, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true}, nil)

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
	svc, _, _ := newServiceForTests(mediaRow, members, nil)

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
	svc, _, gcs := newServiceForTests(nil, nil, repo)
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
	svc, _, _ := newServiceForTests(nil, nil, &stubLicenseRepo{})

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
