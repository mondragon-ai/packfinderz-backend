package square

import (
	"context"
	"strings"

	sq "github.com/square/square-go-sdk"
)

// CustomerSearchParams scopes the fields we can use to find an existing Square customer.
type CustomerSearchParams struct {
	ReferenceID string
	Email       string
}

// SearchCustomer returns the first customer matching the provided filters or nil when none exist.
func (c *Client) SearchCustomer(ctx context.Context, params CustomerSearchParams) (*sq.Customer, error) {
	if c == nil {
		return nil, errAccessTokenRequired
	}
	query := &sq.CustomerQuery{}
	filter := &sq.CustomerFilter{}
	hasFilter := false
	if trimmed := strings.TrimSpace(params.ReferenceID); trimmed != "" {
		filter.ReferenceID = &sq.CustomerTextFilter{Exact: ptrString(trimmed)}
		hasFilter = true
	}
	if trimmed := strings.TrimSpace(params.Email); trimmed != "" {
		filter.EmailAddress = &sq.CustomerTextFilter{Exact: ptrString(trimmed)}
		hasFilter = true
	}
	if !hasFilter {
		return nil, nil
	}
	query.Filter = filter

	req := &sq.SearchCustomersRequest{
		Query: query,
		Limit: int64Ptr(1),
	}
	c.log(ctx, "request", "search_customer", map[string]any{
		"reference_id": params.ReferenceID,
		"email":        params.Email,
	})

	resp, err := c.sdk.Customers.Search(ctx, req)
	if err != nil {
		c.log(ctx, "error", "search_customer", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "search customer")
	}

	customers := resp.GetCustomers()
	if len(customers) == 0 {
		c.log(ctx, "response", "search_customer", map[string]any{"found": false})
		return nil, nil
	}
	customer := customers[0]
	c.log(ctx, "response", "search_customer", map[string]any{
		"customer_id": stringValue(customer.GetID()),
	})
	return customer, nil
}

// EnsureCustomer creates the customer when no matching record exists; otherwise returns the existing customer.
func (c *Client) EnsureCustomer(ctx context.Context, params CustomerCreateParams) (*sq.Customer, error) {
	if c == nil {
		return nil, errAccessTokenRequired
	}
	if customer, err := c.SearchCustomer(ctx, CustomerSearchParams{
		ReferenceID: params.ReferenceID,
		Email:       params.Email,
	}); err != nil {
		return nil, err
	} else if customer != nil {
		return customer, nil
	}
	return c.CreateCustomer(ctx, params)
}
