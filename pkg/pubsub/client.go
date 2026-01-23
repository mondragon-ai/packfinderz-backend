// pkg/pubsub/client.go
package pubsub

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client    *pubsub.Client
	projectID string
	cfg       config.PubSubConfig
}

var (
	errProjectIDRequired = errors.New("gcp project id is required")
	errNoSubscriptions   = errors.New("pubsub subscription name is required")
)

// NewClient creates a Pub/Sub v2 client and ensures the configured subscriptions exist.
func NewClient(ctx context.Context, gcp config.GCPConfig, cfg config.PubSubConfig, logg *logger.Logger) (*Client, error) {
	if strings.TrimSpace(gcp.ProjectID) == "" {
		return nil, errProjectIDRequired
	}

	psClient, err := pubsub.NewClient(ctx, gcp.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("creating pubsub client: %w", err)
	}

	c := &Client{
		client:    psClient,
		projectID: gcp.ProjectID,
		cfg:       cfg,
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
		cfg.DomainSubscription,
	} {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

func (c *Client) ensureSubscriptionExists(ctx context.Context, name string) error {
	fullName := c.subscriptionResourceName(name)
	if fullName == "" {
		return fmt.Errorf("subscription %q not configured", name)
	}

	_, err := c.client.SubscriptionAdminClient.GetSubscription(
		ctx,
		&pubsubpb.GetSubscriptionRequest{Subscription: fullName},
	)
	if err != nil {
		// v2 uses gRPC errors; NotFound means the subscription doesn't exist.
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("subscription %q does not exist", name)
		}
		return fmt.Errorf("checking subscription %q: %w", name, err)
	}

	return nil
}

// Subscription returns a v2 Subscriber handle for the configured subscription name (ID or full resource name).
func (c *Client) Subscription(name string) *pubsub.Subscriber {
	if c == nil || c.client == nil {
		return nil
	}
	fullName := c.subscriptionResourceName(name)
	if fullName == "" {
		return nil
	}
	return c.client.Subscriber(fullName)
}

// MediaSubscription returns the configured media subscription subscriber.
func (c *Client) MediaSubscription() *pubsub.Subscriber {
	return c.Subscription(c.cfg.MediaSubscription)
}

// OrdersSubscription returns the configured orders subscription subscriber.
func (c *Client) OrdersSubscription() *pubsub.Subscriber {
	return c.Subscription(c.cfg.OrdersSubscription)
}

// BillingSubscription returns the configured billing subscription subscriber.
func (c *Client) BillingSubscription() *pubsub.Subscriber {
	return c.Subscription(c.cfg.BillingSubscription)
}

// DomainSubscription returns the configured domain subscription handle.
func (c *Client) DomainSubscription() *pubsub.Subscriber {
	return c.Subscription(c.cfg.DomainSubscription)
}

// Publisher returns a publisher handle for the given topic ID/resource name.
func (c *Client) Publisher(name string) *pubsub.Publisher {
	if c == nil || c.client == nil {
		return nil
	}
	fullName := c.topicResourceName(name)
	if fullName == "" {
		return nil
	}
	return c.client.Publisher(fullName)
}

// DomainPublisher returns the configured domain event publisher.
func (c *Client) DomainPublisher() *pubsub.Publisher {
	return c.Publisher(c.cfg.DomainTopic)
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

func (c *Client) subscriptionResourceName(name string) string {
	if c == nil {
		return ""
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return ""
	}

	if strings.HasPrefix(n, "projects/") && strings.Contains(n, "/subscriptions/") {
		return n
	}

	p := strings.TrimSpace(c.projectID)
	if p == "" {
		return ""
	}
	return fmt.Sprintf("projects/%s/subscriptions/%s", p, n)
}

func (c *Client) topicResourceName(name string) string {
	if c == nil {
		return ""
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return ""
	}
	if strings.HasPrefix(n, "projects/") && strings.Contains(n, "/topics/") {
		return n
	}
	p := strings.TrimSpace(c.projectID)
	if p == "" {
		return ""
	}
	return fmt.Sprintf("projects/%s/topics/%s", p, n)
}
