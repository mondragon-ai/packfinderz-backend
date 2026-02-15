package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const invalidCredentialsMessage = "invalid credentials"

// Service defines the behavior needed by the auth controller.
type Service interface {
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	AdminLogin(ctx context.Context, req LoginRequest) (*AdminLoginResponse, error)
}

type service struct {
	users       userRepository
	memberships membershipsRepository
	session     sessionManager
	jwtCfg      config.JWTConfig
}

type userRepository interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error
}

type membershipsRepository interface {
	ListUserStores(ctx context.Context, userID uuid.UUID) ([]memberships.MembershipWithStore, error)
}

type sessionManager interface {
	Generate(ctx context.Context, accessID string) (string, error)
}

// ServiceParams bundles the dependencies required to build an auth service.
type ServiceParams struct {
	UserRepo        userRepository
	MembershipsRepo membershipsRepository
	SessionManager  sessionManager
	JWTConfig       config.JWTConfig
}

// NewService constructs a login service with the provided dependencies.
func NewService(params ServiceParams) (Service, error) {
	if params.UserRepo == nil {
		return nil, fmt.Errorf("user repository is required")
	}
	if params.MembershipsRepo == nil {
		return nil, fmt.Errorf("memberships repository is required")
	}
	if params.SessionManager == nil {
		return nil, fmt.Errorf("session manager is required")
	}
	return &service{
		users:       params.UserRepo,
		memberships: params.MembershipsRepo,
		session:     params.SessionManager,
		jwtCfg:      params.JWTConfig,
	}, nil
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.authenticate(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	memberships, err := s.memberships.ListUserStores(ctx, user.ID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "list stores")
	}
	systemRole := normalizedSystemRole(user.SystemRole)

	if len(memberships) == 0 && systemRole == "" {
		return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
	}

	now, err := s.recordLogin(ctx, user)
	if err != nil {
		return nil, err
	}

	stores := make([]StoreSummary, 0, len(memberships))
	for _, m := range memberships {
		stores = append(stores, StoreSummary{
			ID:   m.StoreID,
			Name: m.StoreName,
			Type: m.StoreType,
		})
	}

	activeStoreID := (*uuid.UUID)(nil)
	if len(memberships) == 1 {
		id := memberships[0].StoreID
		activeStoreID = &id
	}

	var storeTypePtr *enums.StoreType
	var role enums.MemberRole
	if len(memberships) > 0 {
		id := memberships[0].StoreID
		primary := memberships[0]
		activeStoreID = &id
		role = primary.Role
		storeTypeVal := primary.StoreType
		storeTypePtr = &storeTypeVal
	}

	if systemRole != "" {
		parsedRole, err := enums.ParseMemberRole(systemRole)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "invalid system role")
		}
		role = parsedRole
	}

	if !role.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
	}

	accessID := session.NewAccessID()
	tokenPayload := pkgAuth.AccessTokenPayload{
		UserID:        user.ID,
		ActiveStoreID: activeStoreID,
		Role:          role,
		StoreType:     storeTypePtr,
		JTI:           accessID,
	}
	accessToken, err := pkgAuth.MintAccessToken(s.jwtCfg, now, tokenPayload)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "mint jwt")
	}
	refreshToken, err := s.session.Generate(ctx, accessID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "store refresh token")
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Stores:       stores,
		User:         users.FromModel(user),
	}, nil
}

func (s *service) AdminLogin(ctx context.Context, req LoginRequest) (*AdminLoginResponse, error) {
	user, err := s.authenticate(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	if normalizedSystemRole(user.SystemRole) != string(enums.MemberRoleAdmin) {
		return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
	}

	now, err := s.recordLogin(ctx, user)
	if err != nil {
		return nil, err
	}

	accessID := session.NewAccessID()
	tokenPayload := pkgAuth.AccessTokenPayload{
		UserID: user.ID,
		Role:   enums.MemberRoleAdmin,
		JTI:    accessID,
	}
	accessToken, err := pkgAuth.MintAccessToken(s.jwtCfg, now, tokenPayload)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "mint jwt")
	}
	refreshToken, err := s.session.Generate(ctx, accessID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "store refresh token")
	}

	return &AdminLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         users.FromModel(user),
	}, nil
}

func (s *service) authenticate(ctx context.Context, email, password string) (*models.User, error) {
	input := strings.TrimSpace(email)
	if input == "" {
		return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
	}
	user, err := s.users.FindByEmail(ctx, strings.ToLower(input))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "lookup user")
	}

	valid, err := security.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "verify password")
	}
	if !valid || !user.IsActive {
		return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, invalidCredentialsMessage)
	}
	return user, nil
}

func (s *service) recordLogin(ctx context.Context, user *models.User) (time.Time, error) {
	now := time.Now().UTC()
	if err := s.users.UpdateLastLogin(ctx, user.ID, now); err != nil {
		return time.Time{}, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "update last login")
	}
	user.LastLoginAt = &now
	return now, nil
}

func normalizedSystemRole(role *string) string {
	if role == nil {
		return ""
	}
	value := strings.TrimSpace(*role)
	if value == "" {
		return ""
	}
	return strings.ToLower(value)
}
