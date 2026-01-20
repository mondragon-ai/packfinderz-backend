# PKG & API Reference

## PKG

### `config`

* central `Config` struct driven by `envconfig` plus typed sub-configs (App, Service, DB, Redis, JWT, FeatureFlags, OpenAI, GoogleMaps, GCP, GCS, Media, Pub/Sub, Stripe, Sendgrid).
* `DBConfig.ensureDSN` synthesizes legacy host/user/password vars into `PACKFINDERZ_DB_DSN` when missing.

### `db`

* `Client` boots GORM (Postgres driver via `gorm.io/driver/postgres`), exposes `DB()` accessor, `Ping`, `Close`, context-aware `Exec/Raw`, and `WithTx` that auto rollbacks on errors/panics.

### `migrate`

* Goose helpers (`pkg/migrate/migrate.go`) wrap `goose.Run`, `MigrateToVersion`, and validation (naming + headers).
* `autorun.go` gates dev auto-migrations behind `PACKFINDERZ_APP_ENV=dev` + `PACKFINDERZ_AUTO_MIGRATE`.
* `create.go` templated generation.

### `redis`

* `Client` configures go-redis with URL/address, pooling, TTLs and exposes helpers: Set/Get, SetNX, Incr, IncrWithTTL, FixedWindowAllow, idempotency/rate-limit/counter/refresh token key builders, refresh token CRUD, Ping, Close.

### `logger`

* Structured `zerolog` wrapper with `Options` (level, warn stack, output), `ParseLevel`, context enrichment helpers (`WithField/Fields/RequestID/UserID/StoreID/ActorRole`), and Info/Warn/Error log helpers that honor warn-stack + stack traces.

### `errors`

* Typed error envelope: `Code` enum (`VALIDATION_ERROR`, `UNAUTHORIZED`, ..., `INTERNAL_ERROR`, `DEPENDENCY_ERROR`), `Metadata` lookup (HTTP status, retryable, public message, details flag), `Error` struct builder with `New`, `Wrap`, `WithDetails`, and `As`.
* `MetadataFor` ensures canonical mapping used by API responders.

### Shared helpers (DTOs / Enums / Types)

* `pkg/types` supplies `SuccessEnvelope` and `ErrorEnvelope` (with `APIError` payload) to standardize JSON responses.
* `pkg/errors` codes effectively act as enums for error contracts; controllers reuse them to shape `responses.WriteError`.

#### `pkg/types`: API envelopes

* **SuccessEnvelope**: `{ data: any }`
* **ErrorEnvelope**: `{ error: { code: string, message: string, details?: any } }`
* Used by `responses.WriteSuccess*` + `responses.WriteError` to keep response shapes stable across controllers.

#### `pkg/types`: Postgres composite helpers

* Shared parsing/quoting utilities for Postgres composite types:

  * `quoteCompositeString(value string) string`: escapes `\` and `"` and wraps in quotes.
  * `quoteCompositeNullable(*string) string`: `"NULL"` for nil, else quoted string.
  * `isCompositeNull(value string) bool`: `NULL` case-insensitive check.
  * `parseComposite(raw string, expected int) ([]string, error)`: parses `(a,"b,c",NULL)` into fields (respects quotes + escapes); validates field count when `expected > 0`.
  * `newCompositeNullable(value string) *string`: returns nil when `NULL`, else pointer to value.
  * `toString(value any) (string, bool)`: supports `string`, `[]byte`, and `fmt.Stringer`.
* **Purpose:** allows `sql.Scanner`/`driver.Valuer` implementations to store/retrieve rich structs via Postgres composite columns.

#### `pkg/types`: `Address` (Postgres composite `address_t`)

* Shape:

  * `line1 (string)`, `line2 (*string)`, `city (string)`, `state (string)`, `postal_code (string)`, `country (string)`,
  * `lat (float64)`, `lng (float64)`, `geohash (*string)`
* Implements:

  * `Value() (driver.Value, error)`: builds a composite literal `("...",NULL,"...",... ,lat,lng,NULL)` for insert/update.

    * Validates required fields: `line1`, `city`, `state`, `postal_code`.
    * Defaults `country` to `"US"` when blank.
  * `Scan(value any) error`: parses composite fields back into struct; requires `lat`/`lng`; defaults `country` to `"US"` when blank/NULL.

#### `pkg/types`: `Social` (Postgres composite `social_t`)

* Shape: optional handles/links: `twitter`, `facebook`, `instagram`, `linkedin`, `youtube`, `website` (all `*string`).
* Implements:

  * `Value() (driver.Value, error)`: composite literal with nullable fields.
  * `Scan(value any) error`: decodes 6-field composite into pointers.
* Uses the shared composite helpers (`parseComposite`, `quoteCompositeNullable`, `newCompositeNullable`, `toString`).

#### `pkg/types`: `GeographyPoint` (PostGIS geography POINT)

* Shape: `{ lat: float64, lng: float64 }`
* Implements:

  * `Value() (driver.Value, error)`: returns EWKT string `SRID=4326;POINT(lng lat)` (so Postgres can cast to geography).
  * `Scan(value any) error`: supports:

    * text (`POINT(...)` or `SRID=...;POINT(...)`)
    * WKB bytes (reads byte order + type, expects POINT, extracts lng/lat)
* Includes internal text parsing + WKB decoding helper (`strconvParseFloat`, `fromText`, `fromWKB`).

### `security`

#### `pkg/security/password.go` (Argon2id password hashing)

* Provides an Argon2id implementation with config-driven parameters.
* Key types / functions:

  * `ArgonParams` (memory, time, parallelism, saltLen, keyLen)
  * `HashPassword(password string, cfg config.PasswordConfig) (string, error)`

    * Generates random salt and returns encoded hash:

      * format: `$argon2id$v=19$m=...,t=...,p=...$<saltB64>$<hashB64>`
  * `VerifyPassword(password, encoded string) (bool, error)`

    * Decodes hash string, recomputes, and compares with `subtle.ConstantTimeCompare`.
  * `decodeHash(encoded string) (ArgonParams, salt, hash []byte, error)` with `ErrInvalidHash` for malformed inputs.
  * Parameter clamping helpers:

    * `clampInt`, `clampUint32`
    * `paramsFromConfig(cfg)` clamps:

      * parallelism: 1..255
      * memory: 8 KB .. 512 MB (in KB)
      * time: 1..10
      * saltLen: 8..64
      * keyLen: 16..64

### `enums` (pkg/enums/*)

* Canonical string enums used across DTO validation + models + API contracts. Each enum provides:

  * `String() string`
  * `IsValid() bool`
  * `ParseX(value string) (X, error)` with a clear `invalid ...` error.

#### `StoreType`

* Values: `buyer | vendor`

#### `KYCStatus`

* Values:

  * `pending_verification`
  * `verified`
  * `rejected`
  * `expired`
  * `suspended`

#### `MembershipStatus`

* Values:

  * `invited`
  * `active`
  * `removed`
  * `pending`

#### `MemberRole`

* Values:

  * `owner`
  * `admin`
  * `manager`
  * `viewer`
  * `agent`
  * `staff`
  * `ops`

## API

### Routes

* `/health` (live + ready) under public health router.
* `/api/public` exposes `POST /validate` and `GET /ping`.
* `/api` requires auth/store + idempotency/rate-limit placeholders and currently only serves a `GET /ping`.
* `/api/admin` and `/api/agent` mount the same guards plus `RequireRole("admin")` or `RequireRole("agent")` before exposing their pings.

### Middleware

* `Recoverer`: catches panics, logs with stack, returns `pkg/errors.CodeInternal`.
* `RequestID`: ensures `X-Request-Id` on request/response and attaches to logs.
* `Logging`: records start/complete events with method/path/status/duration via `statusRecorder`.
* `Auth`: validates Bearer token via `api/validators.ParseAuthToken`, injects user/store/role into context+logger.
* `StoreContext`: enforces that `StoreID` exists before proceeding.
* `RequireRole`: gate for admin/agent endpoints; fails with `pkg/errors.CodeForbidden`.
* `Idempotency` / `RateLimit`: placeholders that currently pass through (planned hooks).

### Responses

* `responses.WriteSuccess` / `WriteSuccessStatus` envelope payloads with `pkg/types.SuccessEnvelope`.
* `responses.WriteError` unwraps `pkg/errors.Error`, maps to `pkg/errors.Metadata`, logs via `pkg/logger`, and marshals into `pkg/types.ErrorEnvelope` while honoring `DetailsAllowed`.


### `auth`

#### `pkg/auth/token.go` (Access JWT mint + parse)
- HS256 JWT helper layer (wrapper around `github.com/golang-jwt/jwt/v5`) used to issue and validate **access tokens**.
- Uses a single signing method: `HS256` (`jwtSigningMethod = jwt.SigningMethodHS256`).

**Mint**
- `MintAccessToken(cfg config.JWTConfig, now time.Time, payload AccessTokenPayload) (string, error)`
  - Validates required JWT config:
    - `cfg.Secret` MUST be set
    - `cfg.Issuer` MUST be set
    - `cfg.ExpirationMinutes` MUST be `> 0`
  - Validates enum payload fields:
    - `payload.Role` MUST be valid (`enums.MemberRole.IsValid()`)
    - `payload.StoreType` MAY be nil; if set, MUST be valid (`enums.StoreType.IsValid()`)
    - `payload.KYCStatus` MAY be nil; if set, MUST be valid (`enums.KYCStatus.IsValid()`)
  - Sets standard registered claims:
    - `iss` = `cfg.Issuer`
    - `iat` = `now`
    - `exp` = `now + cfg.ExpirationMinutes`
  - Signs the token with `cfg.Secret`.

**Parse**
- `ParseAccessToken(cfg config.JWTConfig, tokenString string) (*AccessTokenClaims, error)`
  - Requires `cfg.Secret` to be set.
  - Parses into typed claims struct (`AccessTokenClaims`).
  - Enforces:
    - signing method MUST match HS256 (rejects unexpected `alg`)
    - valid methods limited to HS256 via `jwt.WithValidMethods`
    - issuer validation via `jwt.WithIssuer(cfg.Issuer)`

#### `pkg/auth/claims.go` (typed payload + claims)
- `AccessTokenPayload` (input to minting)
  - `UserID uuid.UUID`
  - `ActiveStoreID *uuid.UUID` (nullable)
  - `Role enums.MemberRole`
  - `StoreType *enums.StoreType` (nullable)
  - `KYCStatus *enums.KYCStatus` (nullable)

- `AccessTokenClaims` (JWT claims returned to callers)
  - `user_id uuid.UUID`
  - `active_store_id *uuid.UUID` (omitempty)
  - `role enums.MemberRole`
  - `store_type *enums.StoreType` (omitempty)
  - `kyc_status *enums.KYCStatus` (omitempty)
  - embeds `jwt.RegisteredClaims` (issuer/iat/exp, etc.)
