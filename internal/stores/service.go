package stores

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
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
	ListStoreUsers(ctx context.Context, storeID uuid.UUID) ([]memberships.StoreUserDTO, error)
	GetMembership(ctx context.Context, userID, storeID uuid.UUID) (*models.StoreMembership, error)
	CreateMembership(ctx context.Context, storeID, userID uuid.UUID, role enums.MemberRole, invitedBy *uuid.UUID, status enums.MembershipStatus) (*models.StoreMembership, error)
	DeleteMembership(ctx context.Context, storeID, userID uuid.UUID) error
	CountMembersWithRoles(ctx context.Context, storeID uuid.UUID, roles ...enums.MemberRole) (int64, error)
}

type usersRepository interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, dto users.CreateUserDTO) (*models.User, error)
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error
}

// Service exposes store operations.
type Service interface {
	GetByID(ctx context.Context, id uuid.UUID) (*StoreDTO, error)
	Update(ctx context.Context, userID, storeID uuid.UUID, input UpdateStoreInput) (*StoreDTO, error)
	ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error)
	InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input InviteUserInput) (*memberships.StoreUserDTO, string, error)
	RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error
}

type service struct {
	repo        storeRepository
	memberships membershipsRepository
	users       usersRepository
	passwordCfg config.PasswordConfig
}

// NewService builds a store service with the provided repositories.
func NewService(repo storeRepository, memberships membershipsRepository, usersRepo usersRepository, passwordCfg config.PasswordConfig) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	if usersRepo == nil {
		return nil, fmt.Errorf("users repository required")
	}
	return &service{
		repo:        repo,
		memberships: memberships,
		users:       usersRepo,
		passwordCfg: passwordCfg,
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

// InviteUserInput captures the data required to invite a store user.
type InviteUserInput struct {
	Email     string
	FirstName string
	LastName  string
	Role      enums.MemberRole
}

func (s *service) createNewUser(ctx context.Context, email, firstName, lastName string, storeID uuid.UUID) (*models.User, string, error) {
	if !strings.Contains(email, "@") {
		return nil, "", pkgerrors.New(pkgerrors.CodeValidation, "invalid email")
	}

	tempPassword, err := security.GenerateTempPassword(16)
	if err != nil {
		return nil, "", pkgerrors.Wrap(pkgerrors.CodeInternal, err, "generate temp password")
	}
	hash, err := security.HashPassword(tempPassword, s.passwordCfg)
	if err != nil {
		return nil, "", pkgerrors.Wrap(pkgerrors.CodeInternal, err, "hash password")
	}

	user, err := s.users.Create(ctx, users.CreateUserDTO{
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		PasswordHash: hash,
		StoreIDs:     []uuid.UUID{storeID},
	})
	if err != nil {
		return nil, "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create user")
	}
	return user, tempPassword, nil
}

func (s *service) resetUserPassword(ctx context.Context, userID uuid.UUID) (string, error) {
	tempPassword, err := security.GenerateTempPassword(16)
	if err != nil {
		return "", pkgerrors.Wrap(pkgerrors.CodeInternal, err, "generate temp password")
	}
	hash, err := security.HashPassword(tempPassword, s.passwordCfg)
	if err != nil {
		return "", pkgerrors.Wrap(pkgerrors.CodeInternal, err, "hash password")
	}
	if err := s.users.UpdatePasswordHash(ctx, userID, hash); err != nil {
		return "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update user password")
	}
	return tempPassword, nil
}

func (s *service) fetchStoreUser(ctx context.Context, storeID, userID uuid.UUID) (*memberships.StoreUserDTO, error) {
	users, err := s.memberships.ListStoreUsers(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list store users")
	}
	for _, u := range users {
		if u.UserID == userID {
			return &u, nil
		}
	}
	return nil, pkgerrors.New(pkgerrors.CodeNotFound, "membership not found")
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*StoreDTO, error) {
	store, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
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

func (s *service) ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	allowedRoles := []enums.MemberRole{enums.MemberRoleOwner, enums.MemberRoleManager}
	ok, err := s.memberships.UserHasRole(ctx, userID, storeID, allowedRoles...)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership")
	}
	if !ok {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	users, err := s.memberships.ListStoreUsers(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list store users")
	}
	return users, nil
}

func (s *service) InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	allowedRoles := []enums.MemberRole{enums.MemberRoleOwner, enums.MemberRoleManager}
	ok, err := s.memberships.UserHasRole(ctx, inviterID, storeID, allowedRoles...)
	if err != nil {
		return nil, "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership")
	}
	if !ok {
		return nil, "", pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return nil, "", pkgerrors.New(pkgerrors.CodeValidation, "email is required")
	}
	if !input.Role.IsValid() {
		return nil, "", pkgerrors.New(pkgerrors.CodeValidation, "invalid role")
	}

	var usr *models.User
	var tempPassword string
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			usr, tempPassword, err = s.createNewUser(ctx, email, input.FirstName, input.LastName, storeID)
			if err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup user")
		}
	} else {
		usr = user
	}

	membership, err := s.memberships.GetMembership(ctx, usr.ID, storeID)
	if err == nil && membership != nil {
		dto, fetchErr := s.fetchStoreUser(ctx, storeID, usr.ID)
		if fetchErr != nil {
			return nil, "", fetchErr
		}
		return dto, "", nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership")
	}

	if tempPassword == "" {
		tempPassword, err = s.resetUserPassword(ctx, usr.ID)
		if err != nil {
			return nil, "", err
		}
	}

	if _, err := s.memberships.CreateMembership(ctx, storeID, usr.ID, input.Role, &inviterID, enums.MembershipStatusInvited); err != nil {
		return nil, "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create membership")
	}

	dto, fetchErr := s.fetchStoreUser(ctx, storeID, usr.ID)
	if fetchErr != nil {
		return nil, "", fetchErr
	}
	return dto, tempPassword, nil
}

func (s *service) RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error {
	allowedRoles := []enums.MemberRole{enums.MemberRoleOwner, enums.MemberRoleManager}
	ok, err := s.memberships.UserHasRole(ctx, actorID, storeID, allowedRoles...)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership")
	}
	if !ok {
		return pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	membership, err := s.memberships.GetMembership(ctx, targetUserID, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.New(pkgerrors.CodeNotFound, "membership not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load membership")
	}

	if membership.Role == enums.MemberRoleOwner {
		count, err := s.memberships.CountMembersWithRoles(ctx, storeID, enums.MemberRoleOwner)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "count owners")
		}
		if count <= 1 {
			return pkgerrors.New(pkgerrors.CodeConflict, "cannot remove last owner")
		}
	}

	if err := s.memberships.DeleteMembership(ctx, storeID, targetUserID); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "delete membership")
	}

	return nil
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
