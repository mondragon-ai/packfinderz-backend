package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Success(t *testing.T) {
	setMinimalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.App.Env != "production" {
		t.Fatalf("expected App.Env to be production, got %q", cfg.App.Env)
	}

	if cfg.Redis.URL != "redis://localhost:6379/0" {
		t.Fatalf("unexpected Redis URL: %q", cfg.Redis.URL)
	}

	if got := cfg.GCS.UploadURLExpiry; got != 15*time.Minute {
		t.Fatalf("expected upload expiry 15m, got %v", got)
	}

	if cfg.PubSub.MediaTopic != "media-topic" {
		t.Fatalf("unexpected media topic %q", cfg.PubSub.MediaTopic)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	setMinimalEnv(t)
	if err := os.Unsetenv(EnvAppEnv); err != nil {
		t.Fatalf("failed to unset %s: %v", EnvAppEnv, err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("expected missing required env to return an error")
	}
}

func setMinimalEnv(t *testing.T) {
	t.Helper()

	t.Setenv(EnvAppEnv, "production")
	t.Setenv(EnvPort, "8081")
	t.Setenv(EnvDBDSN, "postgres://user:pass@localhost:5432/packfinderz?sslmode=disable")
	t.Setenv(EnvRedisURL, "redis://localhost:6379/0")
	t.Setenv(EnvJWTSecret, "secret")
	t.Setenv(EnvJWTIssuer, "packfinderz")
	t.Setenv(EnvJWTExpMins, "60")
	t.Setenv(EnvRefreshTokenTTLMinutes, "43200")
	t.Setenv(EnvGCPProjectID, "project-123")
	t.Setenv(EnvGCSBucket, "bucket")
	t.Setenv(EnvGCSUploadExpiry, "15m")
	t.Setenv(EnvGCSDownloadExpiry, "24h")
	t.Setenv(EnvPubSubMediaTopic, "media-topic")
	t.Setenv(EnvPubSubMediaSub, "media-sub")
	t.Setenv(EnvPubSubOrdersTopic, "orders-topic")
	t.Setenv(EnvPubSubOrdersSub, "orders-sub")
	t.Setenv(EnvPubSubBillingTopic, "billing-topic")
	t.Setenv(EnvPubSubBillingSub, "billing-sub")
	t.Setenv(EnvPubSubDomainTopic, "domain-topic")
	t.Setenv(EnvPubSubDomainSub, "domain-sub")
}

func TestAppConfigEnvHelpers(t *testing.T) {
	devConfig := AppConfig{Env: "DEV"}
	if !devConfig.IsDev() {
		t.Fatalf("expected IsDev true for %q", devConfig.Env)
	}
	if devConfig.IsProd() {
		t.Fatalf("expected IsProd false for %q", devConfig.Env)
	}

	prodConfig := AppConfig{Env: "prod"}
	if !prodConfig.IsProd() {
		t.Fatalf("expected IsProd true for %q", prodConfig.Env)
	}
	if prodConfig.IsDev() {
		t.Fatalf("expected IsDev false for %q", prodConfig.Env)
	}
}
