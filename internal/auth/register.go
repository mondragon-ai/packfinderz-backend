package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RegisterRequest contains the payload required for onboarding a new store.
type RegisterRequest struct {
	FirstName   string          `json:"first_name" validate:"required"`
	LastName    string          `json:"last_name" validate:"required"`
	Email       string          `json:"email" validate:"required,email"`
	Password    string          `json:"password" validate:"required"`
	Phone       *string         `json:"phone,omitempty"`
	CompanyName string          `json:"company_name" validate:"required"`
	DBAName     *string         `json:"dba_name,omitempty"`
	StoreType   enums.StoreType `json:"store_type" validate:"required"`
	Address     types.Address   `json:"address" validate:"required"`
	AcceptTOS   bool            `json:"accept_tos"`
}

// RegisterService handles the onboarding transaction.
type RegisterService interface {
	Register(ctx context.Context, req RegisterRequest) error
}

// RegisterServiceParams packages the dependencies for the registration flow.
type RegisterServiceParams struct {
	DB             *db.Client
	PasswordConfig config.PasswordConfig
}

type registerService struct {
	db          *db.Client
	passwordCfg config.PasswordConfig
}

// NewRegisterService builds a registration service with the provided dependencies.
func NewRegisterService(params RegisterServiceParams) (RegisterService, error) {
	if params.DB == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "database client required")
	}
	return &registerService{
		db:          params.DB,
		passwordCfg: params.PasswordConfig,
	}, nil
}

func (s *registerService) Register(ctx context.Context, req RegisterRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "email is required")
	}
	if !req.StoreType.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid store type")
	}
	if !req.AcceptTOS {
		return pkgerrors.New(pkgerrors.CodeValidation, "accept_tos must be true")
	}

	passwordHash, err := security.HashPassword(req.Password, s.passwordCfg)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "hash password")
	}

	return s.db.WithTx(ctx, func(tx *gorm.DB) error {
		userRepo := users.NewRepository(tx)
		storeRepo := stores.NewRepository(tx)
		membershipRepo := memberships.NewRepository(tx)

		if _, err := userRepo.FindByEmail(ctx, email); err == nil {
			return pkgerrors.New(pkgerrors.CodeConflict, "email already registered")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "check user email")
		}

		user, err := userRepo.Create(ctx, users.CreateUserDTO{
			Email:        email,
			PasswordHash: passwordHash,
			FirstName:    req.FirstName,
			LastName:     req.LastName,
			Phone:        req.Phone,
		})
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "create user")
		}

		store, err := storeRepo.Create(ctx, stores.CreateStoreDTO{
			Type:        req.StoreType,
			CompanyName: req.CompanyName,
			DBAName:     req.DBAName,
			Address:     req.Address,
			Geom: types.GeographyPoint{
				Lat: req.Address.Lat,
				Lng: req.Address.Lng,
			},
			OwnerID: user.ID,
		})
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "create store")
		}

		if _, err := membershipRepo.CreateMembership(
			ctx,
			store.ID,
			user.ID,
			enums.MemberRoleOwner,
			nil,
			enums.MembershipStatusActive,
		); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "create membership")
		}

		if err := userRepo.UpdateStoreIDs(ctx, user.ID, []uuid.UUID{store.ID}); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "associate store with user")
		}

		return nil
	})
}
