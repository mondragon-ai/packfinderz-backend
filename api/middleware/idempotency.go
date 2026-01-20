package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	pkgredis "github.com/angelmondragon/packfinderz-backend/pkg/redis"
)

const (
	defaultIdempotencyTTL  = 24 * time.Hour
	criticalIdempotencyTTL = 7 * 24 * time.Hour
)

type routeMatcher func(string) bool

type idempotencyRule struct {
	method  string
	matcher routeMatcher
	ttl     time.Duration
}

var idempotencyRules = []idempotencyRule{
	// 24h TTL endpoints
	{method: http.MethodPost, matcher: matchExact("/api/v1/auth/register"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/stores/me/users/invite"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/licenses"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/media/presign"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/media/finalize"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefixSuffix("/api/v1/products/", "/media/attach"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPut, matcher: matchExact("/api/v1/cart"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefixSuffix("/api/v1/orders/", "/nudge"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefixSuffix("/api/v1/notifications/", "/read"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/notifications/read-all"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/vendor/products"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPut, matcher: matchPrefix("/api/v1/inventory/"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/vendor/ads"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/vendor/payment-methods/cc"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/vendor/subscriptions"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchExact("/api/v1/vendor/subscriptions/cancel"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefix("/api/v1/admin/orders/"), ttl: defaultIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefix("/api/v1/agent/orders/"), ttl: defaultIdempotencyTTL},
	// 7d TTL endpoints
	{method: http.MethodPost, matcher: matchExact("/api/v1/checkout"), ttl: criticalIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefixSuffix("/api/v1/orders/", "/cancel"), ttl: criticalIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefixSuffix("/api/v1/orders/", "/retry"), ttl: criticalIdempotencyTTL},
	{method: http.MethodPost, matcher: matchPrefixSuffix("/api/v1/vendor/orders/", "/decision"), ttl: criticalIdempotencyTTL},
}

type idempotencyRecord struct {
	Status      int               `json:"status"`
	Body        string            `json:"body"`
	Headers     map[string]string `json:"headers,omitempty"`
	RequestHash string            `json:"request_hash"`
}

func Idempotency(store pkgredis.IdempotencyStore, logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := routePattern(r)
			ttl, ok := routeTTL(r.Method, pattern)
			if !ok || store == nil {
				next.ServeHTTP(w, r)
				return
			}

			idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if idempotencyKey == "" {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "Idempotency-Key header required"))
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "read request"))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			requestHash := hashBody(body)
			scope := buildScope(r)
			key := store.IdempotencyKey(scope, idempotencyKey)

			if stored, getErr := store.Get(r.Context(), key); getErr != nil && !errors.Is(getErr, redis.Nil) {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, getErr, "check idempotency"))
				return
			} else if stored != "" {
				record, decodeErr := decodeRecord(stored)
				if decodeErr != nil {
					responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, decodeErr, "decode idempotency record"))
					return
				}
				if record.RequestHash != requestHash {
					responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeIdempotency, "idempotency key reused with different request body"))
					return
				}
				writeStoredResponse(w, record)
				return
			}

			rec := &responseCapture{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			record := idempotencyRecord{
				Status:      defaultStatus(rec.status),
				Body:        base64.StdEncoding.EncodeToString(rec.body.Bytes()),
				RequestHash: requestHash,
			}
			if ct := rec.Header().Get("Content-Type"); ct != "" {
				record.Headers = map[string]string{"Content-Type": ct}
			}

			payload, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				logError(r.Context(), logg, "marshal idempotency record", marshalErr)
				return
			}

			if _, setErr := store.SetNX(r.Context(), key, string(payload), ttl); setErr != nil {
				logError(r.Context(), logg, "persist idempotency record", setErr)
			}
		})
	}
}

func buildScope(r *http.Request) string {
	parts := []string{
		UserIDFromContext(r.Context()),
		StoreIDFromContext(r.Context()),
		r.Method,
		r.URL.Path,
	}
	return strings.Join(parts, "|")
}

func decodeRecord(payload string) (*idempotencyRecord, error) {
	var record idempotencyRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func writeStoredResponse(w http.ResponseWriter, record *idempotencyRecord) {
	if record == nil {
		return
	}
	if ct, ok := record.Headers["Content-Type"]; ok && ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(record.Status)
	if decoded, err := base64.StdEncoding.DecodeString(record.Body); err == nil {
		_, _ = w.Write(decoded)
	}
}

func hashBody(payload []byte) string {
	sum := sha256.Sum256(payload)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func defaultStatus(value int) int {
	if value == 0 {
		return http.StatusOK
	}
	return value
}

func routePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	if ctx := chi.RouteContext(r.Context()); ctx != nil {
		if pattern := ctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func routeTTL(method, pattern string) (time.Duration, bool) {
	if pattern == "" {
		return 0, false
	}
	for _, rule := range idempotencyRules {
		if rule.method != method {
			continue
		}
		if rule.matcher(pattern) {
			return rule.ttl, true
		}
	}
	return 0, false
}

func matchExact(path string) routeMatcher {
	return func(pattern string) bool {
		return pattern == path
	}
}

func matchPrefix(prefix string) routeMatcher {
	return func(pattern string) bool {
		return strings.HasPrefix(pattern, prefix)
	}
}

func matchPrefixSuffix(prefix, suffix string) routeMatcher {
	return func(pattern string) bool {
		return strings.HasPrefix(pattern, prefix) && strings.HasSuffix(pattern, suffix)
	}
}

type responseCapture struct {
	http.ResponseWriter
	body   bytes.Buffer
	status int
}

func (r *responseCapture) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseCapture) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func logError(ctx context.Context, logg *logger.Logger, msg string, err error) {
	if logg == nil || err == nil {
		return
	}
	logg.Error(ctx, msg, err)
}
