package subscriptions

// SquareSubscription represents the subset of metadata we need to mirror into the billing tables.
type SquareSubscription struct {
	ID                 string
	Status             string
	Metadata           map[string]string
	CancelAtPeriodEnd  bool
	CanceledAt         int64
	ChargedThroughDate int64
	StartDate          int64
	Actions            []*SquareSubscriptionAction
	Items              *SquareSubscriptionItemList
}

type SquareSubscriptionItemList struct {
	Data []*SquareSubscriptionItem
}

type SquareSubscriptionItem struct {
	CurrentPeriodStart int64
	CurrentPeriodEnd   int64
	Price              *SquareSubscriptionPrice
}

type SquareSubscriptionPrice struct {
	ID string
}

type SquareSubscriptionParams struct {
	CustomerID      string
	PriceID         string
	PaymentMethodID string
	Metadata        map[string]string
	IncludeActions  bool
}

type SquareSubscriptionCancelParams struct{}

type SquareSubscriptionPauseParams struct {
	PriceID            string
	PauseEffectiveDate string
}

type SquareSubscriptionResumeParams struct {
	PriceID string
}

type SquareSubscriptionAction struct {
	ID            string
	Type          string
	EffectiveDate int64
}
