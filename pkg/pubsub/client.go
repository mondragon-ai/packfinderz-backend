package pubsub

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type Client struct {
	client *pubsub.Client
	cfg    config.PubSubConfig
}

var (
	errProjectIDRequired = errors.New("gcp project id is required")
	errNoSubscriptions   = errors.New("pubsub subscription name is required")
)

// NewClient creates a Pub/Sub client and ensures the configured subscriptions exist.
func NewClient(ctx context.Context, gcp config.GCPConfig, cfg config.PubSubConfig, logg *logger.Logger) (*Client, error) {
	if gcp.ProjectID == "" {
		return nil, errProjectIDRequired
	}

	psClient, err := pubsub.NewClient(ctx, gcp.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("creating pubsub client: %w", err)
	}

	c := &Client{
		client: psClient,
		cfg:    cfg,
	}

	if err := c.ensureSubscriptionsConfigured(ctx); err != nil {
		_ = psClient.Close()
		return nil, err
	}

	if logg != nil {
		logg.Info(ctx, "pubsub client initialized")
	}

	return c, nil
}

func (c *Client) ensureSubscriptionsConfigured(ctx context.Context) error {
	names := subscriptionNames(c.cfg)
	if len(names) == 0 {
		return errNoSubscriptions
	}
	for _, name := range names {
		if err := c.ensureSubscriptionExists(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func subscriptionNames(cfg config.PubSubConfig) []string {
	names := []string{}
	for _, name := range []string{
		cfg.MediaSubscription,
		cfg.OrdersSubscription,
		cfg.BillingSubscription,
	} {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

func (c *Client) ensureSubscriptionExists(ctx context.Context, name string) error {
	sub := c.Subscription(name)
	if sub == nil {
		return fmt.Errorf("subscription %q not configured", name)
	}
	ok, err := sub.Exists(ctx)
	if err != nil {
		return fmt.Errorf("checking subscription %q: %w", name, err)
	}
	if !ok {
		return fmt.Errorf("subscription %q does not exist", name)
	}
	return nil
}

// Subscription returns the named subscription handle or nil if it is not configured.
func (c *Client) Subscription(name string) *pubsub.Subscription {
	if c == nil || c.client == nil {
		return nil
	}
	if strings.TrimSpace(name) == "" {
		return nil
	}
	return c.client.Subscription(name)
}

// MediaSubscription returns the configured media subscription.
func (c *Client) MediaSubscription() *pubsub.Subscription {
	return c.Subscription(c.cfg.MediaSubscription)
}

// OrdersSubscription returns the configured orders subscription.
func (c *Client) OrdersSubscription() *pubsub.Subscription {
	return c.Subscription(c.cfg.OrdersSubscription)
}

// BillingSubscription returns the configured billing subscription.
func (c *Client) BillingSubscription() *pubsub.Subscription {
	return c.Subscription(c.cfg.BillingSubscription)
}

// Ping verifies Pub/Sub connectivity by checking configured subscriptions exist.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil {
		return errors.New("pubsub client not initialized")
	}
	return c.ensureSubscriptionsConfigured(ctx)
}

// Close releases the Pub/Sub client resources.
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
