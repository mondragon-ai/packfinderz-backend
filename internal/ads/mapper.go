package ads

import (
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

func mapAdCreativeToDTO(creative models.AdCreative) AdCreativeDTO {
	return AdCreativeDTO{
		ID:             creative.ID,
		MediaID:        creative.MediaID,
		DestinationURL: creative.DestinationURL,
		Headline:       creative.Headline,
		Body:           creative.Body,
		CreatedAt:      creative.CreatedAt,
		UpdatedAt:      creative.UpdatedAt,
	}
}

// MapAdToDTO converts the persisted ad row into a transport DTO.
func MapAdToDTO(ad *models.Ad) AdDTO {
	dto := AdDTO{
		ID:               ad.ID,
		StoreID:          ad.StoreID,
		Status:           ad.Status,
		Placement:        enums.AdPlacement(ad.Placement),
		TargetType:       ad.TargetType,
		TargetID:         ad.TargetID,
		BidCents:         ad.BidCents,
		DailyBudgetCents: ad.DailyBudgetCents,
		StartsAt:         ad.StartsAt,
		EndsAt:           ad.EndsAt,
		CreatedAt:        ad.CreatedAt,
		UpdatedAt:        ad.UpdatedAt,
	}
	if len(ad.Creatives) > 0 {
		dto.Creatives = make([]AdCreativeDTO, 0, len(ad.Creatives))
		for _, creative := range ad.Creatives {
			dto.Creatives = append(dto.Creatives, mapAdCreativeToDTO(creative))
		}
	}
	return dto
}

func NewAdModelFromCreateInput(input CreateAdInput) models.Ad {
	model := models.Ad{
		ID:               uuid.New(),
		StoreID:          input.StoreID,
		Status:           input.Status,
		Placement:        string(input.Placement),
		TargetType:       input.TargetType,
		TargetID:         input.TargetID,
		BidCents:         input.BidCents,
		DailyBudgetCents: input.DailyBudgetCents,
		StartsAt:         input.StartsAt,
		EndsAt:           input.EndsAt,
	}
	if len(input.Creatives) > 0 {
		model.Creatives = make([]models.AdCreative, 0, len(input.Creatives))
		for _, creative := range input.Creatives {
			model.Creatives = append(model.Creatives, models.AdCreative{
				ID:             uuid.New(),
				MediaID:        creative.MediaID,
				DestinationURL: creative.DestinationURL,
				Headline:       creative.Headline,
				Body:           creative.Body,
			})
		}
	}
	return model
}
