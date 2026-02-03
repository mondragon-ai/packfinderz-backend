package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	pkgmodels "github.com/angelmondragon/packfinderz-backend/pkg/db/models"
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
	DB                    *db.Client
	TxRunner              txRunner
	UserRepoFactory       registerUserRepoFactory
	StoreRepoFactory      registerStoreRepoFactory
	MembershipRepoFactory registerMembershipRepoFactory
	PasswordConfig        config.PasswordConfig
}

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type registerUserRepository interface {
	FindByEmail(ctx context.Context, email string) (*pkgmodels.User, error)
	Create(ctx context.Context, dto users.CreateUserDTO) (*pkgmodels.User, error)
}

type registerStoreRepository interface {
	Create(ctx context.Context, dto stores.CreateStoreDTO) (*pkgmodels.Store, error)
}

type registerMembershipRepository interface {
	CreateMembership(ctx context.Context, storeID, userID uuid.UUID, role enums.MemberRole, invitedBy *uuid.UUID, status enums.MembershipStatus) (*pkgmodels.StoreMembership, error)
}

type registerUserRepoFactory func(tx *gorm.DB) registerUserRepository
type registerStoreRepoFactory func(tx *gorm.DB) registerStoreRepository
type registerMembershipRepoFactory func(tx *gorm.DB) registerMembershipRepository

type registerService struct {
	txRunner          txRunner
	passwordCfg       config.PasswordConfig
	userFactory       registerUserRepoFactory
	storeFactory      registerStoreRepoFactory
	membershipFactory registerMembershipRepoFactory
}

// NewRegisterService builds a registration service with the provided dependencies.
func NewRegisterService(params RegisterServiceParams) (RegisterService, error) {
	if params.TxRunner == nil && params.DB == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "database client required")
	}
	runner := params.TxRunner
	if runner == nil {
		runner = params.DB
	}
	if params.UserRepoFactory == nil {
		params.UserRepoFactory = func(tx *gorm.DB) registerUserRepository {
			return users.NewRepository(tx)
		}
	}
	if params.StoreRepoFactory == nil {
		params.StoreRepoFactory = func(tx *gorm.DB) registerStoreRepository {
			return stores.NewRepository(tx)
		}
	}
	if params.MembershipRepoFactory == nil {
		params.MembershipRepoFactory = func(tx *gorm.DB) registerMembershipRepository {
			return memberships.NewRepository(tx)
		}
	}
	return &registerService{
		txRunner:          runner,
		passwordCfg:       params.PasswordConfig,
		userFactory:       params.UserRepoFactory,
		storeFactory:      params.StoreRepoFactory,
		membershipFactory: params.MembershipRepoFactory,
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

	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		userRepo := s.userFactory(tx)
		storeRepo := s.storeFactory(tx)
		membershipRepo := s.membershipFactory(tx)

		user, err := userRepo.FindByEmail(ctx, email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "check user email")
		}

		if user != nil {
			ok, verifyErr := security.VerifyPassword(req.Password, user.PasswordHash)
			if verifyErr != nil {
				return pkgerrors.Wrap(pkgerrors.CodeInternal, verifyErr, "verify password")
			}
			if !ok {
				return pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
			}
		} else {
			passwordHash, err := security.HashPassword(req.Password, s.passwordCfg)
			if err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "hash password")
			}

			user, err = userRepo.Create(ctx, users.CreateUserDTO{
				Email:        email,
				PasswordHash: passwordHash,
				FirstName:    req.FirstName,
				LastName:     req.LastName,
				Phone:        req.Phone,
			})
			if err != nil {
				fmt.Printf("CREATE USER DB ERROR: %+v\n", err)
				return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "create user")
			}
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

		return nil
	})
}
