package stores

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// StoreDTO exposes safe tenant data in API responses.
type StoreDTO struct {
	ID                   uuid.UUID         `json:"id"`
	Type                 enums.StoreType   `json:"type"`
	CompanyName          string            `json:"company_name"`
	DBAName              *string           `json:"dba_name,omitempty"`
	Description          *string           `json:"description,omitempty"`
	Phone                *string           `json:"phone,omitempty"`
	Email                *string           `json:"email,omitempty"`
	KYCStatus            enums.KYCStatus   `json:"kyc_status"`
	SubscriptionActive   bool              `json:"subscription_active"`
	DeliveryRadiusMeters int               `json:"delivery_radius_meters"`
	Address              types.Address     `json:"address"`
	Social               *types.Social     `json:"social,omitempty"`
	BannerURL            *string           `json:"banner_url,omitempty"`
	LogoURL              *string           `json:"logo_url,omitempty"`
	BannerMediaID        *uuid.UUID        `json:"banner_media_id,omitempty"`
	LogoMediaID          *uuid.UUID        `json:"logo_media_id,omitempty"`
	Ratings              map[string]int    `json:"ratings,omitempty"`
	Categories           []string          `json:"categories,omitempty"`
	OwnerID              uuid.UUID         `json:"owner"`
	Badge                *enums.StoreBadge `json:"badge,omitempty"`
	LastActiveAt         *time.Time        `json:"last_active_at,omitempty"`
	Owner                OwnerSummaryDTO   `json:"owner_detail"`
	Licenses             []StoreLicenseDTO `json:"licenses,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

type OwnerSummaryDTO struct {
	ID           uuid.UUID  `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
	Role         *string    `json:"role,omitempty"`
}

type StoreLicenseDTO struct {
	Number string            `json:"number"`
	Type   enums.LicenseType `json:"type"`
}

// CreateStoreDTO holds creation-time data for a new store.
type CreateStoreDTO struct {
	Type                 enums.StoreType
	CompanyName          string
	DBAName              *string
	Description          *string
	Phone                *string
	Email                *string
	KYCStatus            *enums.KYCStatus
	SubscriptionActive   *bool
	DeliveryRadiusMeters *int
	Address              types.Address
	Social               *types.Social
	Badge                *enums.StoreBadge
	OwnerID              uuid.UUID
}

// FromModel maps the persisted store into a DTO.
func FromModel(m *models.Store, u *OwnerSummaryDTO) *StoreDTO {
	if m == nil {
		return nil
	}

	dto := &StoreDTO{
		ID:                   m.ID,
		Type:                 m.Type,
		CompanyName:          m.CompanyName,
		DBAName:              m.DBAName,
		Description:          m.Description,
		Phone:                m.Phone,
		Email:                m.Email,
		KYCStatus:            m.KYCStatus,
		SubscriptionActive:   m.SubscriptionActive,
		DeliveryRadiusMeters: m.DeliveryRadiusMeters,
		Address:              m.Address,
		Social:               m.Social,
		OwnerID:              m.OwnerID,
		LastActiveAt:         m.LastActiveAt,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}

	if u != nil && u.LastActiveAt != nil {
		la := *u.LastActiveAt
		dto.LastActiveAt = &la
	}

	if m.Badge != nil {
		badge := *m.Badge
		dto.Badge = &badge
	}

	if m.Social != nil {
		cpy := *m.Social
		dto.Social = &cpy
	}

	if m.BannerURL != nil {
		banner := *m.BannerURL
		dto.BannerURL = &banner
	}
	if m.LogoURL != nil {
		logo := *m.LogoURL
		dto.LogoURL = &logo
	}
	if m.BannerMediaID != nil {
		dto.BannerMediaID = cloneUUIDPtr(m.BannerMediaID)
	}
	if m.LogoMediaID != nil {
		dto.LogoMediaID = cloneUUIDPtr(m.LogoMediaID)
	}
	if len(m.Ratings) > 0 {
		dto.Ratings = make(map[string]int, len(m.Ratings))
		for k, v := range m.Ratings {
			dto.Ratings[k] = v
		}
	}
	if len(m.Categories) > 0 {
		dto.Categories = append(dto.Categories, m.Categories...)
	}

	return dto
}

// ToModel prepares the GORM model from creation DTO, supplying defaults.
func (c CreateStoreDTO) ToModel() *models.Store {
	model := &models.Store{
		Type:                 c.Type,
		CompanyName:          c.CompanyName,
		DBAName:              c.DBAName,
		Description:          c.Description,
		Phone:                c.Phone,
		Email:                c.Email,
		KYCStatus:            enums.KYCStatusPendingVerification,
		SubscriptionActive:   false,
		DeliveryRadiusMeters: 0,
		Address:              c.Address,
		Social:               nil,
		OwnerID:              c.OwnerID,
	}

	if c.KYCStatus != nil {
		model.KYCStatus = *c.KYCStatus
	}
	if c.SubscriptionActive != nil {
		model.SubscriptionActive = *c.SubscriptionActive
	}
	if c.DeliveryRadiusMeters != nil {
		model.DeliveryRadiusMeters = *c.DeliveryRadiusMeters
	}
	if c.Social != nil {
		cpy := *c.Social
		model.Social = &cpy
	}
	if c.Badge != nil {
		model.Badge = cloneStoreBadgePtr(c.Badge)
	}

	return model
}

func cloneStoreBadgePtr(value *enums.StoreBadge) *enums.StoreBadge {
	if value == nil {
		return nil
	}
	cpy := *value
	return &cpy
}

func cloneUUIDPtr(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	cpy := *id
	return &cpy
}
