package stores

import (
	"context"
	"errors"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type storeRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error)
}

// Service exposes store read operations.
type Service interface {
	GetByID(ctx context.Context, id uuid.UUID) (*StoreDTO, error)
}

type service struct {
	repo storeRepository
}

// NewService builds a store service.
func NewService(repo storeRepository) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	return &service{repo: repo}, nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*StoreDTO, error) {
	store, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
	}
	if store == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
	}
	return FromModel(store), nil
}
