package media

import (
	"context"
	"sort"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AttachmentReconciler syncs entity media references with the media_attachments table.
type AttachmentReconciler interface {
	Reconcile(ctx context.Context, tx *gorm.DB, entityType string, entityID, storeID uuid.UUID, oldMediaIDs, newMediaIDs []uuid.UUID) error
}

type attachmentReconciler struct {
	repo      attachmentRepository
	mediaRepo mediaRepository
}

type attachmentRepository interface {
	Create(ctx context.Context, tx *gorm.DB, attachment *models.MediaAttachment) error
	Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error
}

// NewAttachmentReconciler constructs the shared helper used by domain services.
func NewAttachmentReconciler(repo attachmentRepository, mediaRepo mediaRepository) (AttachmentReconciler, error) {
	if repo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "attachment repository required")
	}
	if mediaRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "media repository required")
	}
	return &attachmentReconciler{
		repo:      repo,
		mediaRepo: mediaRepo,
	}, nil
}

func (r *attachmentReconciler) Reconcile(ctx context.Context, tx *gorm.DB, entityType string, entityID, storeID uuid.UUID, oldMediaIDs, newMediaIDs []uuid.UUID) error {
	if entityType == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "entity_type required")
	}
	if entityID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "entity_id required")
	}
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store_id required")
	}
	if tx == nil || tx.Statement == nil || tx.Statement.ConnPool == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "transaction required")
	}

	oldSet := dedupe(oldMediaIDs)
	newSet := dedupe(newMediaIDs)

	toCreate := difference(newSet, oldSet)
	toDelete := difference(oldSet, newSet)

	ids := make([]uuid.UUID, 0, len(toCreate))
	for id := range toCreate {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, mediaID := range ids {
		mediaRow, err := r.mediaRepo.FindByID(ctx, mediaID)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "fetch media metadata")
		}
		if mediaRow.StoreID != storeID {
			return pkgerrors.New(pkgerrors.CodeValidation, "media belongs to different store")
		}
		attachment := &models.MediaAttachment{
			MediaID:    mediaID,
			EntityType: entityType,
			EntityID:   entityID,
			StoreID:    storeID,
			GCSKey:     mediaRow.GCSKey,
		}
		if err := r.repo.Create(ctx, tx, attachment); err != nil {
			return err
		}
	}

	ids = ids[:0]
	for id := range toDelete {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, mediaID := range ids {
		if err := r.repo.Delete(ctx, tx, entityType, entityID, mediaID); err != nil {
			return err
		}
	}

	return nil
}

func dedupe(ids []uuid.UUID) map[uuid.UUID]struct{} {
	set := make(map[uuid.UUID]struct{})
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		set[id] = struct{}{}
	}
	return set
}

func difference(a, b map[uuid.UUID]struct{}) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{})
	for id := range a {
		if _, ok := b[id]; !ok {
			out[id] = struct{}{}
		}
	}
	return out
}
