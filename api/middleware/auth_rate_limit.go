package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type rateLimiterStore interface {
	IncrWithTTL(context.Context, string, time.Duration) (int64, error)
}

// AuthRateLimitPolicy defines the throttling parameters for a traffic surface.
type AuthRateLimitPolicy struct {
	name       string
	window     time.Duration
	ipLimit    int
	emailLimit int
}

// NewAuthRateLimitPolicy builds a policy with the supplied window and limits.
func NewAuthRateLimitPolicy(name string, window time.Duration, ipLimit, emailLimit int) AuthRateLimitPolicy {
	return AuthRateLimitPolicy{
		name:       strings.ToLower(strings.TrimSpace(name)),
		window:     window,
		ipLimit:    ipLimit,
		emailLimit: emailLimit,
	}
}

func (p AuthRateLimitPolicy) enabled() bool {
	return p.window > 0 && (p.ipLimit > 0 || p.emailLimit > 0)
}

func (p AuthRateLimitPolicy) normalizedName() string {
	if p.name == "" {
		return "auth"
	}
	return p.name
}

func (p AuthRateLimitPolicy) ipKey(ip string) string {
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("rl:ip:%s:%s", p.normalizedName(), ip)
}

func (p AuthRateLimitPolicy) emailKey(hash string) string {
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("rl:email:%s:%s", p.normalizedName(), hash)
}

// AuthRateLimit enforces per-IP and per-email counters for auth endpoints.
func AuthRateLimit(policy AuthRateLimitPolicy, store rateLimiterStore, logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !policy.enabled() || store == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ip := clientIP(r)
			if policy.ipLimit > 0 {
				if key := policy.ipKey(ip); key != "" {
					if allowed, count, err := allow(ctx, store, key, policy.window, int64(policy.ipLimit)); err != nil {
						responses.WriteError(ctx, nil, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "rate limiting"))
						return
					} else if !allowed {
						respondRateLimited(ctx, logg, w, policy, "ip", ip, "", count, policy.ipLimit)
						return
					}
				}
			}

			var body []byte
			if policy.emailLimit > 0 {
				var err error
				body, err = io.ReadAll(r.Body)
				if err != nil {
					responses.WriteError(ctx, nil, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "read request"))
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))

				email := normalizeEmail(extractEmail(body))
				if email != "" {
					hash := hashValue(email)
					if key := policy.emailKey(hash); key != "" {
						if allowed, count, err := allow(ctx, store, key, policy.window, int64(policy.emailLimit)); err != nil {
							responses.WriteError(ctx, nil, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "rate limiting"))
							return
						} else if !allowed {
							respondRateLimited(ctx, logg, w, policy, "email", "", hash, count, policy.emailLimit)
							return
						}
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func allow(ctx context.Context, store rateLimiterStore, key string, window time.Duration, limit int64) (bool, int64, error) {
	count, err := store.IncrWithTTL(ctx, key, window)
	if err != nil {
		return false, 0, err
	}
	return count <= limit, count, nil
}

func respondRateLimited(ctx context.Context, logg *logger.Logger, w http.ResponseWriter, policy AuthRateLimitPolicy, scope, ip, emailHash string, count int64, limit int) {
	if logg != nil {
		fields := map[string]any{
			"scope":          scope,
			"policy":         policy.normalizedName(),
			"attempts":       count,
			"limit":          limit,
			"window_seconds": int(policy.window.Seconds()),
		}
		if ip != "" {
			fields["ip"] = ip
		}
		if emailHash != "" {
			fields["email_hash"] = emailHash
		}
		logCtx := logg.WithFields(ctx, fields)
		logg.Warn(logCtx, "auth.rate_limit.blocked")
	}
	err := pkgerrors.New(pkgerrors.CodeRateLimit, "rate limit exceeded")
	responses.WriteError(ctx, nil, w, err)
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if header := r.Header.Get("X-Forwarded-For"); header != "" {
		for _, part := range strings.Split(header, ",") {
			if ip := strings.TrimSpace(part); ip != "" {
				return ip
			}
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func extractEmail(payload []byte) string {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	return body.Email
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
