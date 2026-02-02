package analytics

import (
	"net/http"
	"strings"
	"time"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}

func resolveAnalyticsRange(r *http.Request, now time.Time) (time.Time, time.Time, error) {
	query := r.URL.Query()
	from := strings.TrimSpace(query.Get("from"))
	to := strings.TrimSpace(query.Get("to"))

	if from != "" || to != "" {
		if from == "" || to == "" {
			return time.Time{}, time.Time{}, pkgerrors.New(pkgerrors.CodeValidation, "from and to must be provided together")
		}
		start, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return time.Time{}, time.Time{}, pkgerrors.New(pkgerrors.CodeValidation, "invalid from timestamp")
		}
		end, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return time.Time{}, time.Time{}, pkgerrors.New(pkgerrors.CodeValidation, "invalid to timestamp")
		}
		start = start.UTC()
		end = end.UTC()
		if end.Before(start) {
			return time.Time{}, time.Time{}, pkgerrors.New(pkgerrors.CodeValidation, "end must be after start")
		}
		return start, end, nil
	}

	preset := strings.TrimSpace(query.Get("preset"))
	duration, ok := presetDuration(preset)
	if !ok {
		return time.Time{}, time.Time{}, pkgerrors.New(pkgerrors.CodeValidation, "invalid preset")
	}

	end := now
	start := end.Add(-duration)
	return start, end, nil
}

func presetDuration(value string) (time.Duration, bool) {
	if value == "" {
		value = "30d"
	}
	switch strings.ToLower(value) {
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	case "90d":
		return 90 * 24 * time.Hour, true
	default:
		return 0, false
	}
}
