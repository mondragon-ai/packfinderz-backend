package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	nameSanitizeRe = regexp.MustCompile(`[^a-z0-9_]+`)
)

// CreateSQLMigration creates a goose SQL migration file:
//
//	<dir>/<YYYYMMDDHHMMSS>_<name>.sql
func CreateSQLMigration(dir string, name string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("dir is required")
	}
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	safe := strings.ToLower(strings.TrimSpace(name))
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = nameSanitizeRe.ReplaceAllString(safe, "_")
	safe = strings.Trim(safe, "_")
	if safe == "" {
		return "", fmt.Errorf("name %q results in empty sanitized filename", name)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %q: %w", dir, err)
	}

	version := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", version, safe)
	fullpath := filepath.Join(dir, filename)

	// fail if exists
	if _, err := os.Stat(fullpath); err == nil {
		return "", fmt.Errorf("migration already exists: %s", fullpath)
	}

	template := fmt.Sprintf(`-- +goose Up
-- +goose StatementBegin
-- %s
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- rollback %s
-- +goose StatementEnd
`, safe, safe)

	if err := os.WriteFile(fullpath, []byte(template), 0o644); err != nil {
		return "", fmt.Errorf("write migration %q: %w", fullpath, err)
	}

	return fullpath, nil
}
