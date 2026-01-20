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
	ID                   uuid.UUID            `json:"id"`
	Type                 enums.StoreType      `json:"type"`
	CompanyName          string               `json:"company_name"`
	DBAName              *string              `json:"dba_name,omitempty"`
	Description          *string              `json:"description,omitempty"`
	Phone                *string              `json:"phone,omitempty"`
	Email                *string              `json:"email,omitempty"`
	KYCStatus            enums.KYCStatus      `json:"kyc_status"`
	SubscriptionActive   bool                 `json:"subscription_active"`
	DeliveryRadiusMeters int                  `json:"delivery_radius_meters"`
	Address              types.Address        `json:"address"`
	Geom                 types.GeographyPoint `json:"geom"`
	Social               *types.Social        `json:"social,omitempty"`
	OwnerID              uuid.UUID            `json:"owner"`
	LastActiveAt         *time.Time           `json:"last_active_at,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
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
	Geom                 types.GeographyPoint
	Social               *types.Social
	OwnerID              uuid.UUID
}

// FromModel maps the persisted store into a DTO.
func FromModel(m *models.Store) *StoreDTO {
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
		Geom:                 m.Geom,
		Social:               m.Social,
		OwnerID:              m.OwnerID,
		LastActiveAt:         m.LastActiveAt,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}

	if m.Social != nil {
		cpy := *m.Social
		dto.Social = &cpy
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
		Geom:                 c.Geom,
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

	return model
}
