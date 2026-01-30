package media

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubAttachmentRepo struct {
	created []*models.MediaAttachment
	deleted []uuid.UUID
}

func (s *stubAttachmentRepo) Create(ctx context.Context, tx *gorm.DB, attachment *models.MediaAttachment) error {
	s.created = append(s.created, attachment)
	return nil
}

func (s *stubAttachmentRepo) Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error {
	s.deleted = append(s.deleted, mediaID)
	return nil
}

type stubAttachmentMediaRepo struct {
	rows    map[uuid.UUID]*models.Media
	findErr error
}

func (s *stubAttachmentMediaRepo) Create(ctx context.Context, media *models.Media) (*models.Media, error) {
	return media, nil
}

func (s *stubAttachmentMediaRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (s *stubAttachmentMediaRepo) List(ctx context.Context, opts listQuery) ([]models.Media, error) {
	return nil, nil
}

func (s *stubAttachmentMediaRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if media, ok := s.rows[id]; ok {
		return media, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubAttachmentMediaRepo) MarkDeleted(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	return nil
}

func TestAttachmentReconcilerCreatesAndDeletesAttachments(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	entityID := uuid.New()
	entityType := "product"

	existing := uuid.New()
	preserved := uuid.New()
	added := uuid.New()

	stubMedia := &stubAttachmentMediaRepo{
		rows: map[uuid.UUID]*models.Media{
			existing:  {ID: existing, StoreID: storeID, GCSKey: "existing"},
			preserved: {ID: preserved, StoreID: storeID, GCSKey: "preserved"},
			added:     {ID: added, StoreID: storeID, GCSKey: "added"},
		},
	}

	repo := &stubAttachmentRepo{}
	reconciler, err := NewAttachmentReconciler(repo, stubMedia)
	if err != nil {
		t.Fatalf("NewAttachmentReconciler: %v", err)
	}

	tx := testTx()
	err = reconciler.Reconcile(context.Background(), tx, entityType, entityID, storeID, []uuid.UUID{existing, preserved}, []uuid.UUID{preserved, added})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(repo.created) != 1 || repo.created[0].MediaID != added {
		t.Fatalf("expected added attachments %#v", repo.created)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != existing {
		t.Fatalf("expected existing attachment deleted, got %#v", repo.deleted)
	}
}

func TestAttachmentReconcilerRejectsCrossStoreMedia(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	entityID := uuid.New()

	mediaID := uuid.New()
	stubMedia := &stubAttachmentMediaRepo{
		rows: map[uuid.UUID]*models.Media{
			mediaID: {ID: mediaID, StoreID: uuid.New(), GCSKey: "other"},
		},
	}

	repo := &stubAttachmentRepo{}
	reconciler, err := NewAttachmentReconciler(repo, stubMedia)
	if err != nil {
		t.Fatalf("NewAttachmentReconciler: %v", err)
	}

	err = reconciler.Reconcile(context.Background(), testTx(), "license", entityID, storeID, nil, []uuid.UUID{mediaID})
	if err == nil {
		t.Fatalf("expected error when media belongs to different store")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("expected no attachment created, got %+v", repo.created)
	}
}

func TestAttachmentReconcilerRequiresTransaction(t *testing.T) {
	t.Parallel()

	repo := &stubAttachmentRepo{}
	stubMedia := &stubAttachmentMediaRepo{rows: map[uuid.UUID]*models.Media{}}
	reconciler, err := NewAttachmentReconciler(repo, stubMedia)
	if err != nil {
		t.Fatalf("NewAttachmentReconciler: %v", err)
	}

	err = reconciler.Reconcile(context.Background(), nil, "store", uuid.New(), uuid.New(), nil, nil)
	if err == nil {
		t.Fatalf("expected transaction validation error")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAttachmentReconcilerNoChanges(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	entityID := uuid.New()
	mediaID := uuid.New()

	stubMedia := &stubAttachmentMediaRepo{
		rows: map[uuid.UUID]*models.Media{
			mediaID: {ID: mediaID, StoreID: storeID, GCSKey: "gcs"},
		},
	}

	repo := &stubAttachmentRepo{}
	reconciler, err := NewAttachmentReconciler(repo, stubMedia)
	if err != nil {
		t.Fatalf("NewAttachmentReconciler: %v", err)
	}

	err = reconciler.Reconcile(context.Background(), testTx(), "product", entityID, storeID, []uuid.UUID{mediaID}, []uuid.UUID{mediaID})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	if len(repo.created) != 0 || len(repo.deleted) != 0 {
		t.Fatalf("expected no changes, got %+v %+v", repo.created, repo.deleted)
	}
}

type fakeConnPool struct{}

func (fakeConnPool) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error) {
	return nil, nil
}

func (fakeConnPool) ExecContext(_ context.Context, _ string, _ ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (fakeConnPool) QueryContext(_ context.Context, _ string, _ ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (fakeConnPool) QueryRowContext(_ context.Context, _ string, _ ...interface{}) *sql.Row {
	return nil
}

func testTx() *gorm.DB {
	return &gorm.DB{Statement: &gorm.Statement{ConnPool: fakeConnPool{}}}
}
