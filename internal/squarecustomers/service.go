package squarecustomers

import (
	"context"
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/square"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	sq "github.com/square/square-go-sdk"
)

// Service ensures Square customer records exist and exposes the customer identifier.
type Service interface {
	EnsureCustomer(ctx context.Context, input Input) (string, error)
}

// Input contains the fields required to create or locate a Square customer.
type Input struct {
	ReferenceID string
	FirstName   string
	LastName    string
	Email       string
	Phone       *string
	CompanyName string
	Address     types.Address
}

type service struct {
	client *square.Client
}

// NewService builds a service that uses the shared Square client.
func NewService(client *square.Client) Service {
	return &service{client: client}
}

func (s *service) EnsureCustomer(ctx context.Context, input Input) (string, error) {
	if s == nil || s.client == nil {
		return "", errors.New(errors.CodeInternal, "square client required")
	}

	params := square.CustomerCreateParams{
		Email:       strings.TrimSpace(input.Email),
		PhoneNumber: strings.TrimSpace(safeString(input.Phone)),
		GivenName:   strings.TrimSpace(input.FirstName),
		FamilyName:  strings.TrimSpace(input.LastName),
		CompanyName: strings.TrimSpace(input.CompanyName),
		ReferenceID: DefaultReferenceID(input.ReferenceID, input.Email, input.CompanyName),
		Address:     toSquareAddress(input.Address),
	}

	customer, err := s.client.EnsureCustomer(ctx, params)
	if err != nil {
		return "", errors.Wrap(errors.CodeDependency, err, "ensure square customer")
	}
	if customer == nil {
		return "", errors.New(errors.CodeDependency, "square customer missing")
	}
	if id := customer.GetID(); id != nil && strings.TrimSpace(*id) != "" {
		return *id, nil
	}
	return "", errors.New(errors.CodeDependency, "square customer id missing")
}

// DefaultReferenceID returns a deterministic reference value for the provided fields.
func DefaultReferenceID(reference, email, company string) string {
	if trimmed := strings.TrimSpace(reference); trimmed != "" {
		return trimmed
	}
	parts := []string{
		normalizeReferencePart(strings.ToLower(email)),
		normalizeReferencePart(strings.ToLower(company)),
	}
	return fmt.Sprintf("pf:register:%s:%s", parts[0], parts[1])
}

func normalizeReferencePart(raw string) string {
	trimmed := strings.TrimSpace(raw)
	var builder strings.Builder
	for _, r := range trimmed {
		if r == ' ' || r == '_' || r == '-' || r == '.' {
			builder.WriteRune('-')
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
	}
	result := builder.String()
	if result == "" {
		return "pf"
	}
	return result
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func toSquareAddress(addr types.Address) *sq.Address {
	if strings.TrimSpace(addr.Line1) == "" && strings.TrimSpace(addr.City) == "" &&
		strings.TrimSpace(addr.State) == "" && strings.TrimSpace(addr.PostalCode) == "" &&
		strings.TrimSpace(addr.Country) == "" {
		return nil
	}
	return &sq.Address{
		AddressLine1:                 stringPtr(addr.Line1),
		AddressLine2:                 addr.Line2,
		Locality:                     stringPtr(addr.City),
		AdministrativeDistrictLevel1: stringPtr(addr.State),
		PostalCode:                   stringPtr(addr.PostalCode),
		Country:                      countryPtr(addr.Country),
	}
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func countryPtr(value string) *sq.Country {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "US"
	}
	c := sq.Country(trimmed)
	return &c
}
