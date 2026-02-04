package square

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
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
)

// Client stores the Square credentials and metadata required by subscribers and webhooks.
type Client struct {
	accessToken   string
	environment   string
	webhookSecret string
}

// NewClient initializes the Square client metadata and validates the configured secrets.
func NewClient(ctx context.Context, cfg config.SquareConfig, logg *logger.Logger) (*Client, error) {
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

	return &Client{
		accessToken:   accessToken,
		environment:   env,
		webhookSecret: webhookSecret,
	}, nil
}

// AccessToken returns the configured Square access token.
func (c *Client) AccessToken() string {
	if c == nil {
		return ""
	}
	return c.accessToken
}

// Environment reports the normalized Square environment in use.
func (c *Client) Environment() string {
	if c == nil {
		return ""
	}
	return c.environment
}

// SigningSecret returns the configured Square webhook secret.
func (c *Client) SigningSecret() string {
	if c == nil {
		return ""
	}
	return c.webhookSecret
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
