# PKG & API Reference

## PKG
### `config`
- central `Config` struct driven by `envconfig` plus typed sub-configs (App, Service, DB, Redis, JWT, FeatureFlags, OpenAI, GoogleMaps, GCP, GCS, Media, Pub/Sub, Stripe, Sendgrid).
- `DBConfig.ensureDSN` synthesizes legacy host/user/password vars into `PACKFINDERZ_DB_DSN` when missing.

### `db`
- `Client` boots GORM (Postgres driver via `gorm.io/driver/postgres`), exposes `DB()` accessor, `Ping`, `Close`, context-aware `Exec/Raw`, and `WithTx` that auto rollbacks on errors/panics.

### `migrate`
- Goose helpers (`pkg/migrate/migrate.go`) wrap `goose.Run`, `MigrateToVersion`, and validation (naming + headers).
- `autorun.go` gates dev auto-migrations behind `PACKFINDERZ_APP_ENV=dev` + `PACKFINDERZ_AUTO_MIGRATE`.
- `create.go` templated generation.

### `redis`
- `Client` configures go-redis with URL/address, pooling, TTLs and exposes helpers: Set/Get, SetNX, Incr, IncrWithTTL, FixedWindowAllow, idempotency/rate-limit/counter/refresh token key builders, refresh token CRUD, Ping, Close.

### `logger`
- Structured `zerolog` wrapper with `Options` (level, warn stack, output), `ParseLevel`, context enrichment helpers (`WithField/Fields/RequestID/UserID/StoreID/ActorRole`), and Info/Warn/Error log helpers that honor warn-stack + stack traces.

### `errors`
- Typed error envelope: `Code` enum (`VALIDATION_ERROR`, `UNAUTHORIZED`, ..., `INTERNAL_ERROR`, `DEPENDENCY_ERROR`), `Metadata` lookup (HTTP status, retryable, public message, details flag), `Error` struct builder with `New`, `Wrap`, `WithDetails`, and `As`.
- `MetadataFor` ensures canonical mapping used by API responders.

### Shared helpers (DTOs / Enums / Types)
- `pkg/types` supplies `SuccessEnvelope` and `ErrorEnvelope` (with `APIError` payload) to standardize JSON responses.
- `pkg/errors` codes effectively act as enums for error contracts; controllers reuse them to shape `responses.WriteError`.

## API
### Routes
- `/health` (live + ready) under public health router.
- `/api/public` exposes `POST /validate` and `GET /ping`.
- `/api` requires auth/store + idempotency/rate-limit placeholders and currently only serves a `GET /ping`.
- `/api/admin` and `/api/agent` mount the same guards plus `RequireRole("admin")` or `RequireRole("agent")` before exposing their pings.

### Middleware
- `Recoverer`: catches panics, logs with stack, returns `pkg/errors.CodeInternal`.
- `RequestID`: ensures `X-Request-Id` on request/response and attaches to logs.
- `Logging`: records start/complete events with method/path/status/duration via `statusRecorder`.
- `Auth`: validates Bearer token via `api/validators.ParseAuthToken`, injects user/store/role into context+logger.
- `StoreContext`: enforces that `StoreID` exists before proceeding.
- `RequireRole`: gate for admin/agent endpoints; fails with `pkg/errors.CodeForbidden`.
- `Idempotency` / `RateLimit`: placeholders that currently pass through (planned hooks).

### Responses
- `responses.WriteSuccess` / `WriteSuccessStatus` envelope payloads with `pkg/types.SuccessEnvelope`.
- `responses.WriteError` unwraps `pkg/errors.Error`, maps to `pkg/errors.Metadata`, logs via `pkg/logger`, and marshals into `pkg/types.ErrorEnvelope` while honoring `DetailsAllowed`.
