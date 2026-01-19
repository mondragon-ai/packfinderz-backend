package validators

import (
	"net/http"
	"strconv"
	"strings"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

func ParseQueryInt(r *http.Request, key string, defaultVal, min, max int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return defaultVal, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, pkgerrors.New(pkgerrors.CodeValidation, "query parameter must be numeric").WithDetails(map[string]any{"field": key})
	}
	if value < min || value > max {
		return 0, pkgerrors.New(pkgerrors.CodeValidation, "query parameter out of range").WithDetails(map[string]any{"field": key, "min": min, "max": max})
	}
	return value, nil
}
