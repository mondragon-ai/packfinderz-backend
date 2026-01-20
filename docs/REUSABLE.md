
# PKG & API Reference (Canonical / Reusable)

> **Purpose**
> This document defines **canonical helpers, types, enums, and contracts** that MUST be reused across the codebase.
> It exists to prevent duplication and drift when generating new code (services, handlers, middleware, DTOs).
>  
> If something is defined here, **do not re-implement it elsewhere**.

---

## PKG

### `config`
- Central `Config` struct driven by `envconfig`.
- Typed sub-configs:
  - App, Service, DB, Redis, JWT, FeatureFlags
  - OpenAI, GoogleMaps
  - GCP, GCS, Media
  - Pub/Sub, Stripe, Sendgrid
- `DBConfig.ensureDSN`
  - Synthesizes legacy host/user/password vars into `PACKFINDERZ_DB_DSN` when missing.

---

### `db`
- `Client`
  - Boots GORM with Postgres driver.
  - Exposes:
    - `DB()`
    - `Ping()`
    - `Close()`
    - context-aware `Exec` / `Raw`
    - `WithTx(fn)` that auto-rolls back on errors or panics.

---

### `migrate`
- Goose-based migration helpers:
  - `Run`, `MigrateToVersion`
  - Migration validation (filename + header).
- `autorun.go`
  - Dev-only auto migration when:
    - `PACKFINDERZ_APP_ENV=dev`
    - `PACKFINDERZ_AUTO_MIGRATE=true`
- `create.go`
  - Templated migration generation.

---

### `redis`
- `Client`
  - go-redis v9 wrapper.
  - Handles URL vs host config, pooling, TTL defaults.
- Common helpers:
  - `Set`, `Get`, `SetNX`
  - `Incr`, `IncrWithTTL`
  - `FixedWindowAllow` (rate limiting)
  - Idempotency + rate-limit key builders
  - Refresh/session token helpers
  - `Ping`, `Close`

#### Redis session helpers
- `AccessSessionKey(accessID string) string`
  - Builds namespaced Redis key:
  - `buildKey(sessionPrefix, "access", accessID)`
- `Del(ctx, keys...)`
  - Deletes one or more keys.
  - Errors if client not initialized.

---

### `logger`
- Structured `zerolog` wrapper.
- Options:
  - log level
  - warn-stack
  - output destination
- Helpers:
  - `ParseLevel`
  - Context enrichment:
    - `RequestID`
    - `UserID`
    - `StoreID`
    - `ActorRole`
  - `Info / Warn / Error` with optional stack traces.

---

### `errors`
- Canonical typed error system.
- `Code` enum:
  - `VALIDATION_ERROR`
  - `UNAUTHORIZED`
  - `FORBIDDEN`
  - `NOT_FOUND`
  - `CONFLICT`
  - `INTERNAL_ERROR`
  - `DEPENDENCY_ERROR`
- `Metadata`
  - HTTP status
  - retryable flag
  - public message
  - details allowed
- `Error` builder:
  - `New`
  - `Wrap`
  - `WithDetails`
  - `As`
- `MetadataFor(code)`
  - **Single source of truth** for API error mapping.

---

## Shared Helpers / Canonical Types

### `pkg/types`: API envelopes
- **SuccessEnvelope**
  ```json
  { "data": any }
```

* **ErrorEnvelope**

  ```json
  { "error": { "code": string, "message": string, "details"?: any } }
  ```
* Used exclusively by:

  * `responses.WriteSuccess*`
  * `responses.WriteError`

---

### `pkg/types`: Postgres composite helpers

Reusable utilities for `sql.Scanner` / `driver.Valuer` implementations.

* `quoteCompositeString(string)`
* `quoteCompositeNullable(*string)`
* `isCompositeNull(string)`
* `parseComposite(raw string, expected int)`
* `newCompositeNullable(string)`
* `toString(any)`

**Purpose**

* Safely encode/decode Postgres composite types.
* Prevents ad-hoc parsing logic across models.

---

### `pkg/types`: `Address` (`address_t`)

* Fields:

  * `line1`, `line2?`, `city`, `state`, `postal_code`, `country`
  * `lat`, `lng`, `geohash?`
* Behavior:

  * Required: `line1`, `city`, `state`, `postal_code`, `lat`, `lng`
  * Default `country = "US"`
* Implements:

  * `driver.Valuer`
  * `sql.Scanner`

---

### `pkg/types`: `Social` (`social_t`)

* Optional fields:

  * `twitter`, `facebook`, `instagram`, `linkedin`, `youtube`, `website`
* Implements:

  * `driver.Valuer`
  * `sql.Scanner`

---

### `pkg/types`: `GeographyPoint`

* Represents PostGIS `geography(POINT, 4326)`
* Fields: `{ lat, lng }`
* Implements:

  * `driver.Valuer` → EWKT (`SRID=4326;POINT(lng lat)`)
  * `sql.Scanner`

    * WKT / EWKT
    * WKB bytes

---

## Security

### `pkg/security/password`

* Argon2id password hashing.
* Config-driven parameters.
* Hash format:

  ```
  $argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>
  ```
* Helpers:

  * `HashPassword`
  * `VerifyPassword`
* Guarantees:

  * Constant-time comparison
  * Parameter clamping (safe bounds)
* Error:

  * `ErrInvalidHash`

---

## Enums (`pkg/enums/*`)

> Canonical string enums used across DTOs, DB models, auth, and validation.

All enums provide:

* `String()`
* `IsValid()`
* `ParseX(value string)`

### `StoreType`

* `buyer`
* `vendor`

### `KYCStatus`

* `pending_verification`
* `verified`
* `rejected`
* `expired`
* `suspended`

### `MembershipStatus`

* `invited`
* `active`
* `removed`
* `pending`

### `MemberRole`

* `owner`
* `admin`
* `manager`
* `viewer`
* `agent`
* `staff`
* `ops`

---

## Auth (Canonical)

### `pkg/auth/token`

* HS256 JWT access tokens only.
* Single signing method enforced.
* Helpers:

  * `MintAccessToken`
  * `ParseAccessToken`
* Enforced:

  * issuer
  * expiry
  * signing algorithm
* Payload validation uses canonical enums.

---

### `pkg/auth/claims`

* `AccessTokenPayload`

  * `user_id`
  * `active_store_id?`
  * `role`
  * `store_type?`
  * `kyc_status?`
* `AccessTokenClaims`

  * Typed JWT claims returned to callers.
  * Embeds `jwt.RegisteredClaims`.

---

### `pkg/auth/session`

**Refresh-session system (Redis-backed)**

* Refresh tokens:

  * cryptographically random
  * base64url encoded
* Errors:

  * `ErrInvalidRefreshToken` (used consistently for “not found”, expired, or mismatched tokens)

**Manager**

* `Generate(accessID)`
* `Rotate(oldAccessID, refreshToken)`
* `HasSession(accessID)` (used by middleware)
* Guarantees:

  * refresh TTL > access TTL
  * constant-time token comparison
  * single-use rotation semantics

**AccessID**

* UUID string.
* Used as:

  * JWT `jti`
  * Redis session key suffix.

---

## API (High-Level)

### Routes

* `/health`
* `/api/v1/auth/login`
* `/api/public/*`
* `/api/*` (auth required)
* `/api/admin/*`
* `/api/agent/*`

---

### Middleware

* `Recoverer`
* `RequestID`
* `Logging`
* `Auth`

  * Uses `pkg/auth`
  * Verifies Redis session via `pkg/auth/session`
* `StoreContext`
* `RequireRole`
* `Idempotency` (placeholder)
* `RateLimit` (placeholder)

---

### Responses

* **ALL responses MUST use envelopes**

  * `SuccessEnvelope`
  * `ErrorEnvelope`
* Error mapping MUST flow through:

  * `pkg/errors`
  * `pkg/logger`

---

**If it’s not here, it’s not canonical.**
