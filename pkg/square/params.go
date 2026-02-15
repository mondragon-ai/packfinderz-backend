package square

import (
	"strings"

	sq "github.com/square/square-go-sdk"
)

// SubscriptionCreateParams contains the fields required to start a Square subscription.
type SubscriptionCreateParams struct {
	LocationID            string
	PlanVariationID       string
	CustomerID            string
	CardID                string
	IdempotencyKey        string
	StartDate             string
	CanceledDate          string
	TaxPercentage         string
	PriceOverrideAmount   int64
	PriceOverrideCurrency string
}

func (p SubscriptionCreateParams) toSquareRequest(idempotencyKey string) *sq.CreateSubscriptionRequest {
	req := &sq.CreateSubscriptionRequest{
		IdempotencyKey: ptrString(idempotencyKey),
		LocationID:     p.LocationID,
		CustomerID:     p.CustomerID,
	}
	if trimmed := strings.TrimSpace(p.PlanVariationID); trimmed != "" {
		req.PlanVariationID = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.CardID); trimmed != "" {
		req.CardID = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.StartDate); trimmed != "" {
		req.StartDate = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.CanceledDate); trimmed != "" {
		req.CanceledDate = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.TaxPercentage); trimmed != "" {
		req.TaxPercentage = ptrString(trimmed)
	}
	if p.PriceOverrideAmount > 0 {
		req.PriceOverrideMoney = moneyPtr(p.PriceOverrideAmount, p.PriceOverrideCurrency)
	}
	return req
}

// CustomerCreateParams defines the payload to create a Square customer.
type CustomerCreateParams struct {
	Email          string
	PhoneNumber    string
	GivenName      string
	FamilyName     string
	CompanyName    string
	ReferenceID    string
	Address        *sq.Address
	Note           string
	IdempotencyKey string
}

func (p CustomerCreateParams) toSquareRequest(idempotencyKey string) *sq.CreateCustomerRequest {
	req := &sq.CreateCustomerRequest{
		IdempotencyKey: ptrString(idempotencyKey),
	}
	if trimmed := strings.TrimSpace(p.Email); trimmed != "" {
		req.EmailAddress = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.PhoneNumber); trimmed != "" {
		req.PhoneNumber = ptrString("+1" + trimmed)
	}
	if trimmed := strings.TrimSpace(p.GivenName); trimmed != "" {
		req.GivenName = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.FamilyName); trimmed != "" {
		req.FamilyName = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.CompanyName); trimmed != "" {
		req.CompanyName = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.ReferenceID); trimmed != "" {
		req.ReferenceID = ptrString(trimmed)
	}
	if p.Address != nil {
		req.Address = p.Address
	}
	if trimmed := strings.TrimSpace(p.Note); trimmed != "" {
		req.Note = ptrString(trimmed)
	}
	return req
}

// CardCreateParams groups the data needed to vault a card.
type CardCreateParams struct {
	CustomerID        string
	SourceID          string
	CardholderName    string
	BillingAddress    *sq.Address
	ReferenceID       string
	VerificationToken string
	IdempotencyKey    string
}

func (p CardCreateParams) toSquareRequest(idempotencyKey string) *sq.CreateCardRequest {
	req := &sq.CreateCardRequest{
		IdempotencyKey: idempotencyKey,
		SourceID:       p.SourceID,
	}
	if trimmed := strings.TrimSpace(p.VerificationToken); trimmed != "" {
		req.VerificationToken = ptrString(trimmed)
	}
	card := &sq.Card{}
	if trimmed := strings.TrimSpace(p.CustomerID); trimmed != "" {
		card.CustomerID = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.CardholderName); trimmed != "" {
		card.CardholderName = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.ReferenceID); trimmed != "" {
		card.ReferenceID = ptrString(trimmed)
	}
	if p.BillingAddress != nil {
		card.BillingAddress = p.BillingAddress
	}
	if card.CustomerID != nil || card.CardholderName != nil || card.BillingAddress != nil || card.ReferenceID != nil {
		req.Card = card
	}
	return req
}

// PaymentCreateParams encapsulates the inputs for a Square payment.
type PaymentCreateParams struct {
	AmountCents    int64
	Currency       string
	LocationID     string
	CustomerID     string
	SourceID       string
	IdempotencyKey string
	Note           string
	ReferenceID    string
}

func (p PaymentCreateParams) toSquareRequest(idempotencyKey string) *sq.CreatePaymentRequest {
	req := &sq.CreatePaymentRequest{
		IdempotencyKey: idempotencyKey,
		LocationID:     ptrString(p.LocationID),
		CustomerID:     ptrString(p.CustomerID),
		SourceID:       p.SourceID,
	}
	if p.AmountCents > 0 {
		req.AmountMoney = moneyPtr(p.AmountCents, p.Currency)
	}
	if trimmed := strings.TrimSpace(p.Note); trimmed != "" {
		req.Note = ptrString(trimmed)
	}
	if trimmed := strings.TrimSpace(p.ReferenceID); trimmed != "" {
		req.ReferenceID = ptrString(trimmed)
	}
	return req
}

func ptrString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func currencyPtr(code string) *sq.Currency {
	trimmed := strings.ToUpper(strings.TrimSpace(code))
	if trimmed == "" {
		trimmed = "USD"
	}
	c := sq.Currency(trimmed)
	return &c
}

func moneyPtr(amount int64, currency string) *sq.Money {
	if amount == 0 {
		return nil
	}
	return &sq.Money{
		Amount:   int64Ptr(amount),
		Currency: currencyPtr(currency),
	}
}
