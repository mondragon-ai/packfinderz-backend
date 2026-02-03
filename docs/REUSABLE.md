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

**StripeConfig**

* Loads `PACKFINDERZ_STRIPE_API_KEY`, `PACKFINDERZ_STRIPE_SECRET`, and `PACKFINDERZ_STRIPE_ENV` (default `test`).
* `cfg.Environment()` normalizes to `test|live`, and `pkg/stripe.NewClient` enforces the matching `sk_*`/`rk_*` prefix so misconfigured keys fail fast.
* The signing secret stays available for webhook verification while the API key bootstraps the Stripe client used by both the API and worker binaries.
* `internal/webhooks/stripe.Service` consumes `/api/v1/webhooks/stripe`, verifies the `Stripe-Signature` header, deduplicates deliveries via a Redis guard (key pattern `pf:idempotency:stripe-webhook:<event_id>` with TTL `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL`), and mirrors `customer.subscription.*`/`invoice.paid|invoice.payment_failed` events into `subscriptions.status` plus `stores.subscription_active`.
* `cmd/api/main.go` and `cmd/worker/main.go` both call `pkg/stripe.NewClient` during startup and exit immediately when the client returns an error, ensuring missing or invalid Stripe keys block API/worker bootstrapping (`cmd/api/main.go:55-65`; `cmd/worker/main.go:51-70`).

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

### `bigquery`

Reusable BigQuery bootstrap + readiness guard.

**Client**

* `NewClient(ctx, config.GCPConfig, config.BigQueryConfig, logger)` (credentials via JSON/file, dataset + table validation, log the initialization).
* `InsertRows(ctx, table, rows)` uses the configured dataset and accepts `[]any` so ingestion helpers can send maps, ValueSaver structs, or `bigquery.ValuesSaver`.
* `Query(ctx, sql, params)` returns a `*bigquery.RowIterator` so analytics helpers can run parameterized queries without touching the raw SDK.

**Readiness**

* `Ping(ctx)` re-checks the configured dataset + tables so `/health/ready` and `cmd/worker` dependency pings fail fast when `marketplace_events` or `ad_events` are missing.
* Configured via `PACKFINDERZ_BIGQUERY_DATASET` (default `packfinderz`), `PACKFINDERZ_BIGQUERY_MARKETPLACE_TABLE`, and `PACKFINDERZ_BIGQUERY_AD_TABLE`.

**Writer**

* `internal/analytics/writer.BigQueryWriter` builds on this client, exposes `EncodeJSON` for JSON columns, and adds retry/backoff plus optional batching before emitting `marketplace_events` and `ad_event_facts` rows.

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

* `internal/cart.Repository` orchestrates `CartRecord`, `CartItem`, and the new `CartVendorGroup` persistence shapes during checkout staging.
* Methods such as `FindActiveByBuyerStore`, `ReplaceItems`, `UpdateStatus`, and `DeleteByBuyerStore` enforce `buyer_store_id` ownership.
* PF-147 keeps the GORM models in lockstep with `pkg/migrate/migrations/20260306000000_cart_modifications.sql`: `cart_records` now include `buyer_store_id`, `checkout_group_id`, `status cart_status`, `currency default 'USD'`, `valid_until`, `subtotal_cents`, `discounts_cents`, `total_cents`, `ad_tokens text[]`, and timestamps so totals, attributions, and conversion refs live on the parent record while PF-171 adds `payment_method`, `shipping_line`, and `converted_at` so the checkout selections and transition timestamp are also auditable.
* `cart_items` persists the vendor-level snapshot (`product_id`, `vendor_store_id`, `quantity`, `moq`, optional `max_qty`, `unit_price_cents`, `applied_volume_discount jsonb`, `line_subtotal_cents`, `status cart_item_status`, `warnings jsonb`, `created_at`, `updated_at`) to capture inventory rules, authoritative pricing, and warning metadata; renamed columns and enums now mirror the migration, so the upcoming models must match exactly.
* `cart_vendor_groups` (new table) holds vendor aggregates (`cart_id`, `vendor_store_id`, `status vendor_group_status`, `warnings jsonb`, `subtotal_cents`, `promo jsonb`, `total_cents`) with a `(cart_id, vendor_store_id)` unique constraint for auditing vendor-level decisions, and the future `CartVendorGroup` model tracks the same JSONB promo/warnings payloads.
* `GET /api/v1/cart` fetches the active `cart_record` (plus line items and vendor groups) for the buyer store so the UI can recover an in-progress checkout without mutating state.

### `orders`

* `internal/orders.Repository` persists `vendor_orders`, `order_line_items`, and `payment_intents` so checkout execution can materialize the per-vendor snapshot; the canonical `checkout_group_id` now lives on `cart_records` and `vendor_orders` directly (the standalone `checkout_groups` table was dropped), but it still ties every vendor order/line item to the same cart. Each payment intent is built with the checkout-selected payment method and that vendor order’s total so payment tracking stays vendor-scoped.
* Methods preload `VendorOrders.Items` + `PaymentIntent` to keep the in-memory checkout snapshot consistent while fetching by checkout group or order.
* `ListBuyerOrders` exposes cursor pagination via `pkg/pagination`, returning `BuyerOrderList` with `order_number`, totals, `total_items`, payment/fulfillment/shipping statuses, vendor summary, and `next_cursor`.
* `BuyerOrderFilters` govern the list: `order_status`, `fulfillment_status`, `shipping_status`, `payment_status`, `date_from/date_to`, and a normalized `q` search across buyer/vendor names. `vendor_orders` now store `fulfillment_status`, `shipping_status`, and a sequential `order_number` backed by matching enums/indexes.
* `ListVendorOrders` mirrors the buyer list for vendor stores: cursor pagination, `VendorOrderFilters` covering `order_status`, `fulfillment_status`, `shipping_status`, `payment_status`, `date_from/date_to`, `actionable_state` sets, plus normalized search that spans the buyer or vendor names while returning buyer summary info.
* `FindOrderDetail` pulls a single `vendor_orders` row with `order_line_items`, `payment_intent`, buyer/vendor summaries, and the active `order_assignments` row (null when no agent assigned) so controllers can render detail views without extra queries.

### `checkout`

* `internal/checkout/helpers` contains deterministic, database-free logic that groups `CartItem`s by `vendor_store_id` and validates buyer/vendor eligibility (store type, subscription, state) plus MOQ compliance before executing checkout; the persisted `cart_vendor_groups` snapshot now supplies the authoritative per-vendor totals so the orchestration layer mirrors the canonical quote instead of recomputing aggregates. Checkout never recomputes unit prices, discounts, or totals—every vendor-order field is derived directly from the cart snapshot, and any delta only appears when reservation failures reject line items (in which case the vendor order is flagged `rejected` and a warning is emitted).
* `GroupCartItemsByVendor` produces the per-vendor slices consumed by the checkout orchestrator, while `ValidateBuyerStore`, `ValidateVendorStore`, and `ValidateMOQ` centralize the store-state/subscription/MOQ checks shared between cart upserts and checkout orchestration (`internal/checkout/helpers/grouping.go`; `internal/checkout/helpers/validation.go`). `ComputeVendorTotals`/`ComputeTotalsByVendor` remain available for ancillary helpers, but vendor order creation uses the vendor group snapshot for totals.
* PF-079 introduces `ReserveInventory` (same helper package) so checkout can issue conditional updates on `inventory_items` (e.g., `available_qty >= qty` to decrement `available_qty` and increment `reserved_qty`) and surface per-line reservation results for partial success without DB locks; any failed reservation marks that line item as rejected (with the returned reason) and, if every line under a vendor fails, the vendor order is marked rejected so the checkout response clearly surfaces the vendor-level failure.
* PF-080 describes the `internal/checkout/service` transaction that converts the `CartRecord` into a `CheckoutGroup`, per-vendor `VendorOrders`, and `OrderLineItems`, creates `PaymentIntent` rows, and marks the cart `converted` within a single transaction while relying on the helpers/reservation logic and idempotency caching so repeated checkout keys replay the cached response and avoid double-reserving inventory (`internal/checkout/service.go`).
  * As part of the same transaction the service overwrites `cart_records.shipping_address`, `payment_method`, `shipping_line`, `converted_at`, and `checkout_group_id`, ensuring `cart_records` stays in sync with the confirmed checkout decision before the vendor orders are materialized.
* PF-096 exposes `POST /api/v1/checkout` for buyer stores, requires `Idempotency-Key` (7d TTL via `middleware.Idempotency`) so identical retries replay the cached `checkoutResponse` while mismatched bodies return `pkg/errors.CodeIdempotency`/HTTP `409` (`api/middleware/idempotency.go`:37-208; `pkg/errors/errors.go`:9-64), and the handler enforces `store.Type == enums.StoreTypeBuyer`. The request now decodes `cart_id`, `shipping_address`, `payment_method`, and an optional `shipping_line` (the legacy `attributed_ad_click_id` is no longer accepted; checkout relies on the persisted `cart_records.ad_tokens` for attribution), validates via `internal/checkout.Service.Execute` that the cart belongs to the buyer, is `active`, and contains at least one `cart_item.status=ok` row before mutating state, and returns `vendor_orders` grouped by vendor plus `rejected_vendors` that enumerate every rejected line item (status/notes) and surface vendor-level warnings when a vendor has no eligible items, while also mirroring the confirmed `shipping_address`, `payment_method`, and `shipping_line` so the UI knows the exact checkout decision (`api/controllers/checkout.go`:9-145).
  * `pkg/checkout.ValidateMOQ` still runs inside the service; each MOQ violation yields `pkg/errors.CodeStateConflict`/HTTP `422` with a `violations` array (`product_id`, optional `product_name`, `required_qty`, `requested_qty`) so callers can highlight the offending SKUs before checkout splits the cart (`pkg/checkout/validation.go`:11-43).

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
* `ValidateVendorStore(*stores.StoreDTO, buyerState string)` now delegates to `pkg/visibility.EnsureVendorVisible`, so cart upserts and checkout use the same subscription/state gating before hitting the DB.

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

### Cart quote JSON helpers

* `CartItemWarnings` (`cart_item_warning` objects) store a `type` (`clamped_to_moq`, `price_changed`, `vendor_invalid`, etc.) alongside a `message`; the helper implements `driver.Valuer`/`sql.Scanner` so GORM persists the JSONB array in `cart_items.warnings`.
* `AppliedVolumeDiscount` mirrors the `label`/`amount_cents` payload clients send when a tiered discount applies; it’s stored in `cart_items.applied_volume_discount`.
* `VendorGroupWarnings` and `VendorGroupPromo` are the JSONB shapes persisted by the new `cart_vendor_groups` table so each vendor attribution can log warnings, promos, and totals.
  Invalid vendor promos now surface via `VendorGroupWarnings` (type `invalid_promo` with a stable message) without failing the quote so the client can explain why a promo was ignored.
* `internal/cart.Repository` now hydrates `CartRecord.VendorGroups` + `CartItem.Warnings` during fetches so services can inspect the authoritative quote before checkout conversion.
* `types.ShippingLine` (code/title/price) and `types.JSONMap` (flexible key/value maps) provide the same Value/Scan helpers used by `vendor_orders.shipping_line` and `vendor_orders.attributed_token`, letting those JSONB snapshots round-trip through GORM without bespoke serialization code.

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

* `admin` is also used for `/api/admin` routes that deliberately skip the store context middleware so JWTs with `role=admin` can omit `active_store_id`/`store_type`.
* `POST /api/admin/v1/auth/login` issues a storeless `access_token`/`refresh_token` pair for `users.system_role=admin`, sets `X-PF-Token` to the freshly minted access token while the JSON response returns `{"user":<users.UserDTO>,"refresh_token":<refresh_token>}`, and enforces the canonical `401 invalid credentials` when the email/password is wrong or the user lacks the `admin` system role. `AdminLogin` updates `last_login_at`, mints a JWT with `role=admin` and no `active_store_id`/`store_type`, and seeds the refresh session via `session.Generate` so admin tooling can call `/api/admin/*` without store context (api/routes/router.go:117-119; api/controllers/auth.go:39-61; internal/auth/service.go:160-194).
* `POST /api/admin/v1/auth/register` is the dev-only counterpart that only mounts when `cfg.App.IsProd()` is false and immediately rejects production traffic with `403 admin register disabled in production`. The controller calls `AdminRegisterService.Register`, which trims/lowercases the email, hashes the password with `security.HashPassword`, writes `system_role="admin"`/`is_active=true`, and returns `409 Conflict` if the email already exists, before proxying into `auth.AdminLogin` so the response stays alignment with the admin login payload while also writing `X-PF-Token`. Invalid payloads trigger the same `422` from `validators.DecodeJSONBody`, meaning dev automation can seed admins without touching psql while prod stays locked down (api/routes/router.go:109-119; api/controllers/auth.go:63-93; internal/auth/admin_register.go:16-92; pkg/config/config.go:19-36; pkg/security/password.go:29-44).

### `Outbox`

* `event_type_enum`: `order_created`, `license_status_changed`, `media_uploaded`, `notification_requested`, `ad_*`, etc.
* `aggregate_type_enum`: `vendor_order`, `checkout_group`, `license`, `store`, `media`, `ledger_event`, `notification`, `ad`.
* Helpers: `OutboxEventType`/`OutboxAggregateType` in `pkg/enums/outbox.go`.
* Outbox payload envelope struct and actor ref definitions live under `pkg/outbox/envelope.go`.
* Repository/service/registry infrastructure now lives under `pkg/outbox/registry` (see `registry/decoder.go`) so consumers register deterministic decoders while the dispatcher uses the same package to resolve each `event_type` to a single topic, expected aggregate, and typed payload struct stored under `pkg/outbox/payloads`.
* Idempotency manager: `pkg/eventing/idempotency.Manager` wraps Redis `SETNX` with the `pf:evt:processed:<consumer>:<event_id>` key pattern and respects `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (default `720h`) so consumers skip duplicate deliveries before applying side effects.
* Publisher worker: `cmd/outbox-publisher` fetches `published_at IS NULL` batches via `outbox.Repository.FetchUnpublishedForPublish` (locks rows with `FOR UPDATE SKIP LOCKED`, honors `Config.Outbox.BatchSize`/`MaxAttempts`, and orders by `created_at ASC, id ASC`), publishes each row sequentially to `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`, and calls `MarkPublishedTx` on success or `MarkFailedTx` (incrementing `attempt_count`, truncating `last_error`, then continuing) so a single failure never halts the dispatcher; idle or error cycles sleep using the base `Config.Outbox.PollIntervalMS` (default 500ms) with a capped exponential backoff (double up to 10s) plus 0‑250ms jitter to avoid thundering herds while staying responsive, and `publishRow` attaches metadata attributes before waiting (15s timeout) for the Pub/Sub ack (cmd/outbox-publisher/service.go:66-235; pkg/outbox/repository.go:20-101; docs/outbox.md).
* PF-142 introduced an event routing registry so unknown `event_type`s or invalid payload/envelope data are treated as terminal (soon-to-be DLQ) failures before publish, ensuring every dispatch emits what is stored in Postgres while the router enforces topic+aggregate invariants.
* Dead-letter persistence (PF-144): `pkg/outbox/DLQRepository` writes terminal events into the append-only `outbox_dlq` table, storing the exact original envelope (`event_id`, `event_type`, `aggregate_type`, `aggregate_id`, `payload_json`) plus `error_reason`, optional `error_message`, `attempt_count`, and `failed_at` so manual remediation tooling can replay or diagnose failures without touching the pending queue; DLQ writes happen atomically with the terminal mark and are never retried automatically.
* PF-145 ensures the dispatcher feels terminal failures early: a non-retryable error or `attempt_count >= Config.Outbox.MaxAttempts` triggers DLQ persistence before the row is marked terminal, and those terminal flags keep the row out of future `FetchUnpublishedForPublish` results so it is never reprocessed or published again.
* Each publish attempt emits structured log fields such as `outbox_id`, `event_type`, `aggregate_type`, `aggregate_id`, `event_id`, `attempt_count`, and `last_error` so retries stay traceable without hunting for rows directly.
* `Repository.DeletePublishedBefore` exposes the retention cleanup path for PF-140 so jobs can delete published rows older than the cutoff while filtering on `attempt_count >= 5`.
* Config knobs: `PACKFINDERZ_OUTBOX_PUBLISH_BATCH_SIZE` (default `50`), `PACKFINDERZ_OUTBOX_PUBLISH_POLL_MS` (default `500`), `PACKFINDERZ_OUTBOX_MAX_ATTEMPTS` (default `10`), the domain topic via `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`, and the compliance subscription via `PACKFINDERZ_PUBSUB_DOMAIN_SUBSCRIPTION` where `license_status_changed` consumers run.
* `license_status_changed` events are emitted by `internal/licenses/service` whenever a license is created or an admin decision lands, and `emitLicenseStatusEvent` runs inside the same transaction as the license mutation so the downstream worker sees consistent state (`internal/licenses/service.go`:136-419).
* `order_created` events fire when `internal/checkout/service.emitOrderCreatedEvent` (internal/checkout/service.go:150-271) builds an `OrderCreatedEvent` payload that copies the completed `checkout_group_id` and every `vendor_order_id`, tags the aggregate as `checkout_group`/`version=1` (pkg/enums/outbox.go:5-108), and emits it via `pkg/outbox.Service.Emit` so the outbox row persists in the same transaction that flips the cart to `converted`, guaranteeing consumers observe the same split metadata.
* `order_pending_nudge`/`order_expired` events are emitted by `internal/cron/order_ttl_job.go`: the job scans `vendor_orders.status = 'created_pending'`, emits `order_pending_nudge` exactly once when the row hits five days, expires the row after ten days, releases inventory via `orders.ReleaseLineItemInventory`, labels the line items `rejected`, updates `status = 'expired'`, and emits `order_expired` so both buyer/vendor notification consumers have deterministic TTL signals.
* `order_paid` events fire when `internal/orders.Service.ConfirmPayout` finalizes a payout: the payment intent transitions to `paid` with `vendor_paid_at`, the vendor order closes, a `vendor_payout` ledger row is recorded, and the event (enums.EventOrderPaid) carries the relevant order/payment IDs plus the payout timestamp so downstream consumers stay in sync (internal/orders/service.go:859-940; pkg/enums/outbox.go:54-91).
* `internal/notifications/consumer` (set up in `cmd/worker/main`) subscribes to the domain topic, uses `pkg/outbox/idempotency.Manager` to honor the `pf:evt:processed:<consumer>:<event_id>` TTL, and writes `NotificationTypeCompliance` rows with links and rejection details for admins/stores based on the status in the event payload (internal/notifications/consumer.go:18-186; cmd/worker/main.go:83-116).

### `LedgerEvent`

* `ledger_event_type_enum`: `cash_collected`, `vendor_payout`, `adjustment`, `refund`.
* `LedgerEventType` constants live in `pkg/enums/ledger_event_type.go`.
* `ledger_events` table stores `order_id`, `type`, `amount_cents`, optional `metadata`, and `created_at`; indexes `(order_id, created_at)` and `(type, created_at)` satisfy audit queries, and the `order_id` FK uses `ON DELETE RESTRICT` to preserve append-only semantics.
* Every row also captures `buyer_store_id`, `vendor_store_id`, and `actor_user_id` so buyers/vendors can filter the ledger by their stores and agents/admins can see who logged the cash collection or payout.
* `internal/ledger/service.RecordEvent` is the canonical helper and is already wired into payout confirmation flows so that only inserts ever touch `ledger_events`.
* `internal/ledger.Repository` only exposes `Create`/`ListByOrderID`, `internal/ledger.Service.RecordEvent` validates the enum, and no UPDATE/DELETE paths exist so the ledger remains append-only by construction (internal/ledger/service.go:22-64; internal/ledger/repo.go:12-38).
* `internal/orders.Service.ConfirmPayout` flips the matching payment intent to `paid`, stamps `vendor_paid_at`, closes the vendor order, records a `vendor_payout` ledger row, and emits the `order_paid` outbox event so every payout stays append-only while downstream consumers receive the final payment signal (internal/orders/service.go:859-940; pkg/enums/outbox.go:54-91; internal/ledger/service.go:22-64).
* `AdminPayoutOrders`/`AdminPayoutOrderDetail` surface the delivered + settled + unpaid queue so admins can review ready payouts before confirming (api/controllers/admin_orders.go:17-100; internal/orders/repo.go:561-620).

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
* `Repository.DeleteOlderThan` exposes the retention cleanup path so `notification-cleanup` (PF-139) can delete rows older than 30 days via `DELETE FROM notifications WHERE created_at < ?` while still running inside an explicit transaction (internal/notifications/repo.go:1-23).
* `Consumer` subscribes to `license_status_changed` events, honors `pkg/outbox/idempotency.Manager` TTLs, and writes `NotificationTypeCompliance` rows with the right admin/store link plus rejection details when present, keeping the event tied to the originating store (internal/notifications/consumer.go:18-186; cmd/worker/main.go:83-116).
* `Repository` now exposes `List`, `MarkRead`, and `MarkAllRead` while staying store-scoped; `List` orders by `(created_at, id) DESC`, honors `UnreadOnly`, and enforces the `pagination.NormalizeLimit` default (25) / max (100) plus `LimitWithBuffer` to surface the next cursor so paginated queries never exceed the caps, `MarkRead` only flips `read_at` when `NULL` for the matching `notification_id`/`store_id`, and `MarkAllRead` updates every unread row for the store before returning the rows affected (internal/notifications/repo.go:34-113; pkg/pagination/pagination.go:12-40).
* `Service` validates `StoreID`, decodes/encodes cursors with `pagination.ParseCursor`/`EncodeCursor`, and surfaces the `List`, `MarkRead`, and `MarkAllRead` helpers API controllers will consume while keeping store validation, pagination limits, and read-state idempotency centralized (internal/notifications/service.go:1-109; pkg/pagination/pagination.go:12-40).
* `ListNotifications`, `MarkNotificationRead`, and `MarkAllNotificationsRead` sit on top of the notifications service, parse `unreadOnly|limit|cursor` or `notificationId`, enforce the active `StoreID`, require the `Idempotency-Key` injected by `middleware.Idempotency`, and return the success envelopes (`{"items":…,"cursor":…}`, `{"read": true}`, `{"updated": count}`) while honoring store ownership so cross-tenant updates are rejected (api/controllers/notifications.go:1-118; api/routes/router.go:129-133; api/middleware/idempotency.go:37-208).

### `internal/consumers/analytics`
* `Consumer` decodes `order_created`, `cash_collected`, and `order_paid` outbox payloads, guards with `pf:evt:processed:analytics:<event_id>`, and inserts a single `marketplace_events` row per event via `pkg/bigquery.Client.InsertRows`.
* Rows capture `event_id`, `event_type`, `occurred_at`, optional store/order IDs, and the raw JSON payload stored through `bigquery.NullJSON`.
* Canonical analytics DTOs (envelope, marketplace/ad row, query request/response) now live under `internal/analytics/types`, and the analytics/enumeration helpers live in `pkg/enums/analytics_event_type.go` + `pkg/enums/ad_event_fact_type.go`.
* Any payload or insert failure deletes the idempotency key so retries are allowed, and the handler logs via `pkg/logger`.
* `cmd/analytics-worker` now wires an analytics service that decodes the canonical envelope, writes `pf:evt:processed:analytics:<event_id>` with `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL`, and routes to the handler stencil so duplicate Pub/Sub deliveries stay idempotent.
* `internal/analytics/router` validates canonical `event_type` routing and decodes the typed payload before invoking each handler stub, keeping the BigQuery writer interface consistent across events.
* `order_canceled`/`order_expired` handlers now emit termination rows with the payload-encoded reason/ttl metadata so revenue queries can automatically filter them out.

### `internal/cart`
* `Repository` secures `CartRecord` + `CartItem` persistence by scoping every operation to the owning `buyer_store_id`.
* `ReplaceItems` wipes the previous `cart_items` rows before inserting the new snapshot, while `UpdateStatus` flips the record from `active` to `converted`.
* Cart-level discounts map through `pkg/types.CartLevelDiscounts` when the repository writes/reads `cart_level_discount[]`.
* `Service.QuoteCart` enforces buyer KYC/role, vendor visibility (verified/subscribed/in-state), inventory availability, MOQ, volume-tier pricing, subtotal/total math, and cart-level discount metadata before the cart is created or updated so the returned record is the canonical checkout snapshot (`internal/cart/service.go:310-414`).
* `Service.GetActiveCart` validates the requesting buyer store, enforces buyer ownership, and returns the latest `cart_record` with joined `cart_items`, otherwise returning `pkgerrors.CodeNotFound` when no active cart exists (`internal/cart/service.go:259-284`).

### `internal/orders`
* `Repository` (internal/orders/interfaces.go:1-29) persists `checkout_group_id` anchored `vendor_orders`, `order_line_items`, and `payment_intents` plus `ListBuyerOrders`/`ListVendorOrders`, keeping each call scoped to the requesting buyer or vendor.
* Vendor orders now snapshot the cart state: each row stores `cart_id`, `currency`, `shipping_address`, `discounts_cents`, vendor-level `warnings`/`promo`, `payment_method`, `shipping_line`, and `attributed_token`, while the line items capture `cart_item_id`, `warnings`, `applied_volume_discount`, `moq`, `max_qty`, `line_subtotal_cents`, and `attributed_token` so analytics can replay the original cart quote without recomputing it later.
* `ListBuyerOrders` (internal/orders/repo.go:109-211) queries `vendor_orders AS vo`, joins `payment_intents` for payment status and `stores` for buyer/vendor metadata, selects `order_number`, totals, `payment_status`, `fulfillment_status`, `shipping_status`, vendor summary, and `total_items`, filters on `order_status`, fulfillment/shipping/payment statuses, `created_at` range, and a case-insensitive `Query` over both buyer/vendor names, orders by `created_at DESC`, `id DESC`, and paginates via `pkg/pagination.NormalizeLimit`, `LimitWithBuffer`, and the cursor helpers (`pkg/pagination/pagination.go:12-80`).
* `ListVendorOrders` (internal/orders/repo.go:214-326) mirrors the buyer query but scopes to `vendor_store_id`, joins `payment_intents`, selects buyer summary data, supports `fulfillment_status`, `shipping_status`, `payment_status`, actionable status lists, dates, and the same `Query` filter, and orders/paginates by the same cursor helpers so the vendor dashboard can scroll deterministically through `created_at DESC` results.
* `FindOrderDetail` (internal/orders/repo.go:322-347) loads one `vendor_orders` row preloading `order_line_items`, `payment_intent`, buyer/vendor `stores`, and the single `active` `order_assignments` record so services can render the detail view without extra joins; the DTO (internal/orders/dto.go:78-115) exposes `OrderDetail`, `LineItemDetail`, `PaymentIntentDetail`, and `OrderAssignmentSummary`. The `order_assignments` schema (with `active` defaults, foreign keys, and unique active-order index) is defined by `pkg/migrate/migrations/20260128000000_create_order_assignments_table.sql`, whose down script drops the indexes/table for reversibility.
* Agent endpoints (`GET /api/v1/agent/orders`, `GET /api/v1/agent/orders/{orderId}`) reuse `internal/orders.Repository.ListAssignedOrders` (join `order_assignments.active = true`, filter `agent_user_id`, order by created/id desc, `LimitWithBuffer`/cursor pagination) and `FindOrderDetail`; the controllers parse the JWT user ID, page via `pagination.Params`, and reject any detail request whose `ActiveAssignment` is missing or belongs to someone else so only agents see their assigned work (api/controllers/agent_assigned_orders.go:9-109; internal/orders/repo.go:376-596).
* `internal/orders.Service.AgentPickup` verifies the active assignment belongs to the caller, restricts the transition to `hold|hold_for_pickup` (or treats `in_transit` as idempotent), sets `vendor_orders.status`/`shipping_status` to `in_transit`, and timestamps `order_assignments.pickup_time` via the new meta columns (`pickup_time`, `delivery_time`, `cash_pickup_time`, `pickup_signature_gcs_key`, `delivery_signature_gcs_key`) so pickup confirmations stay idempotent (internal/orders/service.go:641-712; pkg/migrate/migrations/20260129000000_add_order_assignment_meta.sql).
* `internal/orders.Service.AgentDeliver` also validates the assignment ownership, enforces the order is `in_transit`, updates `vendor_orders.status`/`shipping_status` to `delivered`, writes `vendor_orders.delivered_at`, and records `order_assignments.delivery_time` (if not already populated) so duplicate deliver calls become safe no-ops (internal/orders/service.go:724-778; pkg/migrate/migrations/20260129000000_add_order_assignment_meta.sql).
* `api/controllers/orders.List`/`Detail` wire `GET /api/v1/orders` and `GET /api/v1/orders/{orderId}` to the buyer/vendor helpers, enforcing `activeStoreId`/`StoreType` via `middleware.StoreIDFromContext`/`StoreTypeFromContext` so buyers/vendors only see their perspective, and mapping validation errors (pagination, filters, UUID) plus 403/404 cases around ownership (`api/controllers/orders/orders.go`:13-221).
* `BuyerOrderFilters`, `VendorOrderFilters`, `BuyerOrderSummary`, `VendorOrderSummary`, `BuyerOrderList`, `VendorOrderList`, and `OrderStoreSummary` (internal/orders/dto.go:10-75) define the inputs/outputs: optional status/date filters plus `Query`, and responses contain sequential `order_number`, totals, status enums, `total_items`, and store summaries (vendor rows omit logos per the MVP assumption).
* The `VendorOrder` model (pkg/db/models/vendor_order.go:12-37) records `fulfillment_status`, `shipping_status`, and `order_number`; `pkg/migrate/migrations/20260126000001_add_vendor_order_fields.sql:4-51` introduces the enum types, `vendor_order_number_seq`, the `order_number` column, and `ux_vendor_orders_order_number` which backs the incremental identifier.
* `api/controllers/orders.VendorOrderDecision` requires a vendor store context, parses `{decision: accept|reject}`, enforces the `/api/v1/vendor/orders/{orderId}/decision` route’s `middleware.Idempotency` guard, delegates to `internal/orders.Service.VendorDecision`, and keeps the response idempotent while emitting the `order_decided` outbox event.
* `internal/orders.Service.VendorDecision` validates `VendorOrderStatus` is `created_pending`, transitions it to `accepted` or `rejected`, and emits `order_decided` (event_type `enums.EventOrderDecided`) inside the same transaction that updates the vendor order, protecting the buyer dashboard from duplicate state changes (`internal/orders/service.go:24-147`; pkg/enums/outbox.go:57-69).
* `api/controllers/orders.VendorLineItemDecision` enforces the vendor store context for `POST /api/v1/vendor/orders/{orderId}/line-items/decision`, parses `{line_item_id, decision: fulfill|reject, notes?}`, and routes the request to `internal/orders.Service.LineItemDecision`, while `middleware.Idempotency` keeps duplicates idempotent (`api/routes/router.go:60-116`; `api/controllers/orders/orders.go:222-318`).
* `internal/orders.Service.LineItemDecision` loads the order & line item, releases inventory for rejects, recomputes `subtotal_cents`/`total_cents`/`balance_due_cents`, updates `fulfillment_status`/`status` when all pending items are resolved, and emits `order_fulfilled` (event_type `enums.EventOrderFulfilled`) so buyers can react to the final shipment state (`internal/orders/service.go:180-359`; pkg/enums/outbox.go:57-72`).
* `api/controllers/orders.VendorLineItemDecision` enforces the vendor store context, parses `{line_item_id, decision: fulfill|reject, notes?}`, and routes the request to `internal/orders.Service.LineItemDecision`.
* `internal/orders.Service.LineItemDecision` loads the order + line item, checks vendor ownership, updates the line item status, releases inventory for rejects, recomputes `subtotal_cents`, `total_cents`, and `balance_due_cents`, sets `fulfillment_status`/`status` to `partial`/`fulfilled` and `hold` when all `pending` line items are resolved, and emits the new `order_fulfilled` outbox event (`enums.EventOrderFulfilled`; pkg/enums/outbox.go:57-72).
* `api/controllers/orders.CancelOrder`, `NudgeVendor`, and `RetryOrder` gate the new buyer actions (`POST /api/v1/orders/{orderId}/cancel|/nudge|/retry`), enforce the buyer store context, parse the `orderId` route param, and forward to `internal/orders.Service.CancelOrder`, `.NudgeVendor`, or `.RetryOrder` respectively, returning the canonical success envelope plus the new `order_id` for retries.
* `internal/orders.Service.CancelOrder` validates the pre-transit state, releases inventory for non-fulfilled items, marks them rejected, zeros `balance_due_cents`, and emits `order_canceled`.
* `internal/orders.Service.NudgeVendor` emits `notification_requested` (payload `Type=order_nudge`) so notification consumers can deliver reminders.
* `internal/orders.Service.RetryOrder` rewinds the expired vendor order, creates a new `vendor_orders`/`order_line_items` snapshot keyed by a freshly minted `checkout_group_id` (no `checkout_groups` table involved), re-reserves inventory via `reservation.ReserveInventory`, builds a fresh payment intent, and emits `order_retried` (payload includes the original and new order IDs) so the buyer can retry without re-running the whole checkout flow.
* `api/controllers/orders` expose `GET /api/v1/orders` and `GET /api/v1/orders/{orderId}`, parsing the shared filters/search params, enforcing buyer/vendor perspective-based authorization, and returning the repository's paginated DTOs and order detail.

### `internal/products`
* `Repository` exposes product CRUD plus detail/list reads that preload `Inventory`, `VolumeDiscounts` (descending `min_qty`), and `Media` (ascending `position`) so services get a single `Product` model with the related SKU, pricing, inventory, discounts, and ordered media (internal/products/repo/repository.go:60-208).
* `UpsertInventory`/`GetInventoryByProductID` respect the 1:1 `inventory_items.product_id PK` row, while `CreateVolumeDiscount`/`ListVolumeDiscounts`/`DeleteVolumeDiscount` keep the `(product_id,min_qty)` uniqueness and descending salary order for tiered pricing lookups (internal/products/repo/repository.go:133-175).
* `VendorSummary` is built via `vendorSummaryQuery`, joining `stores` to the latest `media_attachments` row where `entity_type='store'` so the stored `LogoMediaID`/`LogoGCSKey` can be signed without traversing `media`; the `(entity_type,entity_id)` and `(media_id)` indexes keep these tenant-logo lookups fast (internal/products/repo/repository.go:34-208). Attachment usage must obey the lifecycle/deletion rules documented in `docs/media_attachments_lifecycle.md`, and reconciliation is centralized in `internal/media.NewAttachmentReconciler`, which diffs the entity’s old vs new media IDs inside the same transaction, refuses to bind `media` rows owned by a different `store_id`, fetches the signing key via `media.Repository.FindByID`, and issues the matching `MediaAttachmentRepository` create/delete calls so no domain hand-writes `media_attachments` rows or mutates existing attachments.
* Every non-protected attachment cleanup runs through `cmd/media_deleted_worker/main` + `internal/media/consumer.DeletionConsumer`: they subscribe to `media-deleted-sub`, parse the GCS `OBJECT_DELETE` JSON_API_V1 payload, map `object.name` to `media_id`, iterate attachments in stable order, run per-domain detachment hooks before deleting rows, and remain idempotent so retries safely replay the cleanup path outside the API (cmd/media_deleted_worker/main.go:1-70; internal/media/consumer/deletion_consumer.go:1-200).
* `DELETE /api/v1/media/{mediaId}` enforces the same lifecycle by loading `media_attachments`, rejecting the request whenever any `entity_type` is in `ProtectedAttachmentEntities` (`license`/`ad`), and only emitting the downstream delete event after the guard clears so protected assets never lose their attachments before a worker detaches the remaining references.
* `service` enforces vendor store type, allowed user roles, `reserved_qty <= available_qty`, unique `min_qty` per discount, and that each requested media belongs to the store with `kind=product`; product, inventory, discounts, and product media are saved inside a single transaction before `NewProductDTO` returns the created record with the preloaded vendor summary (internal/products/service.go:63-204).
* `service.DeleteProduct` reuses the same ownership + role guards, fetches the product to ensure it belongs to the active vendor, then deletes the row so FK cascades remove inventory, discounts, and media attachments while the route returns `204` (internal/products/service.go:317-338).
* `service.UpdateProduct` applies the optional changes via `applyUpdateToProduct`, synchronously replaces inventory rows, volume discounts, and media attachments (via `buildProductMediaRows`), defends against duplicate discounts/media IDs, enforces that each media belongs to the active store with `kind=product`, revalidates ownership/roles, and returns the updated `ProductDTO` so the PATCH endpoint surfaces the same canonical payload as creation (internal/products/service.go:226-355).

### `internal/schedulers/licenses`
* `Service` encapsulates expiry warnings and expirations; `Run` ticks every `schedulerInterval` (24h) before calling `process`, keeping the scheduler loop off the hot API path while letting whoever boots it handle logging/metrics (`internal/schedulers/licenses/service.go`:1-220).
* `process` sequentially runs `warnExpiring` and `expireLicenses`, accumulates their errors, and leaves resilience strategies (exponential backoff, alerting) to the caller so a single failure never takes down the Cron loop (`internal/schedulers/licenses/service.go`:94-142).
* `warnExpiring`/`expireLicenses` query `FindExpiringBetween`/`FindExpiredByDate`, emit `license_status_changed` events via `outbox.Service.Emit` (warnings include the `expires on …` reason and `warningType=expiry_warning`), and wrap each transition in `db.WithTx` so the mutation, the emitted event, and any KYC reconciliation happen atomically (`internal/schedulers/licenses/service.go`:61-210).
* `expireLicense` reloads the row inside the same transaction, ignores already-`expired` statuses, updates the license, calls `reconcileKYC` (which uses `DetermineStoreKYCStatus`), and emits the `expired by scheduler` event so downstream consumers see the new status plus the updated store mirror (`internal/schedulers/licenses/service.go`:174-210; `internal/licenses/service.go`:405-416).

### `internal/cron`
* `internal/cron.Registry` tracks the ordered set of jobs to run; `NewRegistry(jobs...)` builds the list, `Register` appends, and `Jobs()` returns a defensive copy so the loop can iterate without races (`internal/cron/registry.go`:11-38).
* `internal/cron.Lock` abstracts exclusive execution; `RedisLock` keys like `pf:cron-worker:lock:<env>` use `SETNX`/TTL plus verification so only the owner that holds the lease can release it, surviving node crashes gracefully (`internal/cron/lock.go`:1-80).
* `internal/cron.Service` loops every 24h (customizable), acquires the lock, logs when leadership is already taken, iterates through `Registry.Jobs()`, emits structured logs (`job start`, `job completed`, `job failed`, `duration_ms`), and records `pkg/metrics.CronJobMetrics` so every job reports `job_duration_seconds`, `job_success`, and `job_failure`; job failures never stop the next job or the next cycle (`internal/cron/service.go`:14-154; `pkg/metrics/cron.go`:16-40).
* `cmd/cron-worker/main` boots config/logger/DB/Redis, flips `cfg.Service.Kind=cron-worker`, runs dev migrations, wires `metrics.NewCronJobMetrics(prometheus.DefaultRegisterer)`, creates `cron.NewRedisLock`, and builds `cron.NewService` with `cron.NewRegistry()` (currently empty) so the cron worker exercises the locking + metrics plumbing until domain jobs register themselves (`cmd/cron-worker/main.go`:13-112; `pkg/migrate/autorun.go`:12-34).
* `internal/cron/order_ttl_job.go` contains PF-138’s pending-order TTL work: it scans `vendor_orders.status='created_pending'` for the 5/10 day thresholds, emits `order_pending_nudge` once, expires stale orders at 10 days, releases inventory through `orders.ReleaseLineItemInventory`, and emits `order_expired` after mutating the row so consumers see the deterministic TTL guard.
* `internal/cron/notification_cleanup_job.go` implements PF-139: it computes `now - 30 days`, calls `notifications.Repository.DeleteOlderThan(ctx, tx, cutoff)` inside `db.WithTx`, and logs the rows deleted so the `notification-cleanup` job keeps `notifications` bounded while remaining idempotent.
* `internal/cron/outbox_retention_job.go` implements PF-140: it computes the 30-day cutoff, calls `outbox.Repository.DeletePublishedBefore(ctx, tx, cutoff, minAttempts=5)`, and logs `rows_deleted`, `min_attempts`, and `cutoff` so the `outbox-retention` job keeps `outbox_events` bounded without touching active consumers.

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

### `internal/auth`

* `internal/auth.Service.Login` pairs membership data with `users.system_role`, lets system agents mint tokens with `role=agent` even without store records, and keeps `/api/v1/agent/*` guarded by `RequireRole("agent")` (internal/auth/service.go).
* PF-198 removed the `users.store_ids` array so the service now relies entirely on the `store_memberships` table to resolve which stores a user can act on.
* PF-200 extends `/api/v1/auth/register` so an existing email (with the matching password) can create another store + owner membership instead of duplicating the user row inside the registration transaction (`internal/auth/register.go`:21-133).
* `api/middleware.Auth` parses the JWT via `pkg/auth.ParseAccessToken`, verifies the refresh session via `session.AccessSessionChecker.HasSession`, and seeds context with `user_id`, `role`, plus optional `store_id`/`store_type` so `middleware.RequireRole("agent")`/`("admin")` can block unauthorized routes even when `activeStoreId` is nil (api/middleware/auth.go:23-80; api/middleware/roles.go:1-27).

### `internal/analytics`

* `internal/analytics.Service.Query` verifies the active store context (vendor or buyer), normalizes either preset (`7d|30d|90d`) or custom `from`/`to` ranges, and returns the KPIs + derived time series directly from `marketplace_events` using the BigQuery query service.
* `api/controllers/analytics/vendor` enforces vendor-only access, reuses the shared range resolver, and forwards the request via `internal/analytics.Service` so `/api/v1/vendor/analytics` can stay read-only while returning the canonical success envelope.
* `api/controllers/analytics/marketplace` enforces any valid store type, resolves the same timeframe, and calls `internal/analytics.Service.Query` so `/api/v1/analytics/marketplace` exposes the same data to both buyers and vendors scoped by `activeStoreId`.

### `internal/billing`

* `Repository` lives next to `pkg/db/models` (`subscription.go`, `payment_method.go`, `charge.go`, `usage_charge.go`) and exposes scoped CRUD helpers for `subscriptions`, `payment_methods`, `charges`, and `usage_charges`. Every call filters by `store_id`, orders by `created_at DESC`, and returns `nil` when no subscription exists so downstream services can gate features by ownership.
* `Service` composes the repository, guards against `nil` dependencies, and exposes methods like `CreateSubscription`, `ListCharges`, and `CreateUsageCharge`, letting controllers/consumers record Stripe state without replaying SQL.
* `Service.ListCharges` accepts store-scoped `limit`, `cursor`, `type`, and `status`, normalizes the cursor pagination via `pkg/pagination`, and returns the canonical `charges` rows plus the next cursor so vendor billing dashboards stay consistent.
* `api/controllers/billing.VendorBillingCharges` enforces vendor context, parses those filters, and streams `charges[]` (amount, currency, type, status, description, billed_at, created_at) plus the encoded cursor so `/api/v1/vendor/billing/charges` mirrors Stripe/local history without hitting Stripe per request.
* The new tables persist Stripe data per store: `subscriptions` carries the Stripe ID, status, period window timestamps, cancel metadata, and arbitrary metadata; `payment_methods` stores the Stripe payment method ID, `payment_method_type`, fingerprint, card brand/last4/expiry, billing details, and metadata; `charges` records amounts, currency, `charge_status`, optional description, `billed_at`, and nullable relations back to subscriptions/payment methods; `usage_charges` keeps metered quantities/amounts per subscription/charge with billing-period windows (pkg/db/models/subscription.go:12-41; pkg/db/models/payment_method.go:13-36; pkg/db/models/charge.go:13-38; pkg/db/models/usage_charge.go:12-36; pkg/migrate/migrations/20260201000000_create_billing_tables.sql:38-156).
* Vendor subscription creation/cancellation is implemented in `api/controllers/subscriptions/vendor` and `internal/subscriptions.Service`, which drive Stripe + DB state in one transaction, gate `stores.subscription_active`, and uses the configured `PACKFINDERZ_STRIPE_SUBSCRIPTION_PRICE_ID` as the default plan.


## API

---

### Routes

* `/health`
* `/api/v1/auth/login`
* `/api/public/*`
* `/api/*` (auth)
* `/api/admin/*`
* `/api/v1/agent/*`
* `/api/v1/agent/orders`
* `/api/v1/agent/orders/{orderId}`
* `/api/v1/agent/orders/queue`
* `/api/v1/vendor/analytics`
* `/api/v1/analytics/marketplace`
* `/api/v1/vendor/billing/charges`

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
