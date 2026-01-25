Below is a **fully rewritten, reorganized, and compressed** version of `reusable.md`, with the new `Ratings` type **canonically integrated**.
It is optimized for **LLM consumption**, minimal token load, and strict “single source of truth” usage.

---

# PKG & API Reference (Canonical / Reusable)

> **Purpose**
> Defines **canonical helpers, types, enums, and contracts** reused across the codebase.
>
> If something exists here, **DO NOT re-implement it elsewhere**.

---

## PKG

---

### `config`

Central config via `envconfig`.

**Typed sub-configs**

* App, Service, DB, Redis, JWT, FeatureFlags
* OpenAI, GoogleMaps
* GCP, GCS, Media
* Pub/Sub, Stripe, Sendgrid, Outbox

**Helpers**

* `DBConfig.ensureDSN`

  * Synthesizes legacy vars → `PACKFINDERZ_DB_DSN` if missing.

---

### `db`

GORM + Postgres client.

**Client**

* `DB()`
* `Ping()`
* `Close()`
* Context-aware `Exec` / `Raw`
* `WithTx(fn)` → auto rollback on error/panic

---

### `migrate`

Goose-based migrations.

* `Run`
* `MigrateToVersion`
* Filename + header validation

**Dev Auto-run**

* Enabled when:

  * `PACKFINDERZ_APP_ENV=dev`
  * `PACKFINDERZ_AUTO_MIGRATE=true`

**Generator**

* `create.go` → templated migrations

---

### `redis`

go-redis v9 wrapper.

**Client**

* URL or host-based config
* Pooling + TTL defaults

**Helpers**

* `Set`, `Get`, `SetNX`
* `Incr`, `IncrWithTTL`
* `FixedWindowAllow` (rate limiting)
* Idempotency + rate-limit key builders
* Refresh/session helpers
* `Ping`, `Close`

### `pagination`

Cursor-based limit/cursor helpers reused across list endpoints.

**Constants**

* `DefaultLimit = 25`
* `MaxLimit = 100`

**Types**

* `Params { Limit int; Cursor string }` → embed in API/list DTOs.
* `Cursor { CreatedAt time.Time; ID uuid.UUID }` → canonical cursor payload for rows.

**Functions**

* `NormalizeLimit(limit)` → clamps to `[DefaultLimit, MaxLimit]`.
* `LimitWithBuffer(limit)` → normalized limit + 1 so services can detect a next page.
* `EncodeCursor(Cursor)` / `ParseCursor(string)` → base64 encode/decode the cursor payload.

**Session Keys**

* `AccessSessionKey(accessID string)`
* `Del(ctx, keys...)`

### `cart`

* `internal/cart.Repository` orchestrates `CartRecord` + `CartItem` persistence during checkout staging.
* Methods such as `FindActiveByBuyerStore`, `ReplaceItems`, `UpdateStatus`, and `DeleteByBuyerStore` enforce `buyer_store_id` ownership.
* Stored cart-level discounts map to the `cart_level_discount[]` column via `pkg/types.CartLevelDiscounts`.
* `cart_records` rows mirror `models.CartRecord` (`buyer_store_id`, optional `session_id`, `status cart_status`, `shipping_address`, subtotal/total/fees/total_discount, `cart_level_discount[]`, timestamps) while `cart_items` captures every product snapshot (product/vendor IDs, sku, unit, unit price, optional compare/unit tier data, discounted/subtotal, featured image, moq, thc/cbd) so checkout can replay pricing without recomputing (pkg/db/models/cart_record.go:12-41; pkg/db/models/cart_item.go:11-37; pkg/migrate/migrations/20260124000003_create_cart_records.sql).

---

### `logger`

Structured `zerolog` wrapper.

**Features**

* Level parsing
* Warn-stack
* Output control

**Context Fields**

* `RequestID`
* `UserID`
* `StoreID`
* `ActorRole`

**Helpers**

* `Info`, `Warn`, `Error` (+ optional stack)

---

### `errors`

Canonical typed error system.

**Codes**

* `VALIDATION_ERROR`
* `UNAUTHORIZED`
* `FORBIDDEN`
* `NOT_FOUND`
* `CONFLICT`
* `INTERNAL_ERROR`
* `DEPENDENCY_ERROR`

**Metadata**

* HTTP status
* Retryable flag
* Public message
* Detail visibility

**Builders**

* `New`
* `Wrap`
* `WithDetails`
* `As`

**Mapping**

* `MetadataFor(code)` → **single API mapping source**

---

### `checkout`

Canonical helpers for cart/checkout validation.

**Helpers**

* `ValidateMOQ([]MOQValidationInput)` returns `nil` when every line item meets its product's MOQ and otherwise builds a `pkgerrors.CodeStateConflict` error so the API can reply with HTTP `422`.

**Types**

* `MOQValidationInput` captures `product_id`, optional `product_name`, the stored `moq`, and the requested `quantity`.
* `MOQViolationDetail` surfaces via the envelope's `violations` array (`product_id`, optional `product_name`, `required_qty`, `requested_qty`) so clients can highlight offending products.

**Guarantee**

Reusable, canonical MOQ enforcement for cart and checkout flows; servers and clients can both refer to this helper and the documented error contract when evaluating quantities.

### `visibility`

Shared vendor visibility helpers for buyer product and store queries.

**Helpers**

* `EnsureVendorVisible(VendorVisibilityInput)` enforces `stores.kyc_status=verified`, `subscription_active=true`, and matching `store.address.state` vs. the buyer `state` filter (plus the buyer’s own state when provided) before exposing any vendor data. Violations map to `pkgerrors.CodeNotFound` (hidden vendors) or `pkgerrors.CodeValidation` (state mismatch).

**Types**

* `VendorVisibilityInput` ships the vendor `Store`, the requested `state`, and the buyer store’s state (optional).

**Guarantee**

Applying this helper everywhere keeps buyer-facing product and directory endpoints consistent: hidden vendors always return `404` and state mismatches keep returning `422`, preventing cross-state leaks.

## Shared Types (`pkg/types`)

---

### API Envelopes (MANDATORY)

**SuccessEnvelope**

```json
{ "data": any }
```

**ErrorEnvelope**

```json
{ "error": { "code": string, "message": string, "details"?: any } }
```

Used exclusively by:

* `responses.WriteSuccess*`
* `responses.WriteError`

---

### Postgres Composite Utilities

Reusable helpers for `sql.Scanner` / `driver.Valuer`:

* `quoteCompositeString`
* `quoteCompositeNullable`
* `isCompositeNull`
* `parseComposite`
* `newCompositeNullable`
* `toString`

**Purpose**

* Safe composite encode/decode
* Zero ad-hoc parsing in models

---

### `Address` (`address_t`)

Postgres composite.

**Fields**

* `line1`, `line2?`
* `city`, `state`, `postal_code`
* `country` (default `"US"`)
* `lat`, `lng`, `geohash?`

**Implements**

* `driver.Valuer`
* `sql.Scanner`

---

### `Social` (`social_t`)

Postgres composite.

**Optional**

* `twitter`, `facebook`, `instagram`
* `linkedin`, `youtube`, `website`

**Implements**

* `driver.Valuer`
* `sql.Scanner`

---

### `GeographyPoint`

PostGIS `geography(POINT, 4326)`.

**Fields**

* `lat`, `lng`

**Implements**

* `driver.Valuer` → `SRID=4326;POINT(lng lat)`
* `sql.Scanner` (WKT / EWKT / WKB)

---

### `Ratings` (JSONB)

Flexible scoring map stored as JSONB.

```go
type Ratings map[string]int
```

**Behavior**

* `nil` → `{}` on write
* Supports `string` or `[]byte` scan
* Strict type validation

**Implements**

* `driver.Valuer`
* `sql.Scanner`

**Usage**

* Product/store ratings
* Arbitrary scoring dimensions
* Avoids schema churn

---

### `CartLevelDiscounts` (`cart_level_discount[]`)

* Represents the Postgres composite array attached to `cart_records.cart_level_discount`.
* `pkg/types.CartLevelDiscounts` implements `driver.Valuer`/`sql.Scanner` and validates required fields (`title`, `id`, `value`, `value_type`, `vendor_id`).
* The cart repository writes/reads this element when persisting buyer snapshots via `internal/cart.Repository`.

## Security

---

### `pkg/security/password`

Argon2id hashing.

**Format**

```
$argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>
```

**Helpers**

* `HashPassword`
* `VerifyPassword`

**Guarantees**

* Constant-time compare
* Safe parameter bounds

**Errors**

* `ErrInvalidHash`

---

## Enums (`pkg/enums/*`)

> Canonical string enums across DTOs, DB, auth, validation.

All enums implement:

* `String()`
* `IsValid()`
* `ParseX(string)`

---

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

### `Outbox`

* `event_type_enum`: `order_created`, `license_status_changed`, `media_uploaded`, `notification_requested`, `ad_*`, etc.
* `aggregate_type_enum`: `vendor_order`, `checkout_group`, `license`, `store`, `media`, `ledger_event`, `notification`, `ad`.
* Helpers: `OutboxEventType`/`OutboxAggregateType` in `pkg/enums/outbox.go`.
* Outbox payload envelope struct and actor ref definitions live under `pkg/outbox/envelope.go`.
* Repository/service/registry infrastructure lives under `pkg/outbox` (see `repository.go`, `service.go`, `registry.go`).
* Idempotency manager: `pkg/eventing/idempotency.Manager` wraps Redis `SETNX` with the `pf:evt:processed:<consumer>:<event_id>` key pattern and respects `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (default `720h`) so consumers skip duplicate deliveries before applying side effects.
* Publisher worker: `cmd/outbox-publisher` fetches batches with `FOR UPDATE SKIP LOCKED`, publishes to `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`, and marks `published_at` or increments `attempt_count`/`last_error` (bounded length) before committing; see `docs/outbox.md` for operational expectations.
* Config knobs: `PACKFINDERZ_OUTBOX_PUBLISH_BATCH_SIZE` (default `50`), `PACKFINDERZ_OUTBOX_PUBLISH_POLL_MS` (default `500`), `PACKFINDERZ_OUTBOX_MAX_ATTEMPTS` (default `25`), the domain topic via `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`, and the compliance subscription via `PACKFINDERZ_PUBSUB_DOMAIN_SUBSCRIPTION` where `license_status_changed` consumers run.
* `license_status_changed` events are emitted by `internal/licenses/service` whenever a license is created or an admin decision lands, and `emitLicenseStatusEvent` runs inside the same transaction as the license mutation so the downstream worker sees consistent state (`internal/licenses/service.go`:136-419).
* `internal/notifications/consumer` (set up in `cmd/worker/main`) subscribes to the domain topic, uses `pkg/outbox/idempotency.Manager` to honor the `pf:evt:processed:<consumer>:<event_id>` TTL, and writes `NotificationTypeCompliance` rows with links and rejection details for admins/stores based on the status in the event payload (internal/notifications/consumer.go:18-186; cmd/worker/main.go:83-116).

### `NotificationType`

* `system_announcement`
* `market_update`
* `security_alert`
* `order_alert`
* `compliance`

### `CartStatus`

* `active`
* `converted`

### `Product`

* `category`: `flower`, `cart`, `pre_roll`, `edible`, `concentrate`, `beverage`, `vape`, `topical`, `tincture`, `seed`, `seedling`, `accessory`
* `classification`: `sativa`, `hybrid`, `indica`, `cbd`, `hemp`, `balanced`
* `unit`: `unit`, `gram`, `ounce`, `pound`, `eighth`, `sixteenth`
* `flavors`: `earthy`, `citrus`, `fruity`, `floral`, `cheese`, `diesel`, `spicy`, `sweet`, `pine`, `herbal`
* `feelings`: `relaxed`, `happy`, `euphoric`, `focused`, `hungry`, `talkative`, `creative`, `sleepy`, `uplifted`, `calm`
* `usage`: `stress_relief`, `pain_relief`, `sleep`, `depression`, `muscle_relaxant`, `nausea`, `anxiety`, `appetite_stimulation`

---

### `internal/licenses`

* `Service` exposes `CreateLicense`, `ListLicenses`, and the new `DeleteLicense` (owner/manager only, expired/rejected rows only, rewrites `stores.kyc_status` to `pending_verification` when no `verified` licenses remain).
* Repository wiring now includes `FindByID`, `Delete`, and `CountValidLicenses` so services can enforce store ownership and compute the `verified` remainder.
* `controllers.LicenseDelete` (registered under `DELETE /api/v1/licenses/{licenseId}`) parses docs/UUID, relies on the same middleware-based context, and returns the canonical success error envelope.
* `Service.VerifyLicense` plus `controllers.AdminLicenseVerify` implemented the admin-only `/api/v1/admin/licenses/{licenseId}/verify` route, validating `verified|rejected` decisions, Idempotency-buffered requests, and conflict handling for non-pending licenses.
* Approvals/rejections now recompute `stores.kyc_status` in the same transaction by reviewing every license for the store and using `DetermineStoreKYCStatus` (internal/licenses/service.go:385-425) so the mirror flips to `verified`, `rejected`, or `expired` only when the aggregated outcome changes.

### `internal/notifications`
* `Repository.Create` inserts compliance notifications so the worker can persist alerts after consuming events (internal/notifications/repo.go:1-23).
* `Consumer` subscribes to `license_status_changed` events, honors `pkg/outbox/idempotency.Manager` TTLs, and writes `NotificationTypeCompliance` rows with the right admin/store link plus rejection details when present, keeping the event tied to the originating store (internal/notifications/consumer.go:18-186; cmd/worker/main.go:83-116).

### `internal/cart`
* `Repository` secures `CartRecord` + `CartItem` persistence by scoping every operation to the owning `buyer_store_id`.
* `ReplaceItems` wipes the previous `cart_items` rows before inserting the new snapshot, while `UpdateStatus` flips the record from `active` to `converted`.
* Cart-level discounts map through `pkg/types.CartLevelDiscounts` when the repository writes/reads `cart_level_discount[]`.
* `Service.UpsertCart` enforces buyer KYC/role, vendor visibility (verified/subscribed/in-state), inventory availability, MOQ, volume-tier pricing, subtotal/total math, and cart-level discount metadata before the cart is created or updated so the returned record is the canonical checkout snapshot (`internal/cart/service.go:39-209`).

### `internal/products`
* `Repository` exposes product CRUD plus detail/list reads that preload `Inventory`, `VolumeDiscounts` (descending `min_qty`), and `Media` (ascending `position`) so services get a single `Product` model with the related SKU, pricing, inventory, discounts, and ordered media (internal/products/repo/repository.go:60-208).
* `UpsertInventory`/`GetInventoryByProductID` respect the 1:1 `inventory_items.product_id PK` row, while `CreateVolumeDiscount`/`ListVolumeDiscounts`/`DeleteVolumeDiscount` keep the `(product_id,min_qty)` uniqueness and descending salary order for tiered pricing lookups (internal/products/repo/repository.go:133-175).
* `VendorSummary` is built via `vendorSummaryQuery`, joining `stores` to the latest `media_attachments` logo row and returning `StoreID`, `CompanyName`, and nullable `LogoMediaID`/`LogoGCSKey` for service-layer URL signing (internal/products/repo/repository.go:34-208).
* `service` enforces vendor store type, allowed user roles, `reserved_qty <= available_qty`, unique `min_qty` per discount, and that each requested media belongs to the store with `kind=product`; product, inventory, discounts, and product media are saved inside a single transaction before `NewProductDTO` returns the created record with the preloaded vendor summary (internal/products/service.go:63-204).
* `service.DeleteProduct` reuses the same ownership + role guards, fetches the product to ensure it belongs to the active vendor, then deletes the row so FK cascades remove inventory, discounts, and media attachments while the route returns `204` (internal/products/service.go:317-338).
* `service.UpdateProduct` applies the optional changes via `applyUpdateToProduct`, synchronously replaces inventory rows, volume discounts, and media attachments (via `buildProductMediaRows`), defends against duplicate discounts/media IDs, enforces that each media belongs to the active store with `kind=product`, revalidates ownership/roles, and returns the updated `ProductDTO` so the PATCH endpoint surfaces the same canonical payload as creation (internal/products/service.go:226-355).

### `internal/schedulers/licenses`
* Scheduler runs every 24h from `cmd/worker`, warning stores 14 days before a license's `expiration_date` and expiring licenses on their due date (`internal/schedulers/licenses/service.go`:1-220).
* `warnExpiring`/`expireLicenses` each execute inside `WithTx`, emit `license_status_changed` outbox events (warnings include the `expires on` message plus `warningType=expiry_warning`), and `expireLicense` reloads the license, skips already expired rows, updates the status, emits the event, and reconciles `stores.kyc_status` via `DetermineStoreKYCStatus` (`internal/schedulers/licenses/service.go`:61-210; internal/licenses/service.go:405-416).

## Auth (Canonical)

---

### `pkg/auth/token`

HS256 access tokens only.

**Helpers**

* `MintAccessToken`
* `ParseAccessToken`

**Enforced**

* Issuer
* Expiry
* Signing algorithm

---

### `pkg/auth/claims`

**AccessTokenPayload**

* `user_id`
* `active_store_id?`
* `role`
* `store_type?`
* `kyc_status?`

**AccessTokenClaims**

* Typed claims
* Embeds `jwt.RegisteredClaims`

---

### `pkg/auth/session`

Redis-backed refresh sessions.

**Refresh Tokens**

* Cryptographically random
* base64url encoded

**Errors**

* `ErrInvalidRefreshToken`
  (not found, expired, mismatched)

**Manager**

* `Generate(accessID)`
* `Rotate(oldAccessID, refreshToken)`
* `HasSession(accessID)`

**Guarantees**

* Refresh TTL > access TTL
* Constant-time compare
* Single-use rotation

**AccessID**

* UUID string
* Used as:

  * JWT `jti`
  * Redis key suffix

---

## API

---

### Routes

* `/health`
* `/api/v1/auth/login`
* `/api/public/*`
* `/api/*` (auth)
* `/api/admin/*`
* `/api/agent/*`

---

### Middleware

* `Recoverer`
* `RequestID`
* `Logging`
* `Auth` (JWT + Redis session)
* `StoreContext`
* `RequireRole`
* `Idempotency` (placeholder)
* `RateLimit` (placeholder)

---

### Responses

**ALL responses MUST**

* Use envelopes
* Map errors via:

  * `pkg/errors`
  * `pkg/logger`

---

**If it’s not here, it’s not canonical.**
