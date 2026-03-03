package ads

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/ads/token"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	serveCandidateLimit = 20
	dedupeTTL           = 10 * time.Minute
)

var ErrRedisUnavailable = errors.New("ads redis unavailable")

type ServeAdInput struct {
	Placement    enums.AdPlacement
	BuyerStoreID uuid.UUID
	RequestID    string
	Now          time.Time
}

type ServeAdResult struct {
	AdID       uuid.UUID          `json:"ad_id"`
	Placement  enums.AdPlacement  `json:"placement"`
	TargetType enums.AdTargetType `json:"target_type"`
	TargetID   uuid.UUID          `json:"target_id"`
	Creative   AdCreativeDTO      `json:"creative"`
	RequestID  string             `json:"request_id"`
	ViewToken  string             `json:"view_token"`
	ClickToken string             `json:"click_token"`
}

type TrackImpressionInput struct {
	Token        string
	RequestID    string
	BuyerStoreID uuid.UUID
	Now          time.Time
}

type TrackClickInput struct {
	Token        string
	RequestID    string
	BuyerStoreID uuid.UUID
	Now          time.Time
}

type TrackClickResult struct {
	DestinationURL string
}

func (s *service) ServeAd(ctx context.Context, input ServeAdInput) (*ServeAdResult, error) {
	if input.Placement == "" || !input.Placement.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "placement is required")
	}
	if input.BuyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id required")
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RequestID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "request_id is required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}

	candidates, err := s.repo.ListEligibleAdsForServe(ctx, input.Placement, serveCandidateLimit, input.Now)
	if err != nil {
		return nil, err
	}

	day := formatDay(input.Now)
	var winner *models.Ad
	var winnerScore uint64
	highestBid := int64(-1)
	for i := range candidates {
		ad := &candidates[i]
		if len(ad.Creatives) == 0 {
			continue
		}
		if ad.DailyBudgetCents <= 0 {
			continue
		}
		spend, err := s.readDailySpend(ctx, ad.ID, day)
		if err != nil {
			return nil, err
		}
		increment := float64(ad.BidCents) / 1000.0
		if spend+increment > float64(ad.DailyBudgetCents) {
			continue
		}

		score := tieBreakScore(input.RequestID, input.Placement, day, ad.ID)
		if winner == nil || ad.BidCents > highestBid || (ad.BidCents == highestBid && score < winnerScore) {
			winner = ad
			highestBid = ad.BidCents
			winnerScore = score
		}
	}

	if winner == nil {
		return nil, nil
	}

	creative := &winner.Creatives[0]
	return s.buildServeResult(ctx, winner, creative, input.RequestID, input.Placement, input.BuyerStoreID, input.Now)
}

func (s *service) TrackImpression(ctx context.Context, input TrackImpressionInput) error {
	if err := input.normalize(); err != nil {
		return err
	}
	payload, err := token.ParseToken(s.tokenSecret, input.Token)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid view token")
	}
	if payload.EventType != enums.AdEventFactTypeImpression {
		return pkgerrors.New(pkgerrors.CodeValidation, "impression token required")
	}
	if payload.RequestID != input.RequestID {
		return pkgerrors.New(pkgerrors.CodeValidation, "request_id mismatch")
	}
	if payload.BuyerStoreID != input.BuyerStoreID {
		return pkgerrors.New(pkgerrors.CodeValidation, "buyer store mismatch")
	}

	day := formatDay(input.Now)
	if day == "" {
		day = formatDay(time.Now().UTC())
	}

	key := impDedupeKey(input.RequestID, payload.AdID, string(payload.Placement))
	set, err := s.redis.SetNX(ctx, key, "1", dedupeTTL)
	if err != nil {
		return wrapRedisErr("dedupe impression", err)
	}
	if !set {
		return nil
	}

	if _, err := s.redis.Incr(ctx, counterKey("imps", payload.AdID, day)); err != nil {
		return wrapRedisErr("increment impressions", err)
	}
	if _, err := s.redis.IncrByFloat(ctx, counterKey("spend", payload.AdID, day), float64(payload.BidCents)/1000.0); err != nil {
		return wrapRedisErr("increment spend", err)
	}
	return nil
}

func (s *service) TrackClick(ctx context.Context, input TrackClickInput) (*TrackClickResult, error) {
	if err := input.normalize(); err != nil {
		return nil, err
	}
	payload, err := token.ParseToken(s.tokenSecret, input.Token)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid click token")
	}
	if payload.EventType != enums.AdEventFactTypeClick {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "click token required")
	}
	if payload.RequestID != input.RequestID {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "request_id mismatch")
	}
	if payload.BuyerStoreID != input.BuyerStoreID {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store mismatch")
	}

	day := formatDay(input.Now)
	if day == "" {
		day = formatDay(time.Now().UTC())
	}

	result := &TrackClickResult{DestinationURL: payload.DestinationURL}

	key := clickDedupeKey(input.RequestID, payload.AdID)
	set, err := s.redis.SetNX(ctx, key, "1", dedupeTTL)
	if err != nil {
		return nil, wrapRedisErr("dedupe click", err)
	}
	if !set {
		return result, nil
	}

	if _, err := s.redis.Incr(ctx, counterKey("clicks", payload.AdID, day)); err != nil {
		return nil, wrapRedisErr("increment clicks", err)
	}

	return result, nil
}

func (input *TrackImpressionInput) normalize() error {
	input.Token = strings.TrimSpace(input.Token)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.Token == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "view_token is required")
	}
	if input.RequestID == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "request_id is required")
	}
	if input.BuyerStoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "buyer store id required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return nil
}

func (input *TrackClickInput) normalize() error {
	input.Token = strings.TrimSpace(input.Token)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.Token == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "click token is required")
	}
	if input.RequestID == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "request_id is required")
	}
	if input.BuyerStoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "buyer store id required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return nil
}

func (s *service) readDailySpend(ctx context.Context, adID uuid.UUID, day string) (float64, error) {
	key := counterKey("spend", adID, day)
	val, err := s.redis.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, wrapRedisErr("read spend", err)
	}
	if val == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, wrapRedisErr("parse spend", err)
	}
	return parsed, nil
}

func (s *service) buildServeResult(ctx context.Context, ad *models.Ad, creative *models.AdCreative, requestID string, placement enums.AdPlacement, buyerStoreID uuid.UUID, now time.Time) (*ServeAdResult, error) {
	basePayload := token.Payload{
		AdID:           ad.ID,
		CreativeID:     creative.ID,
		Placement:      placement,
		TargetType:     ad.TargetType,
		TargetID:       ad.TargetID,
		BuyerStoreID:   buyerStoreID,
		OccurredAt:     now.UTC(),
		ExpiresAt:      now.UTC().Add(s.tokenTTL),
		RequestID:      requestID,
		BidCents:       ad.BidCents,
		DestinationURL: creative.DestinationURL,
	}

	viewTokenPayload := basePayload
	viewTokenPayload.TokenID = uuid.New()
	viewTokenPayload.EventType = enums.AdEventFactTypeImpression
	viewToken, err := token.MintToken(s.tokenSecret, viewTokenPayload)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "mint view token")
	}

	clickTokenPayload := basePayload
	clickTokenPayload.TokenID = uuid.New()
	clickTokenPayload.EventType = enums.AdEventFactTypeClick
	clickToken, err := token.MintToken(s.tokenSecret, clickTokenPayload)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "mint click token")
	}

	return &ServeAdResult{
		AdID:       ad.ID,
		Placement:  placement,
		TargetType: ad.TargetType,
		TargetID:   ad.TargetID,
		Creative:   mapAdCreativeToDTO(*creative),
		RequestID:  requestID,
		ViewToken:  viewToken,
		ClickToken: clickToken,
	}, nil
}

func tieBreakScore(requestID string, placement enums.AdPlacement, day string, adID uuid.UUID) uint64 {
	input := fmt.Sprintf("%s|%s|%s|%s", requestID, placement, day, adID.String())
	sum := sha256.Sum256([]byte(input))
	return binary.BigEndian.Uint64(sum[:8])
}

func counterKey(kind string, adID uuid.UUID, day string) string {
	return fmt.Sprintf("pf:counter:%s:%s:%s", kind, adID.String(), day)
}

func impDedupeKey(requestID string, adID uuid.UUID, placement string) string {
	return fmt.Sprintf("pf:impdedupe:%s:%s:%s", requestID, adID.String(), placement)
}

func clickDedupeKey(requestID string, adID uuid.UUID) string {
	return fmt.Sprintf("pf:clickdedupe:%s:%s", requestID, adID.String())
}

func formatDay(ts time.Time) string {
	return ts.UTC().Format("20060102")
}

func wrapRedisErr(step string, err error) error {
	cause := fmt.Errorf("%w: %v", ErrRedisUnavailable, err)
	return pkgerrors.Wrap(pkgerrors.CodeDependency, cause, step)
}
