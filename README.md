

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

* Multi-vendor checkout via **CheckoutGroup**
* Atomic inventory reservation (optimistic + retry)
* Vendor accept/reject at **order and line-item level**
* Internal agent delivery with **cash-at-delivery**
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

### Outbox Publisher

`cmd/outbox-publisher` is the dedicated outbox publisher that polls `outbox_events` with `FOR UPDATE SKIP LOCKED`, publishes domain envelopes to `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`, and marks `published_at` or increments `attempt_count`/`last_error` before committing the transaction. Run it locally with `make run-outbox-publisher`, build it via `make build-outbox-publisher`, and run the `outbox-publisher` dyno in Heroku alongside `api`/`worker`. The operational guide in `docs/outbox.md` explains claiming, at-least-once semantics, retry expectations, and consumer idempotency requirements. When the API completes checkout it writes an `order_created` outbox row (payload includes `checkout_group_id` plus the `vendor_order_ids`) so analytics and notification consumers can react to the same transactional split recorded in Postgres.

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

  * `CheckoutGroup`
  * N `VendorOrder`s
* Partial success allowed **across vendors**
* Inventory reserved atomically per line item
* Atomic inventory reservation helper (PF-079) conditionally updates `inventory_items.available_qty`/`reserved_qty` and returns per-line results so checkout can continue with other vendors even when a line item cannot be reserved.
* Checkout enforces every product's MOQ (Catalog `products.moq`) and now returns `422` plus a `violations` detail array when a line item falls short so clients can display the same failure reason.
* PF-080 describes the `internal/checkout/service` orchestrator that runs the transaction converting a `CartRecord` → `CheckoutGroup` + `VendorOrders` + `OrderLineItems` while handling reservation-driven partial success semantics.
* Order data models (`checkout_groups`, `vendor_orders`, `order_line_items`, `payment_intents`) persist the CartRecord snapshot before inventory/reservations run; these tables (PF-077) back the checkout group/vendor order abstractions.
* The checkout helpers (`internal/checkout/helpers`) provide deterministic grouping, totals recomputation, and buyer/vendor validation logic that the orchestration layer reuses without hitting the database.
* `internal/checkout/reservation` runs the `inventory_items` conditional update (`available_qty >= qty`), increments `reserved_qty`, and returns per-line reservation results so checkout can report partial success.
* `internal/checkout/service.go` orchestrates the checkout transaction, creates the `CheckoutGroup` + `VendorOrder`s, converts `CartRecord` to orders, and marks the cart `converted` while reusing the helpers/reservation logic.
* Buyer product listings/details only surface licensed, subscribed vendors whose state matches the buyer's `state` filter (see `pkg/visibility.EnsureVendorVisible` for the gating rules and 404/422 contract).

### Payments & Ledger

* MVP: **cash at delivery**
* `LedgerEvent` is append-only
* Payment lifecycle:
  `unpaid → settled → paid`

### Async & Eventing

* API writes business data + `OutboxEvent` in the same transaction and logs `event_id`, `event_type`, `aggregate_type`, and `aggregate_id` for each emission.
* Worker publishes to Pub/Sub and consumers use `pkg/eventing/idempotency.Manager` to enforce `pf:evt:processed:<consumer>:<event_id>` keys before executing side effects.
* Consumers **must be idempotent**; the TTL for idempotency keys is configurable via `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (default `720h`).
* License decisions emit `license_status_changed` events and the worker subscribes via `PACKFINDERZ_PUBSUB_DOMAIN_SUBSCRIPTION` so the compliance consumer can notify stores (verified/rejected) and admins (pending review) while honoring Redis idempotency.

### Ads & Analytics

* Vendor visibility gated by:

  * license verified
  * subscription active
* Last-click attribution (30d)
* BigQuery used for analytics only

---

## Storage Schema

### Postgres (Authoritative)

* Stores, users, memberships (store_memberships join + member_role/membership_status enums)
* Products + `product_media` attachments (category/classification/unit/flavors/feelings/usage enums govern vendor listings)
* Volume discounts (`product_volume_discounts`) for deterministic tiered pricing per product
* Inventory (`inventory_items` tracks available/reserved counts per product), orders
* Cart staging tables (`cart_records`, `cart_items`) persist buyer snapshots at checkout confirmation (status `active|converted`) before creating checkout groups
* Checkout tables (`checkout_groups`, `vendor_orders`, `order_line_items`, `payment_intents`) capture the per-vendor order state, line items, and payment intent before checkout execution hands off to fulfillment
* Payments, ledger events
* Ads, subscriptions
* Outbox events
* Audit logs
* Google Cloud Storage (pkg/storage/gcs) verified via `/health/ready`
* Media metadata (`media` + `media_attachments`)

### Redis (Ephemeral)

* Idempotency keys
* Ad budget counters

### BigQuery

* Marketplace events
* Ad telemetry
* KPI rollups

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

* `POST /api/v1/checkout` finalizes the buyer store's active cart within a single transaction, splitting it into `CheckoutGroup` + per-vendor `VendorOrders`.
* Requires a `Idempotency-Key` header (7-day TTL) and a buyer store context; the request body must include `cart_id` and may include `attributed_ad_click_id`.
* Success returns `201` and the canonical `vendor_orders` payload grouped by vendor plus `rejected_vendors`, explicitly listing any vendors/line items that were rejected (each line item surfaces `status`/`notes` so clients can show the failure reason).
* Errors: `400` (validation), `403` (vendor store or missing store context), `409` (`Idempotency-Key` reused with a different body), `422` (state conflict such as MOQ or reservation failures).

### Cart Upsert

* `PUT /api/v1/cart` – buyer stores use this idempotent endpoint (24h TTL) to persist their cart snapshot once checkout confirmation occurs.
* Server-side validations re-check buyer/vendor KYC, subscriptions, inventory, MOQ, volume tiers, and computed totals before creating/updating the `cart_record` + `cart_items` rows so the checkout runner always consumes a trusted snapshot.
* Requires `Idempotency-Key`; returns the stored record with its line items so the UI can recover or retry.

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
  * Rejects release inventory (idempotently) and all decisions recompute `balance_due_cents`, update fulfillment/shipping readiness, and emit the new `order_fulfilled` outbox event once no pending line items remain.

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

Creates the initial user + store + owner membership in one transaction. Provide `first_name`, `last_name`, `email`, `password`, `company_name`, `store_type`, an `address` object (including `lat`/`lng`), and `accept_tos: true`. Returns `201`, issues access + refresh tokens, and mirrors the newest access token in `X-PF-Token`.

#### Login

```
POST /api/v1/auth/login
```

Validates email/password, collects the store memberships, and returns `200` with tokens plus `stores[]` (for multi-store selection). Each response also sets `X-PF-Token` to the latest access token.

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

* `POST /api/v1/vendor/products` – vendor stores create listings inside the authenticated `/api` surface with a valid `Idempotency-Key`. The request body carries the SKU/title/unit/category/feelings/flavors/usage metadata, `inventory` object (with `available_qty` and optional `reserved_qty`), optional `media_ids` array of `media` UUIDs, and optional `volume_discounts` array (`min_qty`, `unit_price_cents`). The handler validates the active store is a vendor, enforces membership roles, writes the product + inventory + discounts + product media rows in one transaction, and returns the canonical product payload (including inventory, discounts, media, and vendor summary) on success.
* `PATCH /api/v1/vendor/products/{productId}` – vendors may update mutable metadata, pricing, inventory counts, volume discounts, and attached media IDs for an existing product owned by the active store. Requests are validated via `api/controllers/products.VendorUpdateProduct`, which reuses `internal/products.Service.UpdateProduct` to enforce vendor ownership/roles, inventory/reserved invariants, unique discount thresholds, and valid media rows before synchronously updating the product, inventory, discounts, and media attachments and returning the updated product DTO. Authorization/validation failures follow the canonical error envelope.
* `DELETE /api/v1/vendor/products/{productId}` – removes the specified product owned by the active vendor store and relies on FK cascades to clean up inventory, discounts, and media attachments. `api/controllers/products.VendorDeleteProduct` parses the path, enforces store/user context, and delegates to `internal/products.Service.DeleteProduct`, which ensures ownership/role validation and returns `204` with no body when the row is gone.

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

* `POST /api/v1/media/presign` – creates a `media` row in `pending` state, computes a deterministic `gcs_key` containing the newly minted `media_id`, and returns `{media_id, gcs_key, signed_put_url, content_type, expires_at}` for clients to PUT directly to GCS.
  * Requires `activeStoreId` + store role (owner/admin/manager/staff/ops), `Idempotency-Key`, and a sanitized `file_name`.
  * Validates `media_kind`, `mime_type`, and `size_bytes ≤ 20MB`; the signed URL enforces the supplied `Content-Type`.
  * TTL honors `PACKFINDERZ_GCS_UPLOAD_URL_EXPIRY`, and clients must not proxy uploads through the API (use the signed PUT directly).
* `GET /api/v1/media` – lists media owned by `activeStoreId`, returning metadata only (`id`, `kind`, `status`, `file_name`, `mime_type`, `size_bytes`, `created_at`, `uploaded_at`). Supports filters (`kind`, `status`, `mime_type`, `search`) and cursor pagination (`limit` + `cursor`).
* Signed READ URLs for `uploaded`/`ready` media are generated via the media service helper and expire according to `PACKFINDERZ_GCS_DOWNLOAD_URL_EXPIRY`.
* `DELETE /api/v1/media/{mediaId}` – removes media whose status is `uploaded`/`ready`, deletes the GCS object (ignores missing objects), and marks the row as `deleted`; rejects mismatched stores or invalid states with `403`/`409`.

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

### Outbox Publisher Tuning

These knobs control the publisher worker that reads `outbox_events` and pushes domain envelopes to Pub/Sub (see `docs/outbox.md`).

* `PACKFINDERZ_OUTBOX_PUBLISH_BATCH_SIZE` (default `50`) – how many rows to claim in each fetch.
* `PACKFINDERZ_OUTBOX_PUBLISH_POLL_MS` (default `500`) – base sleep when no rows are claimed; applies between healthy loops.
* `PACKFINDERZ_OUTBOX_MAX_ATTEMPTS` (default `25`) – stop claiming rows once they hit this attempt count so failing rows can be audited.
* `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC` (default `pf-domain-events`) – the Pub/Sub topic that the worker publishes to; events flow through this topic plus the `event_type` attribute.
* `PACKFINDERZ_PUBSUB_DOMAIN_SUBSCRIPTION` (required) – the subscription the worker listens to for domain events such as `license_status_changed`.

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
```
