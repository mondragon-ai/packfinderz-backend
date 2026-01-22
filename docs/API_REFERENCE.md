# API Reference

All controllers return responses wrapped in the canonical envelopes defined in `pkg/types`:

```json
{
  "data": { /* success payload */ }
}
```

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "validation failed",
    "details": { /* present only when allowed */ }
  }
}
```

`details` is only emitted when the error metadata allows it (e.g., validation, state conflicts, dependency failures). Unless otherwise noted, every guarded route also requires `Authorization: Bearer <access_token>` and `Idempotency-Key` when the middleware enforces it. Successful auth, refresh, and switch-store responses additionally emit an `X-PF-Token` header with the newly minted access token; health endpoints set `X-PackFinderz-Env`.

---

## Health

### `GET /health/live`
- **Purpose:** lightweight liveness check.
- **Request body:** none.
- **Response:** `200` with `{"data":{"status":"live"}}`.
- **Failure:** none beyond infrastructure (unreachable server).

### `GET /health/ready`
- **Purpose:** verifies Postgres, Redis, and GCS dependencies before accepting traffic.
- **Request body:** none.
- **Success response:** `200` with `{"data":{"status":"ready"}}`.
- **Failure:** `503 DEPENDENCY_ERROR` with `details` containing the dependency map, e.g., `{"postgres":"not configured","redis":"dial tcp ...","gcs":"not configured"}`.

---

## Public `/api/public`

### `GET /api/public/ping`
- **Purpose:** quick connectivity test without authentication.
- **Response:** `200`  
  ```json
  {
    "data": {
      "scope": "public",
      "status": "ok"
    }
  }
  ```
- **Failures:** none beyond infrastructure/encoding; validation does not apply.

### `POST /api/public/validate`
- **Purpose:** validate a name/email pair plus optional list limit.
- **Request body:**
  ```json
  {
    "name": "string, 3-64 chars",
    "email": "valid email"
  }
  ```
- **Query parameters:**
  - `limit` (integer): defaults to `10`, clamped between `1` and `100`.
- **Success response:** `200` with sanitized values:
  ```json
  {
    "data": {
      "name": "sanitized name",
      "email": "lower-cased/trimmed email",
      "limit": 10
    }
  }
  ```
- **Failures:**
  - `400 VALIDATION_ERROR` if JSON is invalid/missing required fields or `limit` outside bounds.

---

## Auth `/api/v1/auth`

### `POST /api/v1/auth/login`
- **Request body:**
  ```json
  {
    "email": "required, email",
    "password": "required"
  }
  ```
- **Success response:** `200` with `auth.LoginResponse`:
  ```json
  {
    "data": {
      "access_token": "...",
      "refresh_token": "...",
      "stores": [
        {
          "id": "...",
          "name": "...",
          "type": "buyer|vendor",
          "logo_url": "optional"
        }
      ],
      "user": {
        "id": "...",
        "email": "...",
        "first_name": "...",
        "last_name": "...",
        "phone": "optional",
        "is_active": true,
        "last_login_at": "optional",
        "system_role": "optional",
        "store_ids": ["..."],
        "created_at": "...",
        "updated_at": "..."
      }
    }
  }
  ```
- **Headers:** `X-PF-Token` carries the access token (same as `data.access_token`).
- **Failures:**
  - `400 VALIDATION_ERROR` for malformed JSON.
  - `401 UNAUTHORIZED` for missing/invalid credentials or inactive user.
  - `429 RATE_LIMIT_EXCEEDED` when the login rate limiter (per `middleware.AuthRateLimit`) is tripped.
  - `500 INTERNAL_ERROR` or `503 DEPENDENCY_ERROR` for service failures.

### `POST /api/v1/auth/register`
- **Idempotent:** yes (`Idempotency-Key` required, 24h TTL).
- **Request body:** `auth.RegisterRequest` (see [`internal/auth/register.go`](internal/auth/register.go)) – required fields include `first_name`, `last_name`, `email`, `password`, `company_name`, `store_type` (`buyer|vendor`), `address` (line1, city, state, postal_code, lat, lng), and `accept_tos` (must be `true`).
- **Success response:** `201` with the same payload as login (`auth.LoginResponse`) and `X-PF-Token`.
- **Failures:**
  - `400 VALIDATION_ERROR` for missing/invalid fields, `accept_tos` false, or duplicate email payloads.
  - `409 CONFLICT` if the email is already registered.
  - `429 RATE_LIMIT_EXCEEDED` via auth rate limiter.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR` for repo/session failures.

### `POST /api/v1/auth/logout`
- **Headers:** `Authorization: Bearer <access_token>`.
- **Success response:** `200` with `{"data":{"status":"logged_out"}}`.
- **Failures:**
  - `401 UNAUTHORIZED` if the `Authorization` header is missing, malformed, or belongs to a rotated session.
  - `500 INTERNAL_ERROR` when revoking the session fails.

### `POST /api/v1/auth/refresh`
- **Headers:** `Authorization: Bearer <access_token>` (can be expired).
- **Request body:**
  ```json
  {
    "refresh_token": "required"
  }
  ```
- **Success response:** `200` with `{"access_token","refresh_token"}` and `X-PF-Token`.
- **Failures:**
  - `400 VALIDATION_ERROR` for missing body.
  - `401 UNAUTHORIZED` for invalid/missing refresh token or session ID.
  - `500 INTERNAL_ERROR` for rotation/minting failures.

### `POST /api/v1/auth/switch-store`
- **Headers:** `Authorization: Bearer <access_token>` (current store).
- **Request body:**
  ```json
  {
    "store_id": "required uuid of target store",
    "refresh_token": "required"
  }
  ```
- **Success response:** `200` with `SwitchStoreResult`:
  ```json
  {
    "data": {
      "access_token": "...",
      "refresh_token": "...",
      "store": {
        "id": "...",
        "name": "...",
        "type": "buyer|vendor",
        "logo_url": "optional"
      }
    }
  }
  ```
- **Failures:**
  - `400 VALIDATION_ERROR` for invalid UUID or missing fields.
  - `401 UNAUTHORIZED` for invalid tokens or refresh tokens.
  - `403 FORBIDDEN` when the user lacks membership or the membership is inactive.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR` for repository/session failures.

---

## Private `/api`

All endpoints under `/api` require `Authorization` with an access token containing an `activeStoreId`. The middleware stack enforces store context, idempotency where configured, and rate limiting.

### `GET /api/ping`
- **Purpose:** confirm authenticated connectivity.
- **Response:** `200` with `{"scope":"private","status":"ok","store_id": "<active store>"}`.
- **Failures:** `401 UNAUTHORIZED` if the token is missing/invalid.

---

## Store Management `/api/v1/stores`

Endpoints require store-scoped JWTs, and owners/managers are authorized automatically via the service layer.

### `GET /api/v1/stores/me`
- **Response:** `200` with `stores.StoreDTO` – includes fields such as:
  - `id`, `type` (`buyer|vendor`), `company_name`, `dba_name`, `description`, `phone`, `email`, `kyc_status`, `subscription_active`, `delivery_radius_meters`.
  - `address` (line1/line2/city/state/postal_code/country/lat/lng), `geom` (latitude/longitude), optional `social`, `banner_url`, `logo_url`, `ratings`, `categories`, `owner`, `last_active_at`, `created_at`, `updated_at`.
- **Failures:**
  - `401 UNAUTHORIZED`: missing/invalid authentication.
  - `403 FORBIDDEN`: missing store context.
  - `404 NOT_FOUND`: store was deleted.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR`: repo failures.

### `PUT /api/v1/stores/me`
- **Request body:** `storeUpdateRequest` – every field is optional:
  - `company_name`, `description`, `phone`, `email`, `social`, `banner_url`, `logo_url`, `ratings`, `categories`.
- **Success response:** updated `stores.StoreDTO`.
- **Failures:**
  - `400 VALIDATION_ERROR` for malformed JSON or invalid nested values (`email`, `social`, etc.).
  - `403 FORBIDDEN` when the caller lacks owner/manager role.
  - `404 NOT_FOUND` if the store record vanished mid-flight.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR` for persistence issues.

### `GET /api/v1/stores/me/users`
- **Response:** `200` with an array of `memberships.StoreUserDTO`:
  - Each item includes `membership_id`, `store_id`, `user_id`, `email`, `first_name`, `last_name`, `role`, `membership_status`, `created_at`, and optional `last_login_at`.
- **Failures:** same as `PUT /stores/me` (validation, forbidden, etc.).

### `POST /api/v1/stores/me/users/invite`
- **Idempotent:** requires `Idempotency-Key` (24h TTL).
- **Request body:**
  ```json
  {
    "email": "required, valid email",
    "first_name": "required",
    "last_name": "required",
    "role": "owner|admin|manager|viewer|agent|staff|ops"
  }
  ```
- **Success response:** `200` with:
  ```json
  {
    "data": {
      "user": { /* memberships.StoreUserDTO */ },
      "temporary_password": "optional string when a new user / reset password was issued"
    }
  }
  ```
  `temporary_password` is omitted if the target user already had an active password.
- **Failures:**
  - `400 VALIDATION_ERROR` for malformed payload or invalid role/email.
  - `403 FORBIDDEN` for insufficient role in the caller store.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR` when creating/updating users or memberships fails.

### `DELETE /api/v1/stores/me/users/{userId}`
- **Path parameter:** `userId` must be a UUID.
- **Success response:** `200` with `{"data":null}`.
- **Failures:**
  - `400 VALIDATION_ERROR` when `userId` is missing/invalid.
  - `403 FORBIDDEN` for insufficient role.
  - `404 NOT_FOUND` if the membership does not exist.
  - `409 CONFLICT` when attempting to remove the last owner.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR` if deletion fails.

---

## Media `/api/v1/media`

### `POST /api/v1/media/presign`
- **Idempotent:** requires `Idempotency-Key` (24h TTL).
- **Request body:**
  ```json
  {
    "media_kind": "product|ads|pdf|license_doc|coa|manifest|user|other",
    "mime_type": "required, must match allowed mime types for the kind",
    "file_name": "required",
    "size_bytes": "required, 1 <= size <= 20 MiB"
  }
  ```
- **Success response:** `200` with `media.PresignOutput`:
  - `media_id`, `gcs_key`, `signed_put_url`, `content_type`, `expires_at`.
  - The signed URL expires after the configured upload TTL.
- **Failures:**
  - `400 VALIDATION_ERROR` for missing/malformed fields or unsupported `mime_type`.
  - `403 FORBIDDEN` when the caller lacks ownership/staff rights for the store.
  - `500 INTERNAL_ERROR` / `503 DEPENDENCY_ERROR` when persisting the media row or signing the URL fails.
  - `409 IDEMPOTENCY_KEY_REUSED` if the same `Idempotency-Key` is replayed with a different body.

---

## Admin & Agent `/api/{admin,agent}`

Both groups proxy the same ping semantics while enforcing `admin` or `agent` roles via `middleware.RequireRole`.

### `GET /api/admin/ping`
- **Response:** `200` with `{"data":{"scope":"admin","status":"ok","store_id":"..."}}`.
- **Failures:** `401 UNAUTHORIZED` for missing tokens; `403 FORBIDDEN` when the role check fails.

### `GET /api/agent/ping`
- **Response:** `200` with `{"data":{"scope":"agent","status":"ok","store_id":"..."}}`.
- **Failures:** same as admin ping.

