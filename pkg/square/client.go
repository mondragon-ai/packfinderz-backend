package square

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	sq "github.com/square/square-go-sdk"
	sqclient "github.com/square/square-go-sdk/client"
	sqcore "github.com/square/square-go-sdk/core"
	sqoption "github.com/square/square-go-sdk/option"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

const (
	sandboxEnv    = "sandbox"
	productionEnv = "production"
)

var (
	errAccessTokenRequired   = errors.New("square access token is required")
	errWebhookSecretRequired = errors.New("square webhook secret is required")
	errInvalidSquareEnv      = fmt.Errorf("square environment must be %q or %q", sandboxEnv, productionEnv)
	errLoggerRequired        = errors.New("square logger is required")
)

var baseURLs = map[string]string{
	sandboxEnv:    "https://connect.squareupsandbox.com",
	productionEnv: "https://connect.squareup.com",
}

// Client exposes Square primitives with centralized auth, logging, idempotency, and error mapping.
type Client struct {
	sdk           *sqclient.Client
	accessToken   string
	environment   string
	webhookSecret string
	baseURL       string
	logger        *logger.Logger
}

// NewClient initializes the Square wrapper and validates the credentials.
func NewClient(ctx context.Context, cfg config.SquareConfig, logg *logger.Logger) (*Client, error) {
	if logg == nil {
		return nil, errLoggerRequired
	}
	env, err := normalizeEnv(cfg.Environment())
	if err != nil {
		return nil, err
	}

	accessToken := strings.TrimSpace(cfg.AccessToken)
	if accessToken == "" {
		return nil, errAccessTokenRequired
	}

	webhookSecret := strings.TrimSpace(cfg.WebhookSecret)
	if webhookSecret == "" {
		return nil, errWebhookSecretRequired
	}

	baseURL := baseURLs[env]
	sdk := sqclient.NewClient(
		sqoption.WithBaseURL(baseURL),
		sqoption.WithToken(accessToken),
	)

	c := &Client{
		sdk:           sdk,
		accessToken:   accessToken,
		environment:   env,
		webhookSecret: webhookSecret,
		baseURL:       baseURL,
		logger:        logg,
	}

	logg.Info(ctx, "square client initialized")
	return c, nil
}

// AccessToken returns the configured Square token.
func (c *Client) AccessToken() string {
	if c == nil {
		return ""
	}
	return c.accessToken
}

// Environment reports the normalized Square environment.
func (c *Client) Environment() string {
	if c == nil {
		return ""
	}
	return c.environment
}

// SigningSecret returns the Square webhook secret.
func (c *Client) SigningSecret() string {
	if c == nil {
		return ""
	}
	return c.webhookSecret
}

// NewIdempotencyKey returns a unique key for Square operations.
func (c *Client) NewIdempotencyKey(prefix string) string {
	key := strings.TrimSpace(prefix)
	if key == "" {
		key = "pf"
	}
	return fmt.Sprintf("%s-%s", key, uuid.NewString())
}

// Subscription operations
func (c *Client) CreateSubscription(ctx context.Context, params SubscriptionCreateParams) (*sq.Subscription, error) {
	req := params.toSquareRequest(c.ensureIdempotencyKey("subscription.create", params.IdempotencyKey))
	c.log(ctx, "request", "create_subscription", map[string]any{
		"location_id":       params.LocationID,
		"plan_variation_id": params.PlanVariationID,
		"customer_id":       params.CustomerID,
		"card_id":           params.CardID,
	})

	resp, err := c.sdk.Subscriptions.Create(ctx, req)
	if err != nil {
		c.log(ctx, "error", "create_subscription", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "create subscription")
	}

	sub := resp.GetSubscription()
	c.log(ctx, "response", "create_subscription", map[string]any{
		"subscription_id": stringValue(sub.GetID()),
		"status":          subscriptionStatusString(sub.GetStatus()),
	})
	return sub, nil
}

func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string) (*sq.Subscription, error) {
	req := &sq.CancelSubscriptionsRequest{SubscriptionID: subscriptionID}
	c.log(ctx, "request", "cancel_subscription", map[string]any{"subscription_id": subscriptionID})

	resp, err := c.sdk.Subscriptions.Cancel(ctx, req)
	if err != nil {
		c.log(ctx, "error", "cancel_subscription", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "cancel subscription")
	}

	sub := resp.GetSubscription()
	c.log(ctx, "response", "cancel_subscription", map[string]any{
		"subscription_id": stringValue(sub.GetID()),
		"status":          subscriptionStatusString(sub.GetStatus()),
	})
	return sub, nil
}

func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*sq.Subscription, error) {
	req := &sq.GetSubscriptionsRequest{SubscriptionID: subscriptionID}
	c.log(ctx, "request", "get_subscription", map[string]any{"subscription_id": subscriptionID})

	resp, err := c.sdk.Subscriptions.Get(ctx, req)
	if err != nil {
		c.log(ctx, "error", "get_subscription", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "get subscription")
	}

	sub := resp.GetSubscription()
	c.log(ctx, "response", "get_subscription", map[string]any{
		"subscription_id": stringValue(sub.GetID()),
		"status":          subscriptionStatusString(sub.GetStatus()),
	})
	return sub, nil
}

// Customer operations
func (c *Client) CreateCustomer(ctx context.Context, params CustomerCreateParams) (*sq.Customer, error) {
	req := params.toSquareRequest(c.ensureIdempotencyKey("customer.create", params.IdempotencyKey))
	c.log(ctx, "request", "create_customer", map[string]any{"reference_id": params.ReferenceID})

	resp, err := c.sdk.Customers.Create(ctx, req)
	if err != nil {
		c.log(ctx, "error", "create_customer", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "create customer")
	}

	cust := resp.GetCustomer()
	c.log(ctx, "response", "create_customer", map[string]any{"customer_id": stringValue(cust.GetID())})
	return cust, nil
}

// Card operations
func (c *Client) CreateCard(ctx context.Context, params CardCreateParams) (*sq.Card, error) {
	req := params.toSquareRequest(c.ensureIdempotencyKey("card.create", params.IdempotencyKey))
	c.log(ctx, "request", "create_card", map[string]any{"customer_id": params.CustomerID})

	resp, err := c.sdk.Cards.Create(ctx, req)
	if err != nil {
		c.log(ctx, "error", "create_card", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "create card")
	}

	card := resp.GetCard()
	c.log(ctx, "response", "create_card", map[string]any{"card_id": stringValue(card.GetID())})
	return card, nil
}

// Payment operations
func (c *Client) CreatePayment(ctx context.Context, params PaymentCreateParams) (*sq.Payment, error) {
	req := params.toSquareRequest(c.ensureIdempotencyKey("payment.create", params.IdempotencyKey))
	c.log(ctx, "request", "create_payment", map[string]any{
		"location_id": params.LocationID,
		"customer_id": params.CustomerID,
		"amount":      params.AmountCents,
	})

	resp, err := c.sdk.Payments.Create(ctx, req)
	if err != nil {
		c.log(ctx, "error", "create_payment", map[string]any{"error": err.Error()})
		return nil, c.mapSquareError(err, "create payment")
	}

	payment := resp.GetPayment()
	c.log(ctx, "response", "create_payment", map[string]any{
		"payment_id": stringValue(payment.GetID()),
		"status":     stringValue(payment.GetStatus()),
	})
	return payment, nil
}

func (c *Client) ensureIdempotencyKey(prefix, provided string) string {
	if strings.TrimSpace(provided) != "" {
		return provided
	}
	return c.NewIdempotencyKey(prefix)
}

func (c *Client) log(ctx context.Context, phase, op string, fields map[string]any) {
	if c == nil || c.logger == nil {
		return
	}
	logFields := map[string]any{
		"operation": op,
		"phase":     phase,
	}
	for k, v := range fields {
		logFields[k] = c.redact(k, v)
	}
	ctx = c.logger.WithFields(ctx, logFields)
	switch phase {
	case "error":
		c.logger.Error(ctx, fmt.Sprintf("square %s", op), errors.New(fmt.Sprint(fields["error"])))
	default:
		c.logger.Info(ctx, fmt.Sprintf("square %s", phase))
	}
}

func (c *Client) redact(key string, value any) any {
	lower := strings.ToLower(key)
	for _, sensitive := range []string{"card", "nonce", "token", "cvv", "cvc", "secret", "email", "phone"} {
		if strings.Contains(lower, sensitive) {
			return "[REDACTED]"
		}
	}
	return value
}

func (c *Client) mapSquareError(err error, op string) error {
	if err == nil {
		return nil
	}
	var apiErr *sqcore.APIError
	if errors.As(err, &apiErr) {
		code := domainCodeForStatus(apiErr.StatusCode)
		for _, sqErr := range c.extractSquareErrors(apiErr) {
			if sqErr == nil {
				continue
			}
			if sqErr.Code == sq.ErrorCodeIdempotencyKeyReused {
				code = pkgerrors.CodeIdempotency
				break
			}
			if sqErr.Category == sq.ErrorCategoryAuthenticationError {
				code = pkgerrors.CodeUnauthorized
				break
			}
		}
		return pkgerrors.Wrap(code, err, fmt.Sprintf("square %s failed", op))
	}
	return pkgerrors.Wrap(pkgerrors.CodeDependency, err, fmt.Sprintf("square %s failed", op))
}

func (c *Client) extractSquareErrors(apiErr *sqcore.APIError) []*sq.Error {
	if apiErr == nil {
		return nil
	}
	inner := apiErr.Unwrap()
	if inner == nil {
		return nil
	}
	raw := strings.TrimSpace(inner.Error())
	if raw == "" {
		return nil
	}
	var payload struct {
		Errors []*sq.Error `json:"errors"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload.Errors
}

func domainCodeForStatus(status int) pkgerrors.Code {
	switch status {
	case http.StatusUnauthorized:
		return pkgerrors.CodeUnauthorized
	case http.StatusForbidden:
		return pkgerrors.CodeForbidden
	case http.StatusNotFound:
		return pkgerrors.CodeNotFound
	case http.StatusConflict:
		return pkgerrors.CodeConflict
	case http.StatusTooManyRequests:
		return pkgerrors.CodeRateLimit
	case http.StatusBadRequest:
		return pkgerrors.CodeValidation
	case http.StatusUnprocessableEntity:
		return pkgerrors.CodeStateConflict
	default:
		if status >= 400 && status < 500 {
			return pkgerrors.CodeValidation
		}
		return pkgerrors.CodeDependency
	}
}

func stringValue(ptr *string) string {
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

func normalizeEnv(raw string) (string, error) {
	env := strings.TrimSpace(strings.ToLower(raw))
	if env == "" {
		env = sandboxEnv
	}
	switch env {
	case sandboxEnv, productionEnv:
		return env, nil
	default:
		return "", errInvalidSquareEnv
	}
}
