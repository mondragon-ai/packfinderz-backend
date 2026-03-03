package ads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service exposes ads-oriented business logic.
type Service interface {
	CreateAd(ctx context.Context, input CreateAdInput) (*AdDTO, error)
	ListAds(ctx context.Context, input ListAdsInput) (AdListResult, error)
	GetAdDetail(ctx context.Context, input GetAdDetailInput) (*AdDetail, error)
	ServeAd(ctx context.Context, input ServeAdInput) (*ServeAdResult, error)
	TrackImpression(ctx context.Context, input TrackImpressionInput) error
	TrackClick(ctx context.Context, input TrackClickInput) (*TrackClickResult, error)
}

type redisStore interface {
	Get(context.Context, string) (string, error)
	SetNX(context.Context, string, any, time.Duration) (bool, error)
	Incr(context.Context, string) (int64, error)
	IncrByFloat(context.Context, string, float64) (float64, error)
}

// GetAdDetailInput captures the parameters required to build the detail payload.
type GetAdDetailInput struct {
	StoreID uuid.UUID
	AdID    uuid.UUID
	Start   time.Time
	End     time.Time
}

// ServiceParams holds the ads service dependencies.
type ServiceParams struct {
	Repo                 *Repository
	DB                   *db.Client
	AttachmentReconciler media.AttachmentReconciler
	Analytics            analytics.Service
	Redis                redisStore
	TokenSecret          string
	TokenTTL             time.Duration
}

type service struct {
	repo        *Repository
	db          *db.Client
	attachments media.AttachmentReconciler
	analytics   analytics.Service
	redis       redisStore
	tokenSecret string
	tokenTTL    time.Duration
}

// NewService constructs an ads service with the provided dependencies.
func NewService(params ServiceParams) (Service, error) {
	if params.Repo == nil {
		return nil, fmt.Errorf("ads repository required")
	}
	if params.DB == nil {
		return nil, fmt.Errorf("db client required")
	}
	if params.AttachmentReconciler == nil {
		return nil, fmt.Errorf("attachment reconciler required")
	}
	if params.Analytics == nil {
		return nil, fmt.Errorf("analytics service required")
	}
	if params.Redis == nil {
		return nil, fmt.Errorf("redis client required")
	}
	if strings.TrimSpace(params.TokenSecret) == "" {
		return nil, fmt.Errorf("ads token secret required")
	}
	if params.TokenTTL <= 0 {
		return nil, fmt.Errorf("ads token ttl required")
	}
	return &service{
		repo:        params.Repo,
		db:          params.DB,
		attachments: params.AttachmentReconciler,
		analytics:   params.Analytics,
		redis:       params.Redis,
		tokenSecret: strings.TrimSpace(params.TokenSecret),
		tokenTTL:    params.TokenTTL,
	}, nil
}

func (s *service) CreateAd(ctx context.Context, input CreateAdInput) (*AdDTO, error) {
	if input.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store_id is required")
	}

	var created *models.Ad
	err := s.db.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		ad, err := repo.CreateAd(ctx, input)
		if err != nil {
			return err
		}
		created = ad

		mediaIDs := collectCreativeMediaIDs(input.Creatives)
		if len(mediaIDs) > 0 {
			if err := s.attachments.Reconcile(ctx, tx, models.AttachmentEntityAd, ad.ID, input.StoreID, nil, mediaIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	dto := MapAdToDTO(created)
	return &dto, nil
}

func (s *service) ListAds(ctx context.Context, input ListAdsInput) (AdListResult, error) {
	return s.repo.ListAds(ctx, input)
}

func (s *service) GetAdDetail(ctx context.Context, input GetAdDetailInput) (*AdDetail, error) {
	if input.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store_id is required")
	}
	if input.AdID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "ad_id is required")
	}
	if input.Start.IsZero() || input.End.IsZero() {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "timeframe required")
	}
	if input.End.Before(input.Start) {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "end must be after start")
	}

	ad, err := s.repo.GetAdByID(ctx, input.StoreID, input.AdID)
	if err != nil {
		return nil, err
	}

	analyticsReq := types.AdQueryRequest{
		VendorStoreID: input.StoreID.String(),
		AdID:          input.AdID.String(),
		Start:         input.Start,
		End:           input.End,
	}

	analyticsResp, err := s.analytics.QueryAd(ctx, analyticsReq)
	if err != nil {
		return nil, err
	}
	if analyticsResp == nil {
		analyticsResp = &types.AdQueryResponse{}
	}

	return &AdDetail{
		Ad:        MapAdToDTO(ad),
		Analytics: *analyticsResp,
	}, nil
}

func collectCreativeMediaIDs(creatives []AdCreativeInput) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(creatives))
	for _, creative := range creatives {
		if creative.MediaID == nil || *creative.MediaID == uuid.Nil {
			continue
		}
		ids = append(ids, *creative.MediaID)
	}
	return ids
}
