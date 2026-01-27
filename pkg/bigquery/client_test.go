package bigquery

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
)

func TestConfiguredTables(t *testing.T) {
	cfg := config.BigQueryConfig{
		MarketplaceEventsTable: " marketplace_events ",
		AdEventsTable:          "",
	}

	tables := configuredTables(cfg)

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if tables[0] != "marketplace_events" {
		t.Fatalf("expected marketplace_events, got %s", tables[0])
	}
}

func TestClientOptionsPrioritizesJSON(t *testing.T) {
	gcp := config.GCPConfig{
		CredentialsJSON:        `{"dummy": "value"}`,
		ApplicationCredentials: "/tmp/creds",
	}

	opts := clientOptions(gcp)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
}

func TestClientOptionsWithFile(t *testing.T) {
	gcp := config.GCPConfig{
		ApplicationCredentials: "/tmp/creds",
	}

	opts := clientOptions(gcp)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option when using credentials file, got %d", len(opts))
	}
}

func TestClientOptionsEmpty(t *testing.T) {
	gcp := config.GCPConfig{}

	opts := clientOptions(gcp)
	if len(opts) != 0 {
		t.Fatalf("expected 0 options when no credentials provided, got %d", len(opts))
	}
}
