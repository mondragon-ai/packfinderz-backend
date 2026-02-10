package address

import (
	"context"
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/maps"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

type Service interface {
	Suggest(ctx context.Context, req SuggestRequest) ([]Suggestion, error)
	Resolve(ctx context.Context, req ResolveRequest) (types.Address, error)
}

type service struct {
	maps *maps.Client
}

func NewService(client *maps.Client) Service {
	return &service{maps: client}
}

func (s *service) Suggest(ctx context.Context, req SuggestRequest) ([]Suggestion, error) {
	if s == nil || s.maps == nil {
		return nil, errors.New(errors.CodeDependency, "maps client unavailable")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New(errors.CodeValidation, "query is required")
	}

	payload := maps.AutocompleteRequest{
		Input: req.Query,
	}
	if country := strings.TrimSpace(req.Country); country != "" {
		payload.IncludedRegionCodes = []string{strings.ToUpper(country)}
	}
	if lang := strings.TrimSpace(req.Language); lang != "" {
		payload.LanguageCode = lang
	}

	resp, err := s.maps.Autocomplete(ctx, payload)
	if err != nil {
		return nil, err
	}

	suggestions := make([]Suggestion, 0, len(resp))
	for _, item := range resp {
		suggestions = append(suggestions, Suggestion{
			PlaceID:     item.PlaceID,
			Description: item.Description,
		})
	}
	return suggestions, nil
}

func (s *service) Resolve(ctx context.Context, req ResolveRequest) (types.Address, error) {
	if s == nil || s.maps == nil {
		return types.Address{}, errors.New(errors.CodeDependency, "maps client unavailable")
	}
	if strings.TrimSpace(req.PlaceID) == "" {
		return types.Address{}, errors.New(errors.CodeValidation, "place_id is required")
	}

	details, err := s.maps.ResolvePlace(ctx, req.PlaceID)
	if err != nil {
		return types.Address{}, err
	}

	return mapPlaceDetails(details)
}

func mapPlaceDetails(details *maps.PlaceDetails) (types.Address, error) {
	if details == nil {
		return types.Address{}, errors.New(errors.CodeDependency, "place details missing")
	}
	if details.Location.Latitude == 0 && details.Location.Longitude == 0 {
		return types.Address{}, errors.New(errors.CodeDependency, "place location missing")
	}

	find := func(kind string) (string, bool) {
		for _, comp := range details.AddressComponents {
			for _, typ := range comp.Types {
				if typ == kind && comp.LongName != "" {
					return comp.LongName, true
				}
			}
		}
		return "", false
	}

	line1 := ""
	if number, ok := find("street_number"); ok {
		line1 = number
	}
	if route, ok := find("route"); ok {
		if line1 != "" {
			line1 = fmt.Sprintf("%s %s", line1, route)
		} else {
			line1 = route
		}
	}
	if line1 == "" && strings.TrimSpace(details.FormattedAddress) != "" {
		parts := strings.Split(details.FormattedAddress, ",")
		line1 = strings.TrimSpace(parts[0])
	}
	if line1 == "" {
		return types.Address{}, errors.New(errors.CodeDependency, "address line1 missing")
	}

	var line2 *string
	if sub, ok := find("subpremise"); ok {
		line2 = ptr(sub)
	}

	city, ok := find("locality")
	if !ok {
		if town, ok2 := find("postal_town"); ok2 {
			city = town
		} else if admin2, ok3 := find("administrative_area_level_2"); ok3 {
			city = admin2
		}
	}
	if city == "" {
		return types.Address{}, errors.New(errors.CodeDependency, "city missing")
	}

	state, ok := find("administrative_area_level_1")
	if !ok {
		return types.Address{}, errors.New(errors.CodeDependency, "state missing")
	}

	postalCode, ok := find("postal_code")
	if !ok {
		return types.Address{}, errors.New(errors.CodeDependency, "postal code missing")
	}

	country, ok := find("country")
	if !ok {
		country = "US"
	}

	return types.Address{
		Line1:      line1,
		Line2:      line2,
		City:       city,
		State:      state,
		PostalCode: postalCode,
		Country:    country,
		Lat:        details.Location.Latitude,
		Lng:        details.Location.Longitude,
	}, nil
}

func ptr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

type SuggestRequest struct {
	Query    string
	Country  string
	Language string
}

type ResolveRequest struct {
	PlaceID string
}

type Suggestion struct {
	PlaceID     string `json:"place_id"`
	Description string `json:"description"`
}
