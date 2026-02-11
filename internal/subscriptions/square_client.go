package subscriptions

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/square"
	sq "github.com/square/square-go-sdk"
)

// SquareSubscriptionClient defines the subset of Square interactions that our subscription service relies on.
type SquareSubscriptionClient interface {
	Create(ctx context.Context, params *SquareSubscriptionParams) (*SquareSubscription, error)
	Cancel(ctx context.Context, id string, params *SquareSubscriptionCancelParams) (*SquareSubscription, error)
	Get(ctx context.Context, id string, params *SquareSubscriptionParams) (*SquareSubscription, error)
	Pause(ctx context.Context, id string, params *SquareSubscriptionPauseParams) (*SquareSubscription, error)
	Resume(ctx context.Context, id string, params *SquareSubscriptionResumeParams) (*SquareSubscription, error)
}

// NewSquareClient wraps the shared pkg/square client with the required location context.
func NewSquareClient(squareClient *square.Client, locationID string) SquareSubscriptionClient {
	return &squareSubscriptionClient{
		square:     squareClient,
		locationID: strings.TrimSpace(locationID),
	}
}

type squareSubscriptionClient struct {
	square     *square.Client
	locationID string
}

func (c *squareSubscriptionClient) Create(ctx context.Context, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	fmt.Printf("[squareSubscriptionClient.Create] start locationID='%s'\n", c.locationID)

	if c.square == nil {
		fmt.Printf("[squareSubscriptionClient.Create] FAIL square client nil\n")
		return nil, fmt.Errorf("square client required")
	}
	if c.locationID == "" {
		fmt.Printf("[squareSubscriptionClient.Create] FAIL locationID empty\n")
		return nil, fmt.Errorf("square location id required")
	}
	if params == nil {
		fmt.Printf("[squareSubscriptionClient.Create] FAIL params nil\n")
		return nil, fmt.Errorf("square subscription params required")
	}

	fmt.Printf("[squareSubscriptionClient.Create] customerID='%s' planVariationID='%s' cardID='%s'\n",
		params.CustomerID, strings.TrimSpace(params.PriceID), params.PaymentMethodID)

	req := square.SubscriptionCreateParams{
		LocationID:      c.locationID,
		PlanVariationID: strings.TrimSpace(params.PriceID),
		CustomerID:      params.CustomerID,
		CardID:          params.PaymentMethodID,
	}

	fmt.Printf("[squareSubscriptionClient.Create] Square API POST /v2/subscriptions request=%+v\n", req)
	resp, err := c.square.CreateSubscription(ctx, req)
	if err != nil {
		fmt.Printf("[squareSubscriptionClient.Create] FAIL CreateSubscription err=%T %v\n", err, err)
		debugSquareErr(err)
		return nil, err
	}

	fmt.Printf("[squareSubscriptionClient.Create] Square API POST /v2/subscriptions response=%+v\n", resp)
	fmt.Printf("[squareSubscriptionClient.Create] OK subscriptionID='%s'\n", safeString(resp.GetID()))
	return convertSubscription(resp, params.PriceID, params.Metadata), nil
}

func (c *squareSubscriptionClient) Cancel(ctx context.Context, id string, params *SquareSubscriptionCancelParams) (*SquareSubscription, error) {
	if c.square == nil {
		return nil, fmt.Errorf("square client required")
	}
	fmt.Printf("[squareSubscriptionClient.Cancel] Square API POST /v2/subscriptions/%s/cancel\n", id)
	resp, err := c.square.CancelSubscription(ctx, id)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[squareSubscriptionClient.Cancel] Square API POST /v2/subscriptions/%s/cancel response=%+v\n", id, resp)
	return convertSubscription(resp, "", nil), nil
}

func (c *squareSubscriptionClient) Get(ctx context.Context, id string, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	if c.square == nil {
		return nil, fmt.Errorf("square client required")
	}
	fmt.Printf("[squareSubscriptionClient.Get] Square API GET /v2/subscriptions/%s\n", id)
	resp, err := c.square.GetSubscription(ctx, id)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[squareSubscriptionClient.Get] Square API GET /v2/subscriptions/%s response=%+v\n", id, resp)
	var metadata map[string]string
	fallbackPrice := ""
	if params != nil {
		metadata = params.Metadata
		fallbackPrice = params.PriceID
	}
	return convertSubscription(resp, fallbackPrice, metadata), nil
}

func (c *squareSubscriptionClient) Pause(ctx context.Context, id string, params *SquareSubscriptionPauseParams) (*SquareSubscription, error) {
	if c.square == nil {
		return nil, fmt.Errorf("square client required")
	}
	req := &sq.PauseSubscriptionRequest{
		SubscriptionID: id,
	}
	fmt.Printf("[squareSubscriptionClient.Pause] Square API POST /v2/subscriptions/%s/pause request=%+v\n", id, req)
	resp, err := c.square.Pause(ctx, req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[squareSubscriptionClient.Pause] Square API POST /v2/subscriptions/%s/pause response=%+v\n", id, resp)
	price := ""
	if params != nil {
		price = params.PriceID
	}
	return convertSubscription(resp, price, nil), nil
}

func (c *squareSubscriptionClient) Resume(ctx context.Context, id string, params *SquareSubscriptionResumeParams) (*SquareSubscription, error) {
	if c.square == nil {
		return nil, fmt.Errorf("square client required")
	}
	req := &sq.ResumeSubscriptionRequest{
		SubscriptionID: id,
	}
	fmt.Printf("[squareSubscriptionClient.Resume] Square API POST /v2/subscriptions/%s/resume request=%+v\n", id, req)
	resp, err := c.square.Resume(ctx, req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[squareSubscriptionClient.Resume] Square API POST /v2/subscriptions/%s/resume response=%+v\n", id, resp)
	price := ""
	if params != nil {
		price = params.PriceID
	}
	return convertSubscription(resp, price, nil), nil
}

func convertSubscription(resp *sq.Subscription, fallbackPrice string, providedMetadata map[string]string) *SquareSubscription {
	if resp == nil {
		return nil
	}
	start := parseDate(resp.GetStartDate())
	end := parseDate(resp.GetChargedThroughDate())
	metadata := cloneMetadata(providedMetadata)
	priceID := strings.TrimSpace(safeString(resp.GetPlanVariationID()))
	if priceID == "" {
		priceID = strings.TrimSpace(fallbackPrice)
	}
	if priceID == "" && metadata != nil {
		priceID = strings.TrimSpace(metadata["price_id"])
	}
	if priceID == "" && metadata != nil {
		priceID = strings.TrimSpace(metadata["plan_variation_id"])
	}
	status := resp.GetStatus()
	cancelAtPeriodEnd := status != nil && *status == sq.SubscriptionStatusCanceled
	return &SquareSubscription{
		ID:                 safeString(resp.GetID()),
		Status:             subscriptionStatusString(status),
		Metadata:           metadata,
		CancelAtPeriodEnd:  cancelAtPeriodEnd,
		StartDate:          start,
		ChargedThroughDate: end,
		CanceledAt:         parseDate(resp.GetCanceledDate()),
		Items: &SquareSubscriptionItemList{
			Data: []*SquareSubscriptionItem{
				{
					CurrentPeriodStart: start,
					CurrentPeriodEnd:   end,
					Price: &SquareSubscriptionPrice{
						ID: priceID,
					},
				},
			},
		},
	}
}

func cloneMetadata(rows map[string]string) map[string]string {
	if len(rows) == 0 {
		return nil
	}
	clone := make(map[string]string, len(rows))
	for k, v := range rows {
		clone[k] = v
	}
	return clone
}

func parseDate(value *string) int64 {
	if value == nil || strings.TrimSpace(*value) == "" {
		return 0
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range formats {
		if ts, err := time.Parse(layout, *value); err == nil {
			return ts.Unix()
		}
	}
	if i, err := strconv.ParseInt(*value, 10, 64); err == nil {
		return i
	}
	return 0
}

func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func subscriptionStatusString(status *sq.SubscriptionStatus) string {
	if status == nil {
		return ""
	}
	return string(*status)
}
