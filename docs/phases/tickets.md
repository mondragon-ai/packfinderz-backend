**Source of truth used:**

* Your `git log --oneline`
* Your locked Master Context + Architecture docs

# PHASED IMPLEMENTATION — REALITY-BASED

---

## **Phase 0 — Repo, Infra, CI, Foundations** ✅ DONE

**Goal:** A deployable, observable, production-grade Go monolith + workers

**Completed (from commits):**

* [x] Repo initialization & layout (`cmd/api`, `cmd/worker`, `pkg`, `internal`)
* [x] Config + env loading
* [x] Structured JSON logger with request/job correlation
* [x] Canonical error codes + API envelopes
* [x] Chi router + middleware stack
* [x] Standard request validation layer
* [x] Postgres bootstrap (Cloud SQL + Heroku compatible)
* [x] Redis bootstrap (sessions, rate limits, idempotency)
* [x] Goose migrations runner + hybrid policy
* [x] Dockerfile + Heroku Procfile (web + worker)
* [x] GitHub Actions CI pipeline (lint, test, build)
* [x] Worker bootstrap wiring (full dependency graph)

➡️ **Phase is complete and locked.**

---

## **Phase 1 — Identity, Auth, Tenancy, RBAC** ✅ DONE

**Goal:** Secure multi-store authentication with session revocation

**Completed:**

* [x] User model + Argon2id password hashing
* [x] Store model with address shape
* [x] Store membership model + role enum
* [x] JWT minting + parsing (claims enforced)
* [x] Refresh token storage + rotation (Redis)
* [x] Login endpoint (email/password)
* [x] Register endpoint (user + first store + owner)
* [x] Logout + refresh endpoints
* [x] Active store switching (JWT refresh)
* [x] Login brute-force rate limiting
* [x] Store profile read/update endpoints
* [x] Store user list / invite / remove endpoints

➡️ **Auth + tenancy is production-grade and DONE.**

**REMAINING (small, real hardening):**

* [ ] Remove/retire deprecated token parser (`api/validators/token.go`) everywhere (per doc note)
* [ ] Add auth middleware tests: missing/expired token, revoked session, missing activeStoreId
* [ ] Add RBAC guard tests for `/api/admin/*` and `/api/v1/agent/*`

---

## **Phase 2 — Media System (Canonical, Enforced)** ✅ DONE

**Goal:** Single, reusable media pipeline for all domains

**Completed:**

* [x] Media table + enums + lifecycle states
* [x] Canonical metadata persistence
* [x] GCS client bootstrap (API + worker)
* [x] Presigned PUT upload flow (create media row)
* [x] Pub/Sub consumer for GCS `OBJECT_FINALIZE`
* [x] Media status transitions (`pending → uploaded`)
* [x] Media list endpoint (store-scoped, paginated)
* [x] Media delete endpoint (reference-aware)
* [x] Presigned READ URL generation (TTL-based)

➡️ **Media is correctly centralized and enforced.**

**REMAINING (only if you truly need it):**

* [ ] Optional: add “delete_requested → deleted” async delete path (if you want delete to be worker-safe)
* [ ] Add media lifecycle GC scheduler: stale `pending` rows cleanup (no GCS object)

---

## **Phase 3 — Compliance & Licensing (OMMA Core)**  ✅ DONE

**Goal:** Store-level compliance gating via licenses

**Completed:**

* [x] License model + migration (media_id required)
* [x] License create endpoint (atomic metadata + media_id)
* [x] License list endpoint (store-scoped)

**Remaining (not yet implemented):**

* [x] License delete endpoint (status + attachment safety)
* [x] Admin approve/reject license endpoint
* [x] Mirror license status → store KYC status
* [x] Emit license status outbox events
* [x] License expiry scheduler (14-day warn, expire)
* [x] Compliance notifications (store + admin)

➡️ **Creation path is correct; enforcement + lifecycle remain.**


**REMAINING (cleanup/ops polish):**

* [ ] License retention scheduler: hard-delete expired licenses after 30d + detach media if unreferenced
* [ ] Admin “license queue” list endpoint (pending verification, paginated)
* [ ] Audit log rows for: admin verify/reject + scheduler expiry flip

---

## **Phase 4 — Async Backbone (Outbox Pattern)** ✅ DONE

**Goal:** Reliable eventing for all side effects

**Completed:**

* [x] OutboxEvent model + enums + migration
* [x] Outbox DTOs, registry, repo, service
* [x] Outbox publisher worker
* [x] Redis-backed idempotency strategy (publisher + consumers)
* [x] Local execution + system documentation

➡️ **This is a major milestone. Fully implemented.**

**REMAINING (safety):**

* [ ] Outbox cleanup scheduler: delete published rows older than 30d
* [ ] DLQ policy + max retry conventions (document + minimal implementation hooks)

---

## **Phase 5 — Products, Inventory, Pricing** ⚠️ MOSTLY COMPLETE

**Goal:** Vendor listings with inventory correctness

**Planned tickets:**

* [x] Product model + migrations (media_id required)
* [x] Inventory model + migrations
* [x] Volume discount model + migrations
* [x] Vendor create product endpoint
* [x] Vendor update product endpoint (media immutable)
* [x] Vendor delete product endpoint
* [x] MOQ validation (client + server)
* [x] Vendor visibility gating (license + subscription)


**REMAINING (what’s actually still missing):**

* [ ] **Inventory set endpoint** (`PUT /api/v1/inventory/{productId}`): service + repo update, idempotency required
* [ ] Inventory list endpoint (vendor-only): paginated list of inventory rows with product summary
* [ ] Product audit logging:

  * [ ] Audit action schema + helper (`pkg/audit` or `internal/audit`)
  * [ ] Emit audit rows on product create/update/delete + inventory set

**Small “vendor summary” ticket (from your note):**

* [ ] Add `VendorSummary` shape returned in product browse/detail:

  * [ ] Repo query joins `stores` + optional logo media attachment
  * [ ] DTO includes `{vendor_store_id, vendor_name, logo_gcs_key?, logo_url?}`

---

## **Phase 6 — Cart & Checkout** ✅ DONE

**Goal:** Multi-vendor checkout with atomic reservation

**Planned tickets:**

* [X] CartRecord + CartItem models
* [X] Cart upsert endpoint
* [X] Cart fetch endpoint
* [X] Checkout submission endpoint
* [X] CheckoutGroup creation
* [X] VendorOrder creation per vendor
* [X] Atomic inventory reservation logic
* [X] Partial checkout semantics
* [X] Checkout idempotency enforcement
* [X] Emit `order_created` outbox event

**REMAINING**

* [ ] Checkout response contract tests (partial vendor failures + line-level failures)
* [ ] “Converted cart” behavior: enforce `cart_records.status=converted` and reject future cart upserts if desired

---

## Phase 7 — Orders: Read APIs + Decisioning + Fulfillment + TTL

**Goal:** Everything after checkout: order list/detail, accept/reject, fulfill, expire, retry

### 7A) Read APIs

* [X] Migrations (if any missing indexes): buyer/vendor list indexes + “needs action” composite index
* [X] Repo: buyer orders list (filters + cursor pagination)
* [X] Repo: vendor orders list (filters + cursor pagination)
* [X] Repo: order detail (preload line items + payment intent)
* [X] Controller/routes:

  * [X] `GET /api/v1/orders` (buyer/vendor perspective)
  * [X] `GET /api/v1/orders/{orderId}`

### 7B) Vendor decision flows

* [X] DTOs: order-level decision request/response
* [X] Service: `POST /api/v1/vendor/orders/{orderId}/decision`

  * [X] State transition validation
  * [X] Set `accepted|rejected|partially_accepted` based on item outcomes
  * [X] Recompute `balance_due_cents` after partial accept
  * [X] Release inventory for rejected line items (reverse reserve)
* [X] Service: `POST /api/v1/vendor/orders/{orderId}/line-items/decision`

  * [X] Per-line accept/reject + recompute order status + balance due
  * [X] Emit outbox events: `order_decided` (and line-level payload detail)

### 7C) Fulfillment -> almsot handled automatically with the (`POST /api/v1/vendor/orders/{orderId}/line-items/decision`) ticket

* [ ] Service: `POST /api/v1/vendor/orders/{orderId}/fulfill`

  * [ ] Mark accepted items fulfilled (idempotent)
  * [ ] Set order status to `fulfilled` then `hold` (hold_for_pickup semantics)
  * [ ] Emit outbox: `order_ready_for_dispatch`

### 7D) Buyer actions

* [X] Service: `POST /api/v1/orders/{orderId}/cancel` (pre-transit only)

  * [X] Release reserved inventory for all non-fulfilled items
  * [X] Order status to `canceled`
  * [X] Emit outbox: `order_canceled`
* [X] Service: `POST /api/v1/orders/{orderId}/nudge` (writes notification event; later email)
* [X] Service: `POST /api/v1/orders/{orderId}/retry` (only `expired`)

  * [X] Create new CheckoutGroup attempt from prior order snapshot (or re-run checkout-like flow)

### 7E) Reservation TTL scheduler (5d + nudge + 5d + expire)

* [ ] Scheduler: find orders `created_pending` beyond TTL window
* [ ] Scheduler: emit “nudge” event once + extend TTL marker
* [ ] Scheduler: expire after second window → `expired_at`, status `expired`
* [ ] Scheduler: release inventory reservations idempotently
* [ ] Emit outbox: `order_expired`

---

## Phase 8 — Delivery & Agents (Internal Logistics)

**Goal:** Agent queue + assignment + pickup/deliver + cash collection

### 8A) Agent identity + gating

* [X] Ensure `users.system_role=agent` path works end-to-end (seed agent user + login + middleware)
* [X] Add `/api/v1/agent/*` route group controllers/tests

### 8B) Assignment + queues

* [X] Migration: `order_assignments` (if not already in DB)
* [X] Repo/service: create assignment on “ready for dispatch” (random auto-assign MVP)
* [X] Endpoint: `GET /api/v1/agent/orders/queue` (unassigned hold orders)
* [X] Endpoint: `GET /api/v1/agent/orders` (my active assignments)

### 8C) State transitions

* [X] Endpoint: `POST /api/v1/agent/orders/{orderId}/pickup` → status `in_transit`
* [X] Endpoint: `POST /api/v1/agent/orders/{orderId}/deliver` → status `delivered`
* [ ] Endpoint: `POST /api/v1/agent/orders/{orderId}/cash-collected`

  * [ ] Create `ledger_events(cash_collected)`
  * [ ] Set `payment_intents.status=settled` + `cash_collected_at`
  * [ ] Emit outbox: `cash_collected`

---

## Phase 9 — Ledger, Payouts, Admin Ops

**Goal:** Append-only finance correctness + admin payout confirmation

* [X] Migration: `ledger_events` (append-only) + required indexes
* [X] Repo/service: append ledger event helpers (no updates)
* [X] Admin endpoint: `GET /api/v1/admin/orders/payouts` & `GET /api/v1/admin/orders/payouts/{orderID}` (delivered + settled, unpaid)
* [X] Admin endpoint: `POST /api/v1/admin/orders/{orderId}/confirm-payout`

  * [X] Create `ledger_events(vendor_payout)`
  * [X] Set `payment_intents.status=paid` + `vendor_paid_at`
  * [X] Set order status `closed`
  * [X] Emit outbox: `order_paid`
* [ ] Optional secondary endpoint: vendor “confirm paid” (audited, doesn’t flip truth)

---

## Phase 10 — Notifications (In-app)

**Goal:** In-app notifications without polling loops

* [X] Migration: `notifications` table + indexes (store_id, read_at, created_at)
* [X] Repo/service: list notifications (cursor pagination + unread filter)
* [X] Endpoint: `GET /api/v1/notifications`
* [X] Endpoint: `POST /api/v1/notifications/{id}/read` (idempotent)
* [X] Endpoint: `POST /api/v1/notifications/read-all` (idempotent)
* [ ] Cleanup scheduler: delete notifications older than 30d

---

## Phase 11 — Analytics (Marketplace via BigQuery)

**Goal:** BigQuery ingestion + vendor/admin analytics endpoints

### 11A) BigQuery infra + schema

* [X] Create dataset + tables (`marketplace_events`, `ad_events`) with partition/cluster rules (this may be code + gcloud CLI --- break up accordingly)
* [X] `pkg/bigquery` client bootstrap + readiness checks (API/worker if needed)

### 11B) Ingestion worker (outbox consumer) 

* [X] Consumer: `order_created` → insert BigQuery row
* [X] Consumer: `cash_collected` → insert BigQuery row
* [X] Consumer: `order_paid` → insert BigQuery row
* [X] Idempotency keys per consumer (`pf:evt:processed:<consumer>:<event_id>`)

### 11C) Query APIs

* [X] Vendor analytics endpoint: `GET /api/v1/vendor/analytics` (time presets + series + KPIs)
* [ ] Admin analytics endpoint: `GET /api/v1/admin/analytics` (global KPIs)

---


## Phase 12 — Subscriptions & Billing (Stripe CC)

**Goal:** Vendor subscription gating + billing history

### 12A) Stripe integration

* [ ] Stripe client bootstrap + config/secrets
* [ ] Migrations: `subscriptions`, `payment_methods`, `charges`, `usage_charges` (if not already applied)

### 12B) Subscription flows

* [ ] `POST /api/v1/vendor/subscriptions` (create subscription, idempotent)
* [ ] `POST /api/v1/vendor/subscriptions/cancel` (idempotent)
* [ ] `GET /api/v1/vendor/subscriptions/active`
* [ ] Webhook consumer (Stripe) updates subscription state + mirrors `stores.subscription_active`

### 12C) Billing history

* [ ] `GET /api/v1/vendor/billing/charges` (ads + subscriptions)
* [ ] Enforce gating everywhere:

  * [ ] browse/search hides vendor listings if `subscription_active=false`
  * [ ] ads/analytics blocked if inactive

---

## Phase 13 — Ads & Attribution

**Goal:** Monetization: ad CRUD + tracking + last-click attribution + rollups

### 13A) Core ads

* [ ] Migrations: `ads`, `ad_creatives`, `ad_clicks`, `ad_events` + indexes
* [ ] Vendor CRUD endpoints:

  * [ ] `POST /api/v1/vendor/ads` (idempotent)
  * [ ] `GET /api/v1/vendor/ads`
  * [ ] `GET /api/v1/vendor/ads/{adId}`
  * [ ] `PATCH /api/v1/vendor/ads/{adId}`
  * [ ] `DELETE /api/v1/vendor/ads/{adId}`
* [ ] Eligibility enforcement: license verified + subscription active + ad status active + budget remaining

### 13B) Tracking + attribution

* [ ] Impression endpoint (or middleware hook) increments Redis counters + writes `ad_events(impression)`
* [ ] Click endpoint writes:

  * [ ] `ad_clicks` row (30d TTL semantics)
  * [ ] `ad_events(click)`
* [ ] Checkout integration: attach last eligible click to `checkout_groups.attributed_ad_click_id` and propagate to vendor orders

### 13C) Rollups + analytics

* [ ] Daily rollup scheduler: compute per-ad spend and write `usage_charges(ad_spend_daily)`
* [ ] Ad analytics endpoint: vendor view (CTR, impressions, clicks, ROAS via attributed orders)

---

## Phase 14 — Search & Geo (PostGIS-enabled filters)

**Goal:** Buyer-origin geo filtering + vendor delivery radius rules

* [ ] PostGIS extension migration + verify in Heroku
* [ ] Store geocoding integration on create (and admin override only)
* [ ] Persist `stores.geom` correctly from lat/lng
* [ ] Directory endpoint: `GET /api/v1/stores/vendors` with:

  * [ ] `ST_DWithin` filter by delivery radius
  * [ ] sort by distance
* [ ] Product browse endpoint improvements:

  * [ ] filter by state + vendor visibility + optional geo gate
* [ ] Index tuning: `gist(stores.geom)` + composite visibility indexes

---

## Phase 15 — Security, Ops, Hardening

**Goal:** Production resilience + auditability

* [ ] Rate limiting expansion to critical endpoints (checkout, decisions, agent cash)
* [ ] Audit log system:

  * [ ] Migration `audit_logs`
  * [ ] Helper to write append-only audit rows
  * [ ] Emit audit logs for: admin verify license, payouts, order decisions, inventory changes
* [ ] Metrics (API + workers) + basic dashboards/alerts
* [ ] Backup/restore runbook (Heroku PG + Redis)
* [ ] Feature flags (minimal) for risky rollouts (ads/subscription/analytics)
* [ ] MFA/email verification adapter hooks (no full build until needed)
