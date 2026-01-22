package pubsub

import (
	"context"
	"strings"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
)

func TestNewClient_ProjectIDRequired(t *testing.T) {
	_, err := NewClient(context.Background(), config.GCPConfig{}, config.PubSubConfig{}, nil)
	if err == nil || !strings.Contains(err.Error(), "gcp project id is required") {
		t.Fatalf("expected project id error, got %v", err)
	}
}

func TestClient_PingWithoutInit(t *testing.T) {
	var client *Client
	err := client.Ping(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected not-initialized error, got %v", err)
	}
}
