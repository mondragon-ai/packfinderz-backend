## Shared conventions
- Authentication: `Authorization: Bearer <token>` is validated by `middleware.Auth`, loads `user_id`, `store_id`, and `role` into context before entering `/api` handlers (api/middleware/auth.go:23-80).
- Store context: `middleware.StoreContext` rejects requests without a store ID once the JWT is validated (api/middleware/store.go:6-16).
- Roles: `middleware.RequireRole("admin"/"agent")` gates `/api/admin` and `/api/agent` ping endpoints (api/middleware/roles.go:1-27).
- Idempotency: `Idempotency-Key` is required for `POST /api/v1/auth/register`, `/api/v1/stores/me/users/invite`, `/api/v1/licenses`, and `/api/v1/media/presign`, with TTL rules defined in `api/middleware/idempotency.go:37-208`.
- Errors: handlers emit `pkg/errors.Code*` metadata so HTTP status and retryability follow `pkg/errors/errors.go:9-100`.

## Health
- `GET /health/live` – unauthenticated liveliness check, returns `{"status":"live"}` and `X-PackFinderz-Env` (api/controllers/health.go:16-21).
- `GET /health/ready` – dependency probe; pings Postgres/Redis/GCS, surfaces failures via `pkg/errors.CodeDependency` details (api/controllers/health.go:23-70).

## Public
- `GET /api/public/ping` – no auth, responds `{"scope":"public","status":"ok"}` (api/controllers/ping.go:10-24).
- `POST /api/public/validate` – validates `name`/`email` payload, echoes sanitized `name`, `email`, and optional `limit` (validators enforce length/format) (api/controllers/validate.go:11-35).

## Auth
- `POST /api/v1/auth/login` – public, body `{"email","password"}` per `auth.LoginRequest`, returns `auth.LoginResponse` plus `X-PF-Token` (auth.Service.Login) for downstream requests (api/controllers/auth.go:13-36; internal/auth/dto.go:9-29).
- `POST /api/v1/auth/register` – public, body `RegisterRequest` (first/last name, email, password, company, store_type, address, accept_tos), reuses `auth.Service` to auto-login, returns 201 with tokens and `X-PF-Token` (api/controllers/register.go:13-41; internal/auth/register.go:21-133).
- `POST /api/v1/auth/logout` – requires Authorization Bearer token, revokes the access session via `session.Manager.Revoke`, returns `{"status":"logged_out"}` (api/controllers/session.go:47-79).
- `POST /api/v1/auth/refresh` – requires Authorization, body `{"refresh_token"}`, rotates session, issues new `AccessToken`/`RefreshToken` plus `X-PF-Token` header (api/controllers/session.go:81-143).
- `POST /api/v1/auth/switch-store` – Authorization plus body `{"store_id","refresh_token"}`, ensures membership, rotates session, returns new tokens and `StoreSummary` (api/controllers/switch_store.go:18-68).

## Private (store-scoped)
- `GET /api/ping` – auth + store context, echoes scope/store_id for health (api/controllers/ping.go:16-24).

## Vendor
- `POST /api/v1/vendor/products` – requires auth, store context, and `Idempotency-Key` (api/middleware/idempotency.go:45-48); body accepts `sku`, `title`, `category`, `unit`, `feelings`, `flavors`, `usage`, inventory quantities, optional `media_ids`, and `volume_discounts`; the controller normalizes enums, validates required fields, and calls `internal/products.Service.CreateProduct`, which ensures the store is a vendor, the caller has one of the allowed store roles, inventory/reserved values make sense, volume discounts have unique `min_qty`, and provided media belong to the same store with `kind=product` before writing the product, inventory, discounts, and media rows in one transaction and returning the created product DTO (api/controllers/products.go:8-206; internal/products/service.go:63-204). Returns `201` on success, `400` for validation failures, `401/403` for auth/role denials, and `409` for conflicts.
- `PATCH /api/v1/vendor/products/{productId}` – requires auth + vendor store context and accepts optional metadata (`sku`, `title`, `subtitle`, `body_html`, `category`, `feelings`, `flavors`, `usage`, `strain`, `classification`, `unit`, `moq`, `price_cents`, `compare_at_price_cents`, `is_active`, `is_featured`, `thc_percent`, `cbd_percent`), plus optional `inventory`, `media_ids`, and `volume_discounts`. Inventory updates must supply both `available_qty` and `reserved_qty` (ints with `reserved_qty ≤ available_qty`), and `media_ids` are deduped while confirming each media record belongs to the same store and has `kind=product`. `controllers.VendorUpdateProduct` normalizes the payload, enforces non-empty trimmed strings, and calls `internal/products.Service.UpdateProduct`, which verifies vendor ownership/roles, ensures unique discount `min_qty`, revalidates the deduped media list, and updates the product, inventory, discounts, and media attachments inside a single transaction before returning the canonical product DTO (api/controllers/products.go:72-205; internal/products/service.go:226-355). Returns `200` on success and `400/401/403/404/409` for validation/auth errors.
- `DELETE /api/v1/vendor/products/{productId}` – requires auth + vendor store context; removes the product row owned by the active store while relying on FK cascades to clean up inventory, discounts, and attached media. `controllers.VendorDeleteProduct` validates the `productId`, store, and user contexts before calling `internal/products.Service.DeleteProduct`, which confirms the store is a vendor, the caller has an allowed role, the product belongs to that store, and then deletes it so inventory, discounts, and product media rows vanish. Returns `204` on success and `400/401/403/404` for canonical failures (api/controllers/products.go:72-244; internal/products/service.go:317-338).

### Stores
- `GET /api/v1/stores/me` – requires active store JWT, returns `stores.StoreDTO` with company, address, owner, KYC, ratings, categories, social links (api/controllers/stores.go:21-48; internal/stores/dto.go:13-105).
- `PUT /api/v1/stores/me` – owner/manager role required, accepts `storeUpdateRequest` (company_name, description, contact, social, banner/logo, ratings, categories), returns updated `StoreDTO` (api/controllers/stores.go:51-124).
- `GET /api/v1/stores/me/users` – owner/manager only, returns `[]memberships.StoreUserDTO` with emails, role/status, last_login (api/controllers/stores.go:126-165; internal/memberships/dto.go:38-76).
- `POST /api/v1/stores/me/users/invite` – owner/manager only, requires `Idempotency-Key`, payload `{"email","first_name","last_name","role"}`, returns invited `StoreUserDTO` plus optional `temporary_password` for new accounts (api/controllers/stores.go:221-302).
- `DELETE /api/v1/stores/me/users/{userId}` – owner/manager only, deletes membership, enforces last-owner guard, returns 200 with empty body (api/controllers/stores.go:168-219).

### Media
- `GET /api/v1/media` – paginated list via optional query `limit`, `kind`, `status`, `mime_type`, `search`, returns `media.ListResult` with signed read URLs for `uploaded`/`ready` items (api/controllers/media.go:134-198; internal/media/list.go:15-139).
- `POST /api/v1/media/presign` – requires `Idempotency-Key`, body `{"media_kind","mime_type","file_name","size_bytes"}`, owner/store check, returns `media.PresignOutput` with `media_id`, `gcs_key`, signed PUT URL, and expiry (api/controllers/media.go:20-91; internal/media/service.go:94-195).
- `DELETE /api/v1/media/{mediaId}` – deletes media if owned, ensures status is deletable, removes GCS object and marks row deleted, returns 204 (api/controllers/media.go:94-132; internal/media/service.go:242-284).

### Licenses
- `POST /api/v1/licenses` – requires `Idempotency-Key`, body includes `media_id`, `issuing_state`, optional dates, `type`, `number`; media must be store-owned, kind `license_doc`, status `uploaded`/`ready`, returns structured license response (api/controllers/licenses.go:21-103; internal/licenses/service.go:51-165).
- `GET /api/v1/licenses` – accepts `limit`/`cursor`, returns `licenses.ListResult` with license metadata + signed GCS download URLs (api/controllers/licenses.go:105-144; internal/licenses/list.go:12-59).

## Admin
- `GET /api/admin/ping` – requires Authorization bearer + role `admin`, share store context if present, no idempotency key required even though idempotency middleware is mounted (api/routes/router.go:64-81; api/controllers/ping.go:26-43).
- `POST /api/v1/admin/licenses/{licenseId}/verify` – admin-only, path parameter parsed as UUID, body `{"decision":"verified|rejected","reason"?}` drives `licenses.Service.VerifyLicense`, which enforces the license is still pending, writes the new status, emits `license_status_changed`, and returns the updated license DTO; invalid decisions or non-pending licenses are rejected with `4xx` errors (api/controllers/licenses.go:233-279; internal/licenses/service.go:382-419).

## Agent
- `GET /api/agent/ping` – requires Authorization + role `agent`, similar to admin ping (api/routes/router.go:83-101; api/controllers/ping.go:36-43).
