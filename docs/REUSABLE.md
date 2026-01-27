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
* `GET /api/v1/cart` fetches the active `cart_record` (plus line items) for the buyer store so the UI can recover an in-progress checkout without mutating state.

### `orders`

* `internal/orders.Repository` writes and reads `checkout_groups`, `vendor_orders`, `order_line_items`, and `payment_intents` so checkout execution can persist and rehydrate the per-vendor order snapshot and payment state.
* Methods preload `VendorOrders.Items` + `PaymentIntent` to keep the in-memory checkout snapshot consistent while fetching by checkout group or order.
* `ListBuyerOrders` exposes cursor pagination via `pkg/pagination`, returning `BuyerOrderList` with `order_number`, totals, `total_items`, payment/fulfillment/shipping statuses, vendor summary, and `next_cursor`.
* `BuyerOrderFilters` govern the list: `order_status`, `fulfillment_status`, `shipping_status`, `payment_status`, `date_from/date_to`, and a normalized `q` search across buyer/vendor names. `vendor_orders` now store `fulfillment_status`, `shipping_status`, and a sequential `order_number` backed by matching enums/indexes.
* `ListVendorOrders` mirrors the buyer list for vendor stores: cursor pagination, `VendorOrderFilters` covering `order_status`, `fulfillment_status`, `shipping_status`, `payment_status`, `date_from/date_to`, `actionable_state` sets, plus normalized search that spans the buyer or vendor names while returning buyer summary info.
* `FindOrderDetail` pulls a single `vendor_orders` row with `order_line_items`, `payment_intent`, buyer/vendor summaries, and the active `order_assignments` row (null when no agent assigned) so controllers can render detail views without extra queries.

### `checkout`

* `internal/checkout/helpers` contains deterministic, database-free logic that groups `CartItem`s by `vendor_store_id`, recomputes vendor totals, and validates buyer/vendor eligibility (store type, subscription, state) plus MOQ compliance before executing checkout.
* `GroupCartItemsByVendor` produces the per-vendor slices consumed by `ComputeVendorTotals`/`ComputeTotalsByVendor` so the checkout group and per-vendor order totals are deterministic, while `ValidateBuyerStore`, `ValidateVendorStore`, and `ValidateMOQ` centralize the store-state/subscription/MOQ checks shared between cart upserts and checkout orchestration (`internal/checkout/helpers/grouping.go`; `internal/checkout/helpers/validation.go`).
 * PF-079 introduces `ReserveInventory` (same helper package) so checkout can issue conditional updates on `inventory_items` (e.g., `available_qty >= qty` to decrement `available_qty` and increment `reserved_qty`) and surface per-line reservation results for partial success without DB locks.
 * PF-080 describes the `internal/checkout/service` transaction that converts the `CartRecord` into a `CheckoutGroup`, per-vendor `VendorOrders`, and `OrderLineItems`, creates `PaymentIntent` rows, and marks the cart `converted` within a single transaction while relying on the helpers/reservation logic and idempotency caching so repeated checkout keys replay the cached response and avoid double-reserving inventory (`internal/checkout/service.go`).
* PF-096 exposes `POST /api/v1/checkout` for buyer stores, requires `Idempotency-Key` (7d TTL via `middleware.Idempotency`) so identical retries replay the cached `checkoutResponse` while mismatched bodies return `pkg/errors.CodeIdempotency`/HTTP `409` (`api/middleware/idempotency.go`:37-208; `pkg/errors/errors.go`:9-64), and the handler enforces `store.Type == enums.StoreTypeBuyer`, decodes `cart_id` + optional `attributed_ad_click_id`, calls `internal/checkout.Service.Execute`, and returns `vendor_orders` grouped by vendor plus `rejected_vendors` that enumerate every rejected line item (status/notes) so the UI can show which vendors/items failed (`api/controllers/checkout.go`:9-145).
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
* `order_created` events fire when `internal/checkout/service.emitOrderCreatedEvent` (internal/checkout/service.go:150-271) builds an `OrderCreatedEvent` payload that copies the completed `checkout_group_id` and every `vendor_order_id`, tags the aggregate as `checkout_group`/`version=1` (pkg/enums/outbox.go:5-108), and emits it via `pkg/outbox.Service.Emit` so the outbox row persists in the same transaction that flips the cart to `converted`, guaranteeing consumers observe the same split metadata.
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
* `Consumer` subscribes to `license_status_changed` events, honors `pkg/outbox/idempotency.Manager` TTLs, and writes `NotificationTypeCompliance` rows with the right admin/store link plus rejection details when present, keeping the event tied to the originating store (internal/notifications/consumer.go:18-186; cmd/worker/main.go:83-116).
* `Repository` now exposes `List`, `MarkRead`, and `MarkAllRead` while staying store-scoped; `List` orders by `(created_at, id) DESC`, honors `UnreadOnly`, and enforces the `pagination.NormalizeLimit` default (25) / max (100) plus `LimitWithBuffer` to surface the next cursor so paginated queries never exceed the caps (internal/notifications/repo.go:34-80; pkg/pagination/pagination.go:12-40).
* `Service` validates `StoreID`, decodes/encodes cursors with `pagination.ParseCursor`/`EncodeCursor`, and surfaces the `List`, `MarkRead`, and `MarkAllRead` helpers API controllers will consume while keeping store validation and pagination limits centralized (internal/notifications/service.go:1-87; pkg/pagination/pagination.go:12-40).

### `internal/cart`
* `Repository` secures `CartRecord` + `CartItem` persistence by scoping every operation to the owning `buyer_store_id`.
* `ReplaceItems` wipes the previous `cart_items` rows before inserting the new snapshot, while `UpdateStatus` flips the record from `active` to `converted`.
* Cart-level discounts map through `pkg/types.CartLevelDiscounts` when the repository writes/reads `cart_level_discount[]`.
* `Service.UpsertCart` enforces buyer KYC/role, vendor visibility (verified/subscribed/in-state), inventory availability, MOQ, volume-tier pricing, subtotal/total math, and cart-level discount metadata before the cart is created or updated so the returned record is the canonical checkout snapshot (`internal/cart/service.go:39-209`).
* `Service.GetActiveCart` validates the requesting buyer store, enforces buyer ownership, and returns the latest `cart_record` with joined `cart_items`, otherwise returning `pkgerrors.CodeNotFound` when no active cart exists (`internal/cart/service.go:259-284`).

### `internal/orders`
* `Repository` (internal/orders/interfaces.go:1-29) persists `checkout_groups`, `vendor_orders`, `order_line_items`, and `payment_intents` plus `ListBuyerOrders`/`ListVendorOrders`, keeping each call scoped to the requesting buyer or vendor.
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
* `internal/orders.Service.RetryOrder` rewinds the expired vendor order, creates a new `checkout_groups`/`vendor_orders`/`order_line_items` snapshot, re-reserves inventory via `reservation.ReserveInventory`, builds a fresh payment intent, and emits `order_retried` (payload includes the original and new order IDs) so the buyer can retry without re-running the whole checkout group.
* `api/controllers/orders` expose `GET /api/v1/orders` and `GET /api/v1/orders/{orderId}`, parsing the shared filters/search params, enforcing buyer/vendor perspective-based authorization, and returning the repository's paginated DTOs and order detail.

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

### `internal/auth`

* `internal/auth.Service.Login` pairs membership data with `users.system_role`, lets system agents mint tokens with `role=agent` even without store records, and keeps `/api/v1/agent/*` guarded by `RequireRole("agent")` (internal/auth/service.go).
* `api/middleware.Auth` parses the JWT via `pkg/auth.ParseAccessToken`, verifies the refresh session via `session.AccessSessionChecker.HasSession`, and seeds context with `user_id`, `role`, plus optional `store_id`/`store_type` so `middleware.RequireRole("agent")`/`("admin")` can block unauthorized routes even when `activeStoreId` is nil (api/middleware/auth.go:23-80; api/middleware/roles.go:1-27).


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
