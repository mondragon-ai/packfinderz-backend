package stores

import (
	"context"
	"errors"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type storeRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error)
	Update(ctx context.Context, store *models.Store) error
}

type membershipsRepository interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

// Service exposes store read and update operations.
type Service interface {
	GetByID(ctx context.Context, id uuid.UUID) (*StoreDTO, error)
	Update(ctx context.Context, userID, storeID uuid.UUID, input UpdateStoreInput) (*StoreDTO, error)
}

type service struct {
	repo        storeRepository
	memberships membershipsRepository
}

// NewService builds a store service with the provided repositories.
func NewService(repo storeRepository, memberships membershipsRepository) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	return &service{
		repo:        repo,
		memberships: memberships,
	}, nil
}

// UpdateStoreInput captures the allowed store fields for mutation.
type UpdateStoreInput struct {
	CompanyName *string
	Description *string
	Phone       *string
	Email       *string
	Social      *types.Social
	BannerURL   *string
	LogoURL     *string
	Ratings     *map[string]int
	Categories  *[]string
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

func (s *service) Update(ctx context.Context, userID, storeID uuid.UUID, input UpdateStoreInput) (*StoreDTO, error) {
	allowedRoles := []enums.MemberRole{enums.MemberRoleOwner, enums.MemberRoleManager}
	ok, err := s.memberships.UserHasRole(ctx, userID, storeID, allowedRoles...)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership")
	}
	if !ok {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	store, err := s.repo.FindByID(ctx, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
	}

	if input.CompanyName != nil {
		store.CompanyName = *input.CompanyName
	}
	if input.Description != nil {
		store.Description = cloneStringPtr(input.Description)
	}
	if input.Phone != nil {
		store.Phone = cloneStringPtr(input.Phone)
	}
	if input.Email != nil {
		store.Email = cloneStringPtr(input.Email)
	}
	if input.Social != nil {
		store.Social = cloneSocial(input.Social)
	}
	if input.BannerURL != nil {
		store.BannerURL = cloneStringPtr(input.BannerURL)
	}
	if input.LogoURL != nil {
		store.LogoURL = cloneStringPtr(input.LogoURL)
	}
	if input.Ratings != nil {
		if *input.Ratings == nil {
			store.Ratings = nil
		} else {
			store.Ratings = cloneRatings(*input.Ratings)
		}
	}
	if input.Categories != nil {
		store.Categories = cloneCategories(*input.Categories)
	}

	if err := s.repo.Update(ctx, store); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store")
	}
	return FromModel(store), nil
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cpy := *value
	return &cpy
}

func cloneSocial(value *types.Social) *types.Social {
	if value == nil {
		return nil
	}
	cpy := *value
	return &cpy
}

func cloneRatings(value map[string]int) types.Ratings {
	if value == nil {
		return nil
	}
	res := make(types.Ratings, len(value))
	for k, v := range value {
		res[k] = v
	}
	return res
}

func cloneCategories(value []string) pq.StringArray {
	if value == nil {
		return nil
	}
	res := make(pq.StringArray, len(value))
	copy(res, value)
	return res
}
