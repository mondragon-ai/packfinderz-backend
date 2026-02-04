

# PackFinderz — B2B Cannabis Marketplace

> **A compliant, multi-vendor B2B cannabis operating system**
> Supports licensed buyers and vendors with **correct inventory reservation**, **multi-vendor checkout**, **cash-at-delivery (MVP)**, **agent delivery**, **append-only ledger**, **ads + analytics**, and a **future-proof async architecture**.

---

## Table of Contents

1. [Overview](#overview)
2. [Repository Conventions](#repository-conventions)
3. [Grounding Rules](#grounding-rules)
4. [Development Setup](#development-setup)
5. [Core Components](#core-components)
6. [Storage Schema](#storage-schema)
7. [Testing & Quality](#testing--quality)
8. [API Usage](#api-usage)
9. [Configuration](#configuration)
10. [Development Workflow](#development-workflow)
11. [Safety Boundaries](#safety-boundaries)
12. [Documentation](#documentation)
13. [Makefile Reference](#makefile-reference)
14. [Quick Reference](#quick-reference)

---

## Overview

### Purpose

**PackFinderz** is a **two-sided B2B marketplace** for regulated cannabis commerce, designed to be:

* **Correct by construction** (no overselling, auditable money flow)
* **Compliance-first** (licenses, verification, admin oversight)
* **Multi-tenant** (buyer & vendor stores, multi-store users)
* **Async-safe** (Outbox → Pub/Sub, idempotent workers)
* **Extensible** (ads, analytics, ACH, multi-state)

### Key Capabilities (MVP)

* Multi-vendor checkout anchored by the cart's `checkout_group_id` (no standalone `checkout_groups` table)
* Atomic inventory reservation (optimistic + retry)
* Vendor accept/reject at **order and line-item level**
* Internal agent delivery with **cash-at-delivery**
* Internal agents authenticate via `users.system_role='agent'`, receive JWTs with `role=agent`, and use `/api/v1/agent/orders` (list/detail) plus `/api/v1/agent/orders/queue` to manage assigned and unassigned pickups. They confirm handoffs (pickup/deliver) through `POST /api/v1/agent/orders/{orderId}/pickup` and `/api/v1/agent/orders/{orderId}/deliver`, which transition the vendor order through `in_transit` → `delivered` while recording the agent’s timestamps.
* Agents confirm pickups with `POST /api/v1/agent/orders/{orderId}/pickup`, which marks the order as `in_transit` while recording the assignment’s `pickup_time` and rejecting invalid states.
* After delivery they call `POST /api/v1/agent/orders/{orderId}/cash-collected` so the payment intent is marked `settled` with `cash_collected_at`, the assignment records `cash_pickup_time`, the order balance zeroes out, the `cash_collected` outbox event is emitted, and the ledger logs `cash_collected` exactly once; duplicate cash collection requests fail once the intent is already `settled`, `paid`, `failed`, or `rejected`, validation failures mark the intent `failed`, store a failure reason, put the order on hold, and emit a `payment_failed` event so administrators can intervene before retries.
* Append-only **ledger events**
* Subscription-gated vendor visibility
* Ads with **last-click attribution**
* BigQuery analytics (analytics-only)

### Architecture Summary

* **API Monolith (Go)**: all synchronous decisions
* **Postgres (Heroku SQL)**: authoritative source of truth
* **PostGIS**: geo-search & delivery radius filtering
* **Redis**: idempotency + ad budget gating (ephemeral only)
* **Outbox Pattern** → **Pub/Sub** → **Workers**
* **BigQuery**: analytics warehouse (never authoritative)
* **GCS**: licenses, COAs, manifests, media

## Infrastructure Bootstraps

### Database & Heroku SQL

`pkg/db` is the shared GORM bootstrap that both the API and worker binaries consume. It honors `PACKFINDERZ_DB_DSN` (or the legacy host/port vars) and exposes knobs (`PACKFINDERZ_DB_MAX_*`, `PACKFINDERZ_DB_CONN_*`) for pooling/timeouts before returning helpers such as `Ping`, `WithTx`, and context-bound raw SQL executions. Domain repositories should accept `*gorm.DB` via constructor injection (see `internal/repo.Base`) and call `WithTx` or the raw SQL helpers for atomic operations, while schema work stays in Goose migrations. `make dev` handles Heroku SQL startup & redis Startup

### Heroku Deployment

`heroku.yml` wires the `web` dyno to `./bin/api` and the `worker` dyno to `./bin/worker`, matching the Go binaries produced by the Buildpack. Follow the [Heroku release & deploy checklist](docs/heroku_deploy.md) before pushing to production so migrations, readiness checks, and post-deploy verifications happen consistently.

### Redis & Readiness

`pkg/redis` wraps the Heroku-friendly go-redis client, enforces sensible dial/read/write timeouts, and exposes key builders for idempotency, counters, rate-limits, and refresh-token sessions. Its `Ping` method is now part of `/health/ready`, so the readiness endpoint verifies both Postgres and Redis before advertising readiness. Configure Redis via `PACKFINDERZ_REDIS_URL` (or address/password) plus the optional pooling/timeouts (`POOL_SIZE`, `MIN_IDLE_CONNS`, `DIAL_TIMEOUT`, `READ_TIMEOUT`, `WRITE_TIMEOUT`).

### Worker Bootstrapping

`cmd/worker` now mirrors the API stack by loading config, structured logging, GORM DB, Redis, Pub/Sub, and GCS clients before handing control to its long-running service loop. The new `pkg/pubsub` helper confirms the configured subscriptions exist and offers a Ping surface that the worker runs alongside `db.Ping` (and `redis.Ping`) to guard readiness, so failures stop startup instead of letting the Heroku worker dyno spin without its dependencies. The worker context carries `serviceKind=worker` and emits structured heartbeat logs while the consumers run.

The worker also consumes the `gcp-meda-sub` Pub/Sub subscription for GCS `OBJECT_FINALIZE` notifications and marks matching `media.gcs_key` rows as `uploaded`, keeping the media lifecycle in sync with bucket uploads.

### Cron Worker

`cmd/cron-worker` is the dedicated scheduler binary for time-based invariants. It boots the shared config, structured logger, Postgres, and Redis clients (and runs Goose migrations in dev), then loops every 24 hours while coordinating a global Redis lock so only one instance runs the jobs. Each job start/end/duration is logged and emits Prometheus metrics (`job_duration_seconds`, `job_success`, `job_failure`) so the cron layer can be monitored independently of the API and worker dynos.

The first job running today enforces the license lifecycle: it issues the `license_expiring_soon` warning 14 days before expiration, marks verified licenses as `expired` and re-evaluates store KYC, and finally removes license+media/attachment rows (plus their GCS objects) when the expiration date is more than 30 days in the past so the compliance tables stay bounded while the cron worker emits deterministic outbox events for observability.

The cron worker also runs the order TTL scheduler (PF-138), nudging vendors with `order_pending_nudge` once orders hit five days pending and expiring them after ten days while releasing inventory and emitting `order_expired` events so downstream consumers can notify both buyer and vendor deterministically. It additionally runs the notification cleanup job (PF-139) so `notifications` rows older than 30 days are purged daily, the outbox retention job (PF-140) which removes published `outbox_events` older than 30 days whose `attempt_count` already indicates they have been retried via the DLQ, and the new pending media cleanup job (PF-204) that deletes `media.status=pending` rows older than seven days alongside any attachments so abandoned uploads never linger.

### Outbox Publisher

`cmd/outbox-publisher` is the dedicated outbox publisher that polls `outbox_events` with `FOR UPDATE SKIP LOCKED`, publishes domain envelopes to `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`, and marks `published_at` or increments `attempt_count`/`last_error` before committing the transaction. Each publish attempt emits structured log fields such as `event_id`, `attempt_count`, and `last_error` so retries are traceable without digging into Postgres. Run it locally with `make run-outbox-publisher`, build it via `make build-outbox-publisher`, and run the `outbox-publisher` dyno in Heroku alongside `api`/`worker`. The operational guide in `docs/outbox.md` explains claiming, at-least-once semantics, retry expectations, and consumer idempotency requirements. When the API completes checkout it writes an `order_created` outbox row (payload includes `checkout_group_id` plus the `vendor_order_ids`) so analytics and notification consumers can react to the same transactional split recorded in Postgres.

---

## Repository Conventions

### Code Style

* Go (idiomatic, explicit error handling)
* DTOs separate from DB models
* Structured JSON logging
* No secrets in logs
* Minimal, additive changes only

### Folder Layout (Canonical)

```
cmd/
  api/               # API binary (authoritative decisions)
  worker/            # Async workers (side effects only)

api/
  routes/            # chi router wiring
  controllers/       # thin HTTP handlers
  middleware/        # auth, RBAC, idempotency
  validators/        # request validation
  responses/         # canonical JSON envelopes

internal/
  auth/
  stores/
  users/
  licenses/
  products/
  inventory/
  cart/
  checkout/
  orders/
  payments/
  ledger/
  ads/
  analytics/
  notifications/
  agents/
  admin/
  outbox/
  schedulers/

pkg/
  config/
  db/
  redis/
  geo/
  logging/
  errs/
  security/

docs/                # architecture, ADRs, runbooks
```

---

## Grounding Rules

**READ BEFORE CHANGING CODE**

* Work **only** within this repo.
* **Do not invent scope** or env vars.
* Prefer **additive changes**; refactors require ADRs.
* **Correctness > performance**.
* LOCKED sections require explicit ADR approval.

---

## Development Setup

### Quick Start

```bash
make dev
```

Runs:

* API (`cmd/api`)
* Worker (`cmd/worker`)
* Assumes infra already running

### Infrastructure (Local)

* Postgres + PostGIS
* Redis
* Pub/Sub (emulator if needed)

### Database Migrations

Schema changes live in `migrations/` and are executed via Goose through the `cmd/migrate` binary.

```bash
make migrate-up
```

The helper maps to `go run ./cmd/migrate up` but you can pass any Goose command (`down`, `status`, etc.) directly to the binary (`go run ./cmd/migrate status`).

API and workers auto-run migrations **only when**:

* `PACKFINDERZ_APP_ENV=dev`
* `PACKFINDERZ_AUTO_MIGRATE=true`

The startup path blocks on Goose failures in dev. In `prod` mode the auto-run path is disabled, so run `cmd/migrate` manually (local machine or CI job) ahead of deploying schema changes. Heroku deployments do not need—or want—a dedicated migration dyno; keep `cmd/migrate` as the manual tool instead.

### Postgres Extensions

The earliest migrations configure Postgres for the stack by enabling `pgcrypto` (UUID helpers) and `postgis` (geo queries). Once you run `make migrate-up`, verify both extensions are active with:

```sql
SELECT extname
FROM pg_extension
WHERE extname IN ('pgcrypto', 'postgis');
```

Re-running the migration is safe because the statements use `CREATE EXTENSION IF NOT EXISTS`.

---

## Core Components

### Tenancy & Identity

* Canonical tenant = **Store**
* Users may belong to multiple stores
* JWT includes `activeStoreId`, `role`, optional `store_type`/`kyc_status`, and standard `iat`/`exp`
* Tokens respect `PACKFINDERZ_JWT_EXPIRATION_MINUTES` and are refreshed when the store changes
* Refresh tokens are opaque, rotate on each exchange, and are stored in Redis under `pf:session:access:<jti>` whose TTL is governed by `PACKFINDERZ_REFRESH_TOKEN_TTL_MINUTES`

### Checkout & Orders

* Client cart → server `CartRecord`
 * Checkout creates:

  * N `VendorOrder`s (all sharing the canonical `checkout_group_id` recorded on the cart)
* Partial success allowed **across vendors**
* Inventory reserved atomically per line item
* Atomic inventory reservation helper (PF-079) conditionally updates `inventory_items.available_qty`/`reserved_qty` and returns per-line results so checkout can continue with other vendors even when a line item cannot be reserved.
* Checkout enforces every product's MOQ (Catalog `products.moq`) and now returns `422` plus a `violations` detail array when a line item falls short so clients can display the same failure reason.
* PF-080 describes the `internal/checkout/service` orchestrator that runs the transaction converting a `CartRecord` → `vendor_orders`, `order_line_items`, and `payment_intents`, reusing the cart's `checkout_group_id` as the grouping anchor so there is no dedicated `checkout_groups` table while still handling reservation-driven partial success semantics.
* Order data models (`vendor_orders`, `order_line_items`, `payment_intents`) persist the CartRecord snapshot before inventory/reservations run; the grouping ID (`checkout_group_id`) now lives on both `cart_records` and `vendor_orders` while the dedicated `checkout_groups` table has been removed, so each vendor order stays anchored to the canonical cart snapshot, and the cart also records the selected `payment_method`, `shipping_line`, and `converted_at` timestamp so the checkout decision remains auditable. Each vendor order gets its own `payment_intent` seeded with the checkout-selected payment method and that vendor total so payment tracking stays vendor-scoped. Checkout execution no longer recomputes unit pricing, discounts, or vendor totals—those values flow straight from the `cart_items`/`cart_vendor_groups` snapshot unless reservation failures reject individual line items.
* The checkout helpers (`internal/checkout/helpers`) group `CartItem`s by vendor and validate buyer/vendor eligibility (store type, state, subscription, MOQ) without hitting the database; vendor totals now flow directly from the persisted `cart_vendor_groups` snapshot so each vendor order mirrors the canonical cart quote instead of recomputing aggregates.
* `internal/checkout/reservation` runs the `inventory_items` conditional update (`available_qty >= qty`), increments `reserved_qty`, and returns per-line reservation results so checkout can report partial success; if a line item cannot be reserved it is marked `rejected` and a vendor with no successful reservations has its vendor order status flipped to `rejected` so the response clearly shows the failed vendor even though no order will proceed to fulfillment.
* `internal/checkout/service.go` orchestrates the checkout transaction, converts the `CartRecord` into `VendorOrder`s while capturing the confirmed shipping/payment selections, and marks the cart `converted` so downstream flows can read the canonical totals that came straight out of the cart snapshot.
* Buyer product listings/details only surface licensed, subscribed vendors whose state matches the buyer's `state` filter (see `pkg/visibility.EnsureVendorVisible` for the gating rules and 404/422 contract).
* Cart quotes expire after 15 minutes (`valid_until`) and the checkout service rejects any expired quote so the client must re-quote before attempting checkout again.
* Once a cart transitions to `converted`, its checkout response is replayed on future attempts instead of mutating the cart again, keeping conversion idempotent even when retries happen.

### Payments & Ledger

* MVP: **cash at delivery**
* `LedgerEvent` is append-only
* `ledger_events` table stores every money lifecycle row (`order_id`, `type`, `amount_cents`, `metadata`, `created_at`) with `(order_id, created_at)` and `(type, created_at)` indexes and an `ON DELETE RESTRICT` FK to `vendor_orders`.
* Each ledger row also stores `buyer_store_id`, `vendor_store_id`, and `actor_user_id` to let buyers, vendors, and agents/admins audit who produced the event.
* Admins can review payout-eligible orders via `GET /api/admin/v1/orders/payouts` and inspect each detail with `GET /api/admin/v1/orders/payouts/{orderId}` before confirming the payout; the `/api/admin` group omits the store context guard so an admin JWT may lack `activeStoreId`.
* Admins confirm payouts through `POST /api/admin/v1/orders/{orderId}/confirm-payout` (Idempotency-Key required); the flow records a `vendor_payout` ledger row, marks the payment intent as `paid` with `vendor_paid_at`, closes the order, and emits the `order_paid` outbox event so downstream consumers stay in sync.
* Payment lifecycle:
  `unpaid → settled → paid`

### Async & Eventing

* API writes business data + `OutboxEvent` in the same transaction and logs `event_id`, `event_type`, `aggregate_type`, and `aggregate_id` for each emission.
* Worker publishes to Pub/Sub and consumers use `pkg/eventing/idempotency.Manager` to enforce `pf:evt:processed:<consumer>:<event_id>` keys before executing side effects.
* Consumers **must be idempotent**; the TTL for idempotency keys is configurable via `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (default `720h`).
* License decisions emit `license_status_changed` events and the worker subscribes via `PACKFINDERZ_PUBSUB_DOMAIN_SUBSCRIPTION` so the compliance consumer can notify stores (verified/rejected) and admins (pending review) while honoring Redis idempotency.
* `cmd/analytics-worker` listens on `PACKFINDERZ_PUBSUB_ANALYTICS_SUBSCRIPTION`, deserializes the canonical analytics envelope, and gates duplicates with `pkg/outbox/idempotency.Manager` before routing to the analytics handlers.
* `internal/analytics/router` enforces canonical event routing and typed payload decoding so each handler stub receives the right DTO plus the shared BigQuery writer interface.
* `order_canceled`/`order_expired` handlers now append `marketplace_events` rows with the termination metadata payload so queries can exclude terminated orders while optionally surfacing cancellation reasons and TTLs.
* `order_paid` and `cash_collected` handlers now append `marketplace_events` rows with `gross_revenue_cents`/`net_revenue_cents` and the actual `occurred_at`, giving the analytics query service explicit events to drive revenue time series without relying on publish time.
* `internal/analytics/writer.BigQueryWriter` wraps `pkg/bigquery.Client`, normalizes JSON columns via `EncodeJSON`, retries transient BigQuery insert failures, and exposes optional batching for `marketplace_events`/`ad_event_facts`.

### Ads & Analytics

* Vendor visibility gated by:

  * license verified
  * subscription active
* Last-click attribution (30d)
* Cart quote attribution tokens are treated as JWTs; only tokens that pass signature and expiry validation are persisted or echoed and invalid tokens are silently ignored.
* BigQuery used for analytics only
* Canonical analytics DTOs (envelope, marketplace/ad rows, query requests/responses) live under `internal/analytics/types` while event enums live in `pkg/enums/analytics_event_type.go`/`pkg/enums/ad_event_fact_type.go`.
* Vendors and buyers can query KPIs/time-series via `GET /api/v1/vendor/analytics` (vendor-only route) or the new `GET /api/v1/analytics/marketplace` endpoint, both of which run parameterized BigQuery queries (presets 7d/30d/90d or custom `from`/`to`) against `marketplace_events` and return the canonical success envelope scoped to `activeStoreId`.
* Analytics ingestion uses `cmd/analytics-worker` powered by `PACKFINDERZ_PUBSUB_ANALYTICS_TOPIC`/`PACKFINDERZ_PUBSUB_ANALYTICS_SUBSCRIPTION`; the worker decodes the canonical analytics envelope and writes the `pf:evt:processed:analytics:<event_id>` guard via `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL`.
* Vendor subscription lifecycle is handled through `POST /api/v1/vendor/subscriptions` (create, idempotent), `POST /api/v1/vendor/subscriptions/cancel` (idempotent), and `GET /api/v1/vendor/subscriptions` (fetch the single active subscription or `null`). The POSTs require an `Idempotency-Key` and Square customer/payment method IDs; the API mirrors Square state into the local `subscriptions` table and flips `stores.subscription_active`.

---

## Storage Schema

### Postgres (Authoritative)

- Stores, users, memberships (store_memberships join + member_role/membership_status enums). Store relationships now resolve exclusively through `store_memberships` since PF-198 removed the legacy `users.store_ids` array.
* Products + `product_media` attachments (category/classification/unit/flavors/feelings/usage enums govern vendor listings)
* Volume discounts (`product_volume_discounts`) for deterministic tiered pricing per product
* Inventory (`inventory_items` tracks available/reserved counts per product), orders
* Cart staging tables (`cart_records`, `cart_items`, `cart_vendor_groups`) persist the authoritative quote (cart totals, vendor aggregates, item warnings) at checkout confirmation (status `active|converted`) before creating checkout groups
* Checkout tables (`vendor_orders`, `order_line_items`, `payment_intents`) capture the per-vendor order state, line items, and payment intent before checkout execution hands off to fulfillment while `checkout_group_id` remains the shared anchor stored on carts/orders.
* Payments, ledger events, and Square billing tables (`subscriptions`, `payment_methods`, `charges`, `usage_charges`)
* Ads, subscriptions
* Outbox events
* Audit logs
* Google Cloud Storage (pkg/storage/gcs) verified via `/health/ready`
  * Media metadata (`media` + `media_attachments`, which tie `entity_type`/`entity_id` to `store_id` and cache `gcs_key` so usage lookups stay tenant-scoped)
  * License uploads now persist a `media_attachments` row (`entity_type='license'`) so the referenced `media_kind=license_doc` asset stays protected while the license exists.
  * Product gallery media plus the single COA reference (`products.coa_media_id`) now call the canonical `internal/media.AttachmentReconciler` during create/update transactions (`entity_type='product_gallery'` / `product_coa`) so their attachments mirror the latest media IDs without cross-store leaks.
  * Store branding (logo/banner) calls the same reconciler with `entity_type='store_logo'` and `entity_type='store_banner'` so each store keeps exactly one attachment per usage and updates run inside the store transaction.
  * Attachment reconciliation happens through `internal/media.NewAttachmentReconciler`, which diffs usages inside a transaction and follows the lifecycle rules described in `docs/media_attachments_lifecycle.md`.
  * Lifecycle rules (protected attachments, deletion preconditions, and cleanup ordering) are detailed in `docs/media_attachments_lifecycle.md`.
  * `DELETE /api/v1/media/{mediaId}` loads `media_attachments`, rejects the request whenever a `license` or `ad` attachment exists, and deletes the GCS object once the guard passes so the delete-media worker sees the corresponding `OBJECT_DELETE` event.
  * The `cmd/media_deleted_worker` binary subscribes to `pubsub.MediaDeletionSubscription()` and executes `internal/media/consumer.DeletionConsumer` so every GCS `OBJECT_DELETE` event detaches attachments, deletes the media row, and logs each step after the API already enforced protection.

### Redis (Ephemeral)

* Idempotency keys
* Ad budget counters

### BigQuery

* Marketplace events
* Ad telemetry
* KPI rollups
* `internal/consumers/analytics.Consumer` (BigQuery + Redis idempotency) processes `order_created`, `cash_collected`, and `order_paid` outbox events so each analytics row is deduplicated via `pf:evt:processed:analytics:<event_id>` before writing to the configured `marketplace_events` table.
* Environment vars:
  * `PACKFINDERZ_BIGQUERY_DATASET` (default `packfinderz`)
  * `PACKFINDERZ_BIGQUERY_MARKETPLACE_TABLE` (default `marketplace_events`)
  * `PACKFINDERZ_BIGQUERY_AD_TABLE` (default `ad_events`)
* API and worker startup use `pkg/bigquery.NewClient` to verify the configured dataset and tables before processing so `/health/ready` and the worker dependency ping surface missing BigQuery infrastructure immediately.

---

## Testing & Quality

### Unit Tests

* Domain logic
* State transitions
* RBAC enforcement
* Inventory reservation

### Integration Tests

* Checkout end-to-end
* Outbox publishing
* Idempotency replay
* Ads budget gating

### Failure Tests

* Duplicate events
* Partial checkout
* Worker retries
* Scheduler idempotency

### Integration Harness

* `make test` runs the new `scripts/integration/run.sh` scaffold. Supply the route with `INTEGRATION_ARGS="--route <name>"` (e.g., `register`) so the harness knows which flow to prepare.
* Required environment variables (`API_BASE_URL`, `STORE_PASSWORD`) are validated up front and stay exported for downstream steps when the route implementations are added.
* `scripts/integration/http_client.sh` exposes a shared HTTP client with base URL handling, retries, timeouts, and JSON-safe helpers for GET/POST/PUT/DELETE. Source it in downstream scripts (like the register flow) for consistent behavior.
* The `register` route (`scripts/integration/register.sh`) calls `POST /api/v1/auth/register` twice with buyer and vendor flags, captures the store/user IDs plus access/refresh tokens, and emits a machine-readable JSON block (`{"buyer":…,"vendor":…}`) so later scripts can consume the credentials.
* The current scaffolding only verifies configuration; future routes will add additional HTTP calls/assertions.

### CI Pipeline

The GitHub Actions workflow (`.github/workflows/ci.yml`) runs gofmt, `golangci-lint`, `go test ./...`, `go build ./cmd/api ./cmd/worker ./cmd/migrate`, and a gitleaks secret scan on every pull request and push to `main`; branch protection should require the `CI` job to pass before merging. DB-dependent tests must use `//go:build db` so they stay excluded from this pipeline (run them locally with `go test -tags=db ./...` once infrastructure is ready), and any secrets caught by gitleaks fail the workflow.

---

## API Usage

### Idempotency

* Money-adjacent `POST` endpoints require an `Idempotency-Key` header; missing the header now yields a `400`.
* `api/middleware.Idempotency` stores the first response (status, body, and `Content-Type`) in Redis per scope+key and replays it on matching keys; mismatched request bodies trigger `409 IDEMPOTENCY_KEY_REUSED`.
* TTLs are 24h by default and 7 days for checkout/payment flows (see `DESIGN_DOC.md` section 6 for the complete endpoint list).
* `POST /api/v1/checkout` uses the idempotency middleware so the first successful response (checkout group + vendor orders) is cached for 7 days; duplicate calls with the same key/body replay that response, while a different payload triggers `409 IDEMPOTENCY_KEY_REUSED`, preventing double reservations.

### Checkout Submission

* `POST /api/v1/checkout` finalizes the buyer store's active cart within a single transaction, splitting it into per-vendor `VendorOrders` that share the cart's `checkout_group_id`.
* Requires a `Idempotency-Key` header (7-day TTL) and a buyer store context; the request body must include `cart_id`, `shipping_address`, and `payment_method`, with an optional `shipping_line` so the API can confirm or override the cart’s pending shipment selection.
* The request is rejected before mutating state if the cart is missing, already converted, or contains no `cart_items` with `status=ok`, so callers receive deterministic errors and can rebuild the quote before retrying. Once checkout succeeds the service writes the confirmed shipping metadata plus `payment_method`/`converted_at` to `cart_records`, flips `status` to `converted`, and persists the shared `checkout_group_id` so the cart remains the canonical anchor for downstream orders.
* Success returns `201` and the canonical `vendor_orders` payload grouped by vendor plus `rejected_vendors`, explicitly listing any vendors/line items that were rejected (each line item surfaces `status`/`notes` so clients can show the failure reason). Even if a vendor has no eligible cart items, the `rejected_vendors` array now includes a warning so clients can show the vendor-level rejection reason alongside the confirmed `shipping_address`, `payment_method`, and `shipping_line` that also appear in the response.
* Errors: `400` (validation), `403` (vendor store or missing store context), `409` (`Idempotency-Key` reused with a different body), `422` (state conflict such as MOQ or reservation failures).
* `GET /api/v1/checkout/{identifier}/confirmation` fetches the checkout result identified by either `checkout_group_id` or `cart_id` so buyers can poll for the latest vendor order statuses after checkout. The response includes each vendor order’s status/payment intent/assignment plus the cached `cart_vendor_groups`; requires the buyer store context and returns `404` if the identifier is unknown.

### Cart Upsert

* `PUT /api/v1/cart` – buyer stores use this idempotent endpoint (24h TTL) to persist their cart snapshot once checkout confirmation occurs.
* Server-side validations re-check buyer/vendor KYC, subscriptions, inventory, MOQ, volume tiers, and computed totals before creating/updating the `cart_record` + `cart_items` rows so the checkout runner always consumes a trusted snapshot.
* Requires `Idempotency-Key`; returns the stored record with its line items so the UI can recover or retry.
* Vendor gating now reuses `internal/checkout/helpers.ValidateVendorStore`, which delegates to `pkg/visibility.EnsureVendorVisible`, so any `subscription_active=false` or cross-state vendor is rejected before the cart is saved.

### Cart Fetch

* `GET /api/v1/cart` – returns the buyer store's currently active `cart_record` along with its `cart_items` so the UI can recover or refresh the pending checkout.
* Enforces the active store context, verifies the buyer store's ownership, and returns `404` when no active cart exists.

### Orders

* `GET /api/v1/orders` – cursor-paginated orders scoped to the active store's perspective (`buyer_store_id` for buyers, `vendor_store_id` for vendors).
  * Accepts `limit` (default 25, max 100) plus `cursor` for pagination, `q` for a global name search, `order_status`, `fulfillment_status`, `shipping_status`, `payment_status`, and RFC 3339 `date_from`/`date_to` filters. Vendor stores may also pass `actionable_statuses=created_pending,accepted` (comma-separated statuses).
  * Returns `BuyerOrderList` or `VendorOrderList` data with totals, discount/fee metadata, `payment_status`, `fulfillment_status`, `shipping_status`, `total_items`, and the peer store summary.
  * `403` when the active store is missing from the JWT/store context.

* `GET /api/v1/orders/{orderId}` – returns the full `OrderDetail` (order summary, buyer/vendor store metadata, line items, payment intent info, and the active agent assignment if present).
* Buyer stores only see orders where they are the buyer; vendor stores only see their vendor orders.
* `403` when the order doesn't belong to the active store, `404` when the `orderId` cannot be found.
* `POST /api/v1/orders/{orderId}/cancel` – buyer cancel (pre-transit) releases unreleased inventory, zeros the balance due, and emits the `order_canceled` event for downstream notifications.
* `POST /api/v1/orders/{orderId}/nudge` – buyer nudges the vendor (idempotent) and emits a `notification_requested` event so email/alert systems can react.
* `POST /api/v1/orders/{orderId}/retry` – only expired orders can be retried; the service reuses the order snapshot for that vendor, re-creates the vendor order/line items, reserves inventory, and emits `order_retried` with the new order ID while returning `201`.

### Vendor Decisions

* `POST /api/v1/vendor/orders/{orderId}/decision` – the vendor acknowledges or rejects an order at the order level.
  * Requires a vendor store context and body `{ "decision": "accept" | "reject" }`.
  * A successful accept transitions the order status to `accepted`; a reject sets it to `rejected`.
  * The endpoint is idempotent via `Idempotency-Key`, and it emits the `order_decided` outbox event so the buyer can be notified of the vendor's acknowledgment.
* `POST /api/v1/vendor/orders/{orderId}/line-items/decision` – the vendor resolves an individual line item (`line_item_id`, `decision`: `fulfill|reject`, optional `notes`).
  * Rejects release inventory (idempotently) and all decisions recompute `balance_due_cents`, update fulfillment/shipping readiness, move the order into `ready_for_dispatch`, and emit the new `order_ready_for_dispatch` outbox event once no pending line items remain.

### Vendor Billing History

* `GET /api/v1/vendor/billing/charges` – vendor-only endpoint that streams the local `charges` rows in cursor order. Requires the vendor store context, accepts optional `limit` (positive integer, default 25, max 100), `cursor` (`created_at|id` base64 token), `type` (`subscription`|`ad_spend`|`other`), and `status` (`pending`|`succeeded`|`failed`|`refunded`) filters, and returns `charges[]` plus a `cursor` for the next page. Each charge exposes `id`, `amount_cents`, `currency`, `type`, `status`, `description`, `created_at`, and `billed_at`, so the UI mirrors provider/local history without calling the billing provider per request.

### Health

```bash
GET /health
GET /health/ready
```

### API Versioning

* All endpoints under `/api/v1`
* Breaking changes require `/api/v2`

### Authentication

#### Register

```
POST /api/v1/auth/register
```

Creates the initial user + store + owner membership bundle in one transaction. If the email already exists and the provided password matches that account, the endpoint creates a new store + membership for the existing user instead of inserting another user row. Provide `first_name`, `last_name`, `email`, `password`, `company_name`, `store_type`, an `address` object (including `lat`/`lng`), and `accept_tos: true`. Returns `201`, issues access + refresh tokens, and mirrors the newest access token in `X-PF-Token`.

#### Login

```
POST /api/v1/auth/login
```

Validates email/password, collects the store memberships, and returns `200` with tokens plus `stores[]` (for multi-store selection). Each response also sets `X-PF-Token` to the latest access token.

#### Admin Login

```
POST /api/admin/v1/auth/login
```

Builds a storeless admin session (`role=admin`, `activeStoreId` omitted) for `users.system_role="admin"`. Returns HTTP 200 with a `refresh_token`, the admin `user` DTO, and the `X-PF-Token` header containing the access token that can be used against `/api/admin/*`. Invalid credentials yield the same 401 response as the regular login route.

#### Admin Register (dev-only)

```
POST /api/admin/v1/auth/register
```

Available only when `PACKFINDERZ_APP_ENV != prod`, this endpoint seeds a `system_role="admin"` user, hashes the password via `security.HashPassword`, returns the admin `user` DTO plus `access_token`/`refresh_token`, and mirrors the access token in `X-PF-Token` so dev tooling can immediately hit `/api/admin/*`. Duplicate emails return `409 Conflict`.

#### Logout

```
POST /api/v1/auth/logout
```

Requires an Authorization bearer token; revokes the refresh mapping so the session cannot be renewed. Returns `200` when the session is terminated.

#### Refresh

```
POST /api/v1/auth/refresh
```

### Vendor Products

* `POST /api/v1/vendor/products` – vendor stores create listings inside the authenticated `/api` surface with a valid `Idempotency-Key`. The request body carries the SKU/title/unit/category/feelings/flavors/usage metadata, `inventory` object (with `available_qty` and optional `reserved_qty`), optional `media_ids` array of `media` UUIDs, and optional `volume_discounts` array (`min_qty`, `discount_percent`). The handler validates the active store is a vendor, enforces membership roles, writes the product + inventory + discounts + product media rows in one transaction, and returns the canonical product payload (including inventory, discounts, media, and vendor summary) on success. Each returned media object now includes `media_id` so clients can correlate the attachment with the original `media` row.
* `PATCH /api/v1/vendor/products/{productId}` – vendors may update mutable metadata, pricing, inventory counts, volume discounts, and attached media IDs for an existing product owned by the active store. Requests are validated via `api/controllers/products.VendorUpdateProduct`, which reuses `internal/products.Service.UpdateProduct` to enforce vendor ownership/roles, inventory/reserved invariants, unique discount thresholds, and valid media rows before synchronously updating the product, inventory, discounts, and media attachments and returning the updated product DTO. Authorization/validation failures follow the canonical error envelope.
* `DELETE /api/v1/vendor/products/{productId}` – removes the specified product owned by the active vendor store and relies on FK cascades to clean up inventory, discounts, and media attachments. `api/controllers/products.VendorDeleteProduct` parses the path, enforces store/user context, and delegates to `internal/products.Service.DeleteProduct`, which ensures ownership/role validation and returns `204` with no body when the row is gone.
* Products now expose `max_qty` (per line limit) plus `inventory.low_stock_threshold` so the service validates non-negative constraints and the internal inventory rows record the threshold for operational tooling.
* `GET /api/v1/vendor/products` – vendor-only table query that returns cursor-paginated `ProductSummary` rows (`id`, `sku`, `title`, `category`, `classification`, `price_cents`, `compare_at_price_cents`, `thc_percent`, `cbd_percent`, `has_promo`, `created_at`, `updated_at`). The call accepts `limit`, `cursor`, `category`, `classification`, `price_min_cents`, `price_max_cents`, `thc_min`, `thc_max`, `cbd_min`, `cbd_max`, `has_promo`, and `q` (title/sku search). Results are scoped to the active vendor store and work even when the `state` query is omitted or differs, letting the internal product table filter and page the catalog without buyer restrictions.

### Product Browse

* `GET /api/v1/products` – buyer and vendor stores hit a cursor-paginated catalog endpoint that returns lightweight `ProductSummary` rows (`id`, `sku`, `title`, `category`, `classification`, `price_cents`, `compare_at_price_cents`, `thc_percent`, `cbd_percent`, `has_promo`, `vendor_store_id`, `created_at`, `updated_at`). The handler accepts `limit`, `cursor`, `state` (required for buyers and must match the buyer’s own state), `category`, `classification`, `price_min_cents`, `price_max_cents`, `thc_min`, `thc_max`, `cbd_min`, `cbd_max`, `has_promo`, and `q` (title/sku search). Buyer stores only see `is_active=true` products from verified vendors with `subscription_active=true` whose `address.state` equals the requested state, while vendor stores always view their own store’s listings even if the state filter differs so they can manage the catalog.

Accepts a JSON body with `refresh_token` and the outgoing access token in the Authorization header (even if expired). Returns `200` with a rotated refresh token plus a new access token set in both the response body and `X-PF-Token`.

#### Switch Store

```
POST /api/v1/auth/switch-store
```

Requires both the current Authorization bearer token and the existing `refresh_token`. Validates that the user belongs to `store_id`, rotates the session, and returns `200` with a new access token (in the body + `X-PF-Token`) preconfigured with `activeStoreId`.

### Store Management

These endpoints rely on `activeStoreId` and enforce owner/manager access for mutating flows.

* `GET /api/v1/stores/me` – returns the requested store’s profile for the active store.
* `PUT /api/v1/stores/me` – updates mutable store metadata (description, phone, email, social links, banner/logo URLs, ratings, categories) while keeping address and geo locked until an admin override exists.
* `GET /api/v1/stores/me/users` – lists memberships plus user info (`email`, `name`, `role`, `created_at`, `last_login_at`); accessible to owner/manager roles.
* `POST /api/v1/stores/me/users/invite` – invites (or reuses) a user, creates a membership, and issues a temporary password for new accounts (passwords are never logged).
* `DELETE /api/v1/stores/me/users/{userId}` – removes only the membership row, returns `409` if the target is the last owner, and leaves the user record intact.

### Licenses

* `POST /api/v1/licenses` – upload license metadata (media_id, issuing_state, type, number, optional issue/expiration dates). Requires owner/manager access for the active store and enforces `Idempotency-Key` to avoid duplicate uploads.
* `GET /api/v1/licenses` – lists the active store’s licenses with cursor pagination. Responses include signed download URLs from GCS.
* `DELETE /api/v1/licenses/{licenseId}` – removes expired or rejected licenses owned by the active store. Only `manager`/`owner` roles may call this, the row must be `expired`/`rejected`, and the store is downgraded to `pending_verification` if no `verified` licenses remain. Media rows stay untouched when the license is deleted.
* `POST /api/v1/admin/licenses/{licenseId}/verify` – admin-only endpoint that approves (`verified`) or rejects (`rejected`) a pending license. The request accepts `{decision, reason?}`, enforces `Idempotency-Key`, and returns the updated license row; invalid or already-finalized licenses yield `409`.

### Media Uploads

* `POST /api/v1/media/presign` – creates a `media` row in `pending` state, assigns a deterministic `gcs_key` following the `{store_id}/{media_kind}/{media_id}.{extension}` pattern (extension included when the upload filename provides one), and returns `{media_id, gcs_key, signed_put_url, content_type, expires_at}` for clients to PUT directly to GCS.
  * Requires `activeStoreId` + store role (owner/admin/manager/staff/ops), `Idempotency-Key`, and a sanitized `file_name`.
  * Validates `media_kind`, `mime_type`, and `size_bytes ≤ 20MB`; the signed URL enforces the supplied `Content-Type`.
  * TTL honors `PACKFINDERZ_GCS_UPLOAD_URL_EXPIRY`, and clients must not proxy uploads through the API (use the signed PUT directly).
* `GET /api/v1/media` – lists media owned by `activeStoreId`, returning metadata only (`id`, `kind`, `status`, `file_name`, `mime_type`, `size_bytes`, `created_at`, `uploaded_at`). Supports filters (`kind`, `status`, `mime_type`, `search`) and cursor pagination (`limit` + `cursor`).
* Signed READ URLs for `uploaded`/`ready` media are generated via the media service helper and expire according to `PACKFINDERZ_GCS_DOWNLOAD_URL_EXPIRY`.
* `DELETE /api/v1/media/{mediaId}` – removes media whose status is `uploaded`/`ready`, deletes the GCS object (ignores missing objects), and marks the row as `deleted`; rejects mismatched stores or invalid states with `403`/`409`.

### Square Webhooks

* `POST /api/v1/webhooks/square` – consumes Square `subscription.*` and `invoice.*` events. The handler verifies the `Square-Signature` header using `PACKFINDERZ_SQUARE_WEBHOOK_SECRET`, deduplicates deliveries via a Redis guard keyed by `event.id` (TTL=`PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL`), and keeps `subscriptions.status` plus `stores.subscription_active` aligned with Square truth.

### Error Contract

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": {}
  }
}
```

---

## Configuration

### Core Environment Variables

```bash
PACKFINDERZ_APP_ENV=development
PACKFINDERZ_APP_PORT=8080

PACKFINDERZ_DB_DSN=postgres://...
PACKFINDERZ_DB_DRIVER=postgres

REDIS_ADDR=localhost:6379

PACKFINDERZ_LOG_LEVEL=info
PACKFINDERZ_LOG_WARN_STACK=false
PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL=720h
```

> **Rule:** Do not add new env vars without documentation.

### Eventing Idempotency

`PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (default `720h`) controls how long the Redis key `pf:evt:processed:<consumer>:<event_id>` stays locked after a consumer first handles an Outbox event. Workers should wire `pkg/eventing/idempotency.Manager` with this TTL so retries do not re-run side effects.

### ACH Payment Gate

* `PACKFINDERZ_FEATURE_ALLOW_ACH` (default `false`) controls whether checkout accepts `payment_method=ach`. When enabled, each vendor order seeds its `payment_intents` row with `method=ach` and `status=pending` so downstream ACH pipelines see the intended transaction; when disabled, ACH requests return a validation error and buyers must use `cash`. Payment intents still honor `amount_cents = vendor_orders.total_cents`, and future ACH work can move `payment_status` into the new `failed`/`rejected` values when transactions are declined.

### Square

* `PACKFINDERZ_SQUARE_ACCESS_TOKEN` (required) – the Square access token used for API calls.
* `PACKFINDERZ_SQUARE_WEBHOOK_SECRET` (required) – the webhook signing secret used to verify Square events.
* `PACKFINDERZ_SQUARE_ENV` (default `sandbox`) – selects between sandbox/production Square hosts and enforces the matching token conventions. The API and worker boot fail fast when tokens are missing or invalid so misconfigured environments surface immediately.
* `PACKFINDERZ_SQUARE_SUBSCRIPTION_PLAN_ID` (required) – the Square plan variation ID used when creating vendor subscriptions.

### Outbox Publisher Tuning

These knobs control the publisher worker that reads `outbox_events` and pushes domain envelopes to Pub/Sub (see `docs/outbox.md`).

* `PACKFINDERZ_OUTBOX_PUBLISH_BATCH_SIZE` (default `50`) – how many rows to claim in each fetch.
* `PACKFINDERZ_OUTBOX_PUBLISH_POLL_MS` (default `500`) – base sleep when no rows are claimed; applies between healthy loops.
* `PACKFINDERZ_OUTBOX_MAX_ATTEMPTS` (default `10`) – stop claiming rows once they hit this attempt count so failing rows can be audited.
* `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC` (default `pf-domain-events`) – the Pub/Sub topic that the worker publishes to; events flow through this topic plus the `event_type` attribute.
* `PACKFINDERZ_PUBSUB_DOMAIN_SUBSCRIPTION` (required) – the subscription the worker listens to for domain events such as `license_status_changed`.

### Analytics Worker

* `PACKFINDERZ_PUBSUB_ANALYTICS_TOPIC` (required) – the Pub/Sub topic that feeds analytics events into the pipeline.
* `PACKFINDERZ_PUBSUB_ANALYTICS_SUBSCRIPTION` (required) – the subscription consumed by `cmd/analytics-worker` which decodes the canonical envelope and honors the `pf:evt:processed:analytics:<event_id>` guard.

### Auth Rate Limiting

Configurable Redis-backed throttles protect the login/register endpoints from brute-force attacks.

* `PACKFINDERZ_AUTH_RATE_LIMIT_LOGIN_WINDOW` (default `1m`) – fixed window duration for login counters.
* `PACKFINDERZ_AUTH_RATE_LIMIT_LOGIN_IP_LIMIT` (default `20`) – max login attempts per IP per window.
* `PACKFINDERZ_AUTH_RATE_LIMIT_LOGIN_EMAIL_LIMIT` (default `5`) – max login attempts per normalized email per window.
* `PACKFINDERZ_AUTH_RATE_LIMIT_REGISTER_WINDOW` (default `5m`) – fixed window duration for register counters.
* `PACKFINDERZ_AUTH_RATE_LIMIT_REGISTER_IP_LIMIT` (default `20`) – max register attempts per IP per window.
* `PACKFINDERZ_AUTH_RATE_LIMIT_REGISTER_EMAIL_LIMIT` (default `3`) – max register attempts per normalized email per window.

Redis keys follow `rl:ip:<policy>:<ip>` and `rl:email:<policy>:<hash>` so each policy keeps its own bucket.

### Password Hashing Configuration

Argon2id parameters are configurable so production can tune memory/time while defaults remain safe for local development.

* `PACKFINDERZ_ARGON_MEMORY_KB` (default `65536`)
* `PACKFINDERZ_ARGON_TIME` (default `3`)
* `PACKFINDERZ_ARGON_PARALLELISM` (default `2`)
* `PACKFINDERZ_ARGON_SALT_LEN` (default `16`)
* `PACKFINDERZ_ARGON_KEY_LEN` (default `32`)

---

## Development Workflow

### First-Time Setup

1. Start infra
2. Run migrations
3. Seed minimal data
4. Run `make dev`
5. Verify `/health/ready`

### Daily Workflow

1. Pull latest
2. Run migrations
3. Run tests
4. Make additive changes
5. Verify RBAC + idempotency
6. Update docs if behavior changes

---

## Safety Boundaries

### MUST NOT

* Log passwords, tokens, license docs
* Mutate ledger rows
* Bypass RBAC or `activeStoreId`
* Add RedisTimeSeries
* Introduce new storage without ADR

### MUST

* Hash passwords (Argon2id)
* Use idempotency for money actions
* Append-only audit logs
* Signed URLs for media access

---

## Documentation

Additional docs live in `docs/`:

* Architecture overview
* Data design
* Security & ops
* ADRs
* Runbooks
* Outbox publisher guidance (`docs/outbox.md`)

**AGENTS.md** is authoritative for repo-aware edits.

---

## Makefile Reference

### Development

```makefile
make dev        # Run API + worker
make api        # Run API only
make worker     # Run worker only
make run-outbox-publisher  # Run new outbox publisher worker
make test       # Run the integration harness (set INTEGRATION_ARGS="--route <route>")
```

### Behavior

* API and worker run concurrently
* Graceful shutdown via SIGINT
* Go tooling only (no wrappers)

---

## Quick Reference

```bash
make dev                # Run API + worker
go test ./...           # Run tests
make run-outbox-publisher  # Run outbox publisher
make test INTEGRATION_ARGS="--route login"  # Integration harness scaffold (requires API_BASE_URL + STORE_PASSWORD)
```
