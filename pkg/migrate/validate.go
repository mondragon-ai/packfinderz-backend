package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	sqlFileRe = regexp.MustCompile(`^(\d{14})_[a-z0-9_]+\.sql$`)
)

// ValidateDir validates migration filenames + basic SQL headers.
func ValidateDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("dir is required")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", dir, err)
	}

	seen := map[string]string{} // version -> filename

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		m := sqlFileRe.FindStringSubmatch(name)
		if m == nil {
			return fmt.Errorf("invalid migration filename %q (expected YYYYMMDDHHMMSS_name.sql)", name)
		}

		version := m[1]
		if prev, ok := seen[version]; ok {
			return fmt.Errorf("duplicate migration version %s in %q and %q", version, prev, name)
		}
		seen[version] = name

		full := filepath.Join(dir, name)
		b, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("read file %q: %w", full, err)
		}

		txt := string(b)
		if !strings.Contains(txt, "-- +goose Up") {
			return fmt.Errorf("migration %q missing \"-- +goose Up\"", name)
		}
		if !strings.Contains(txt, "-- +goose Down") {
			return fmt.Errorf("migration %q missing \"-- +goose Down\"", name)
		}
	}

	// If no sql migrations exist, that's allowed (early repo), but you can hard-fail if you want.
	return nil
}
