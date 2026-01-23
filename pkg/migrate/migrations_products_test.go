package migrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductsMigrationContainsSchemas(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("migrations", "*_create_products_table.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no product migration file found")
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}
	content := string(data)

	checks := []string{
		"CREATE TYPE IF NOT EXISTS category AS ENUM",
		"CREATE TYPE IF NOT EXISTS classification AS ENUM",
		"CREATE TYPE IF NOT EXISTS unit AS ENUM",
		"CREATE TYPE IF NOT EXISTS flavors AS ENUM",
		"CREATE TYPE IF NOT EXISTS feelings AS ENUM",
		"CREATE TYPE IF NOT EXISTS usage AS ENUM",
		"CREATE TABLE IF NOT EXISTS products",
		"CREATE TABLE IF NOT EXISTS product_media",
		"CREATE INDEX IF NOT EXISTS idx_products_store_is_active",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_product_media_product_position",
	}

	for _, sub := range checks {
		if !strings.Contains(content, sub) {
			t.Errorf("missing expected statement %q", sub)
		}
	}
}
