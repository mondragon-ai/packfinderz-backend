package migrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryMigrationContainsConstraints(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("migrations", "*_create_inventory_items.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no inventory migration file found")
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}
	content := string(data)

	checks := []string{
		"CREATE TABLE IF NOT EXISTS inventory_items",
		"FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE",
		"CHECK (available_qty >= 0)",
		"CHECK (reserved_qty >= 0)",
		"DROP TABLE IF EXISTS inventory_items",
	}

	for _, sub := range checks {
		if !strings.Contains(content, sub) {
			t.Errorf("missing expected statement %q", sub)
		}
	}
}
