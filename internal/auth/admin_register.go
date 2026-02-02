package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
	"gorm.io/gorm"
)

// AdminRegisterRequest contains the credentials for the dev-only admin registration flow.
type AdminRegisterRequest struct {
	FirstNames string `json:"first_names" validate:"required"`
	LastName   string `json:"last_name" validate:"required"`
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
}

// AdminRegisterService handles creating dev admin users.
type AdminRegisterService interface {
	Register(ctx context.Context, req AdminRegisterRequest) (*users.UserDTO, error)
}

// AdminRegisterServiceParams names the dependencies for the admin register flow.
type AdminRegisterServiceParams struct {
	DB             *db.Client
	PasswordConfig config.PasswordConfig
}

type adminRegisterService struct {
	db          *db.Client
	passwordCfg config.PasswordConfig
}

// NewAdminRegisterService builds a dev admin registration service.
func NewAdminRegisterService(params AdminRegisterServiceParams) (AdminRegisterService, error) {
	if params.DB == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "database client required")
	}
	return &adminRegisterService{
		db:          params.DB,
		passwordCfg: params.PasswordConfig,
	}, nil
}

func (s *adminRegisterService) Register(ctx context.Context, req AdminRegisterRequest) (*users.UserDTO, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "email is required")
	}
	firstNames := strings.TrimSpace(req.FirstNames)
	if firstNames == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "first_names is required")
	}
	lastName := strings.TrimSpace(req.LastName)
	if lastName == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "last_name is required")
	}

	passwordHash, err := security.HashPassword(req.Password, s.passwordCfg)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "hash password")
	}

	var created *users.UserDTO
	err = s.db.WithTx(ctx, func(tx *gorm.DB) error {
		userRepo := users.NewRepository(tx)

		if _, err := userRepo.FindByEmail(ctx, email); err == nil {
			return pkgerrors.New(pkgerrors.CodeConflict, "email already registered")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "check user email")
		}

		user, err := userRepo.Create(ctx, users.CreateUserDTO{
			Email:        email,
			PasswordHash: passwordHash,
			FirstName:    firstNames,
			LastName:     lastName,
			IsActive:     boolRef(true),
			SystemRole:   stringRef("admin"),
		})
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeInternal, err, "create user")
		}

		created = users.FromModel(user)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func boolRef(v bool) *bool {
	return &v
}

func stringRef(v string) *string {
	return &v
}
