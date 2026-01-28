package stripe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/stripe/stripe-go/v84"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

const (
	testEnv = "test"
	liveEnv = "live"
)

var (
	errAPIKeyRequired   = errors.New("stripe api key is required")
	errSecretRequired   = errors.New("stripe webhook secret is required")
	errInvalidStripeEnv = fmt.Errorf("stripe environment must be %q or %q", testEnv, liveEnv)
)

// Client wraps Stripe's API client plus env-specific metadata.
type Client struct {
	api           *stripe.Client
	environment   string
	signingSecret string
}

// NewClient initializes Stripe once with the configured secrets and env.
func NewClient(ctx context.Context, cfg config.StripeConfig, logg *logger.Logger) (*Client, error) {
	env, err := normalizeEnv(cfg.Environment())
	if err != nil {
		return nil, err
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errAPIKeyRequired
	}

	signingSecret := strings.TrimSpace(cfg.Secret)
	if signingSecret == "" {
		return nil, errSecretRequired
	}

	if err := validateAPIKey(env, apiKey); err != nil {
		return nil, err
	}

	api := stripe.NewClient(apiKey)
	stripe.Key = apiKey

	if logg != nil {
		logg.Info(ctx, fmt.Sprintf("stripe client initialized (%s)", env))
	}

	return &Client{
		api:           api,
		environment:   env,
		signingSecret: signingSecret,
	}, nil
}

// API returns the underlying Stripe API client.
func (c *Client) API() *stripe.Client {
	if c == nil {
		return nil
	}
	return c.api
}

// Environment reports the normalized Stripe environment in use.
func (c *Client) Environment() string {
	if c == nil {
		return ""
	}
	return c.environment
}

// SigningSecret returns the webhook signing secret.
func (c *Client) SigningSecret() string {
	if c == nil {
		return ""
	}
	return c.signingSecret
}

func normalizeEnv(raw string) (string, error) {
	env := strings.TrimSpace(strings.ToLower(raw))
	if env == "" {
		env = testEnv
	}
	switch env {
	case testEnv, liveEnv:
		return env, nil
	default:
		return "", errInvalidStripeEnv
	}
}

func validateAPIKey(env, key string) error {
	switch env {
	case testEnv:
		if strings.HasPrefix(key, "sk_test") || strings.HasPrefix(key, "rk_test") {
			return nil
		}
		return fmt.Errorf("stripe environment %q requires a test secret key (sk_test/rk_test)", testEnv)
	case liveEnv:
		if strings.HasPrefix(key, "sk_live") || strings.HasPrefix(key, "rk_live") {
			return nil
		}
		return fmt.Errorf("stripe environment %q requires a live secret key (sk_live/rk_live)", liveEnv)
	default:
		return errInvalidStripeEnv
	}
}
