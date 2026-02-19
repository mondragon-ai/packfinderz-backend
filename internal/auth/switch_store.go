package auth

import (
	"context"
	"errors"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SwitchStoreInput captures the data required to switch stores.
type SwitchStoreInput struct {
	UserID        uuid.UUID
	StoreID       uuid.UUID
	AccessTokenID string
}

// SwitchStoreResult returns the tokens issued after switching stores.
type SwitchStoreResult struct {
	AccessToken  string
	RefreshToken string
	Store        StoreSummary
}

type storeLastLoginUpdater interface {
	UpdateLastLoggedInAt(ctx context.Context, storeID uuid.UUID) error
}

type switchStoreService struct {
	memberships  switchMembershipsRepository
	session      switchSessionRotator
	jwtCfg       config.JWTConfig
	storeUpdater storeLastLoginUpdater
}

type switchMembershipsRepository interface {
	GetMembershipWithStore(ctx context.Context, userID, storeID uuid.UUID) (*memberships.MembershipWithStore, error)
}

type switchSessionRotator interface {
	Rotate(ctx context.Context, oldAccessID, provided string) (string, string, error)
	RefreshToken(ctx context.Context, accessID string) (string, error)
}

// SwitchStoreServiceParams bundles dependencies for the switch flow.
type SwitchStoreServiceParams struct {
	MembershipsRepo switchMembershipsRepository
	SessionManager  switchSessionRotator
	JWTConfig       config.JWTConfig
	StoreRepo       storeLastLoginUpdater
}

// NewSwitchStoreService constructs the service.
func NewSwitchStoreService(params SwitchStoreServiceParams) (SwitchStoreService, error) {
	if params.MembershipsRepo == nil {
		return nil, errors.New("memberships repository required")
	}
	if params.SessionManager == nil {
		return nil, errors.New("session manager required")
	}
	if params.StoreRepo == nil {
		return nil, errors.New("store repository required")
	}
	return &switchStoreService{
		memberships:  params.MembershipsRepo,
		session:      params.SessionManager,
		jwtCfg:       params.JWTConfig,
		storeUpdater: params.StoreRepo,
	}, nil
}

// SwitchStoreService is the interface exposed to the controller.
type SwitchStoreService interface {
	Switch(ctx context.Context, input SwitchStoreInput) (*SwitchStoreResult, error)
}

func (s *switchStoreService) Switch(ctx context.Context, input SwitchStoreInput) (*SwitchStoreResult, error) {
	membership, err := s.memberships.GetMembershipWithStore(ctx, input.UserID, input.StoreID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeForbidden, "store membership required")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "lookup membership")
	}
	if membership.Status != enums.MembershipStatusActive {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "store membership inactive")
	}

	if err := s.storeUpdater.UpdateLastLoggedInAt(ctx, input.StoreID); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "update store last login")
	}

	refreshToken, err := s.session.RefreshToken(ctx, input.AccessTokenID)
	if err != nil {
		if errors.Is(err, session.ErrInvalidRefreshToken) {
			return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, "invalid session")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "load refresh token")
	}

	newAccessID, newRefreshToken, err := s.session.Rotate(ctx, input.AccessTokenID, refreshToken)
	if err != nil {
		if errors.Is(err, session.ErrInvalidRefreshToken) {
			return nil, pkgerrors.New(pkgerrors.CodeUnauthorized, "invalid refresh token")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "rotate session")
	}

	payload := pkgAuth.AccessTokenPayload{
		UserID:        input.UserID,
		ActiveStoreID: &input.StoreID,
		Role:          membership.Role,
		StoreType:     &membership.StoreType,
		JTI:           newAccessID,
	}

	accessToken, err := pkgAuth.MintAccessToken(s.jwtCfg, time.Now().UTC(), payload)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "mint jwt")
	}

	result := &SwitchStoreResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		Store: StoreSummary{
			ID:   membership.StoreID,
			Name: membership.StoreName,
			Type: membership.StoreType,
		},
	}

	return result, nil
}
