## Phase 0 — Repo, Infra, CI, Foundations

**Goal:** A deployable, observable, production-grade Go monolith + worker binaries.

* [x] Ticket: Initialize repo and standard Go monolith layout (`cmd/api`, `cmd/worker`, `pkg`, `internal`)
* [x] Ticket: Implement config + env loading
* [x] Ticket: Implement structured JSON logging with request/job correlation
* [x] Ticket: Standardize API error codes + response envelopes
* [x] Ticket: Wire Chi router + middleware stack
* [x] Ticket: Implement standard request validation layer
* [x] Ticket: Bootstrap Postgres for Cloud SQL + Heroku compatibility
* [x] Ticket: Bootstrap Redis for sessions, rate limits, idempotency
* [x] Ticket: Add Goose migrations runner + hybrid policy conventions
* [x] Ticket: Add Dockerfile + Heroku Procfile (web + worker)
* [x] Ticket: Add GitHub Actions CI (lint, test, build)
* [x] Ticket: Wire worker bootstrap dependency graph

---

## Phase 1 — Identity, Auth, Tenancy, RBAC

**Goal:** Secure multi-store authentication with session revocation and role-based access.

* [x] Ticket: Implement User model with Argon2id password hashing
* [x] Ticket: Implement Store model with address shape
* [x] Ticket: Implement StoreMembership model with role enum
* [x] Ticket: Implement JWT minting/parsing with enforced claims
* [x] Ticket: Implement refresh token storage + rotation in Redis
* [x] Ticket: Implement login endpoint (email/password)
* [x] Ticket: Implement register endpoint (user + first store + owner membership)
* [x] Ticket: Implement logout + refresh endpoints
* [x] Ticket: Implement active store switching via refresh/JWT
* [x] Ticket: Implement login brute-force rate limiting (Redis)
* [x] Ticket: Implement store profile read/update endpoints
* [x] Ticket: Implement store user list/invite/remove endpoints
* [x] Ticket: Implement admin auth model + storeless admin role
* [x] Ticket: Implement admin login endpoint (storeless) + token issuance
* [x] Ticket: Implement dev-only admin register endpoint

* [ ] Ticket: Add auth middleware tests (missing/expired token, revoked session, missing activeStoreId)
* [ ] Ticket: Add RBAC guard tests for `/api/admin/*` and `/api/v1/agent/*`

---

## Phase 2 — Media System

**Goal:** Single, reusable media pipeline with safe delete semantics.

* [x] Ticket: Implement media table + enums + GORM model for upload lifecycle
* [x] Ticket: Implement canonical media metadata persistence
* [x] Ticket: Bootstrap GCS client (API + worker)
* [x] Ticket: Implement presigned PUT upload flow (create media row + signed PUT)
* [x] Ticket: Implement Pub/Sub consumer for GCS `OBJECT_FINALIZE` to mark uploaded
* [x] Ticket: Implement presigned READ URL generation (TTL-based)
* [x] Ticket: Implement media list endpoint (store-scoped, paginated)
* [x] Ticket: Implement media delete endpoint (reference-aware)
* [x] Ticket: Enforce protected attachment checks on media delete

* [ ] Ticket: Validate only `activeStoreID` queries return store related media files
* [ ] Ticket: Deleting is not actually deleting a media record. Must delete media row + the GCS object. Only `license` kind is protected if not in an `uploaded` or `ready` state. 

---

## Phase 3 — Compliance & Licensing

**Goal:** Store-level compliance gating via licenses and lifecycle automation.

* [x] Ticket: Add licenses schema + GORM model (media_id required)
* [x] Ticket: Implement license create endpoint (atomic metadata + media_id)
* [x] Ticket: Implement license list endpoint (store-scoped)
* [x] Ticket: Implement admin approve/reject license endpoint
* [x] Ticket: Implement license delete endpoint (expired-only semantics)
* [x] Ticket: Mirror license status to store KYC/compliance status
* [x] Ticket: Emit license status outbox events + compliance notifications
* [x] Ticket: Implement license expiry scheduler (14-day warn + expire)
* [x] Ticket: Wire license lifecycle cron jobs into cron worker
* [x] Ticket: Implement license retention scheduler (hard-delete expired after 30d + detach media if unreferenced)
* [x] Ticket: Implement admin license queue list endpoint (pending verification, paginated)

* [ ] Ticket: Add audit log rows for admin verify/reject + scheduler expiry flip

---

## Phase 4 — Async Backbone: Outbox + DLQ

**Goal:** Reliable eventing for side effects with retry/DLQ support.

* [x] Ticket: Implement outbox table/enums/envelope + migration
* [x] Ticket: Implement outbox DTOs/registry/repo/service + wiring
* [x] Ticket: Implement Redis-backed idempotency strategy (publisher + consumers)
* [x] Ticket: Implement outbox dispatcher worker binary
* [x] Ticket: Implement typed event→topic routing registry + payload validation
* [x] Ticket: Implement DLQ table + model + repository
* [x] Ticket: Publish terminal failures to DLQ
* [x] Ticket: Implement outbox retention cleanup job (>30d published)
* [x] Ticket: Document and implement DLQ retry policy + MAX_ATTEMPTS conventions (minimal hooks)
* [ ] Ticket: Create DLQ replay tooling/runbook (safe requeue + idempotency expectations)

---

## Phase 5 — Products, Inventory, Pricing

**Goal:** Vendor listings with inventory correctness and marketplace visibility gating.

* [x] Ticket: Add products schema + GORM model + repos
* [x] Ticket: Add inventory schema + GORM model + repos
* [x] Ticket: Add volume discounts schema + GORM model + repos
* [x] Ticket: Implement vendor create product endpoint
* [x] Ticket: Implement vendor update product endpoint
* [x] Ticket: Implement vendor delete product endpoint
* [x] Ticket: Implement MOQ validation in product flows
* [x] Ticket: Enforce visibility gating (license + subscription + state)
* [x] Ticket: Patch product create route for discount type + migration alignment
* [ ] Ticket: Implement inventory set endpoint (`PUT /api/v1/inventory/{productId}`) with idempotency
* [ ] Ticket: Implement inventory list endpoint (vendor-only) with pagination + product summary
* [ ] Ticket: Implement audit action schema/helper for product/inventory actions
* [ ] Ticket: Emit audit rows on product create/update/delete + inventory set
* [ ] Ticket: Add VendorSummary to product browse/detail (join stores + optional logo attachment)

---

## Phase 6 — Cart Quote Contract: Authoritative, Server-Computed

**Goal:** Server-authoritative cart quote persisted as auditable snapshot (pre-checkout).

* [x] Ticket: Migrate cart schema to authoritative quote model (enums + cart_records/cart_items/cart_vendor_groups)
* [x] Ticket: Update/add GORM models for cart quote schema (CartRecord/CartItem/CartVendorGroup + enums + JSONB typing)
* [x] Ticket: Implement CartRecord repo upsert (active cart per buyer_store)
* [x] Ticket: Implement CartItem repo replace-on-quote (delete+insert) with tx handle
* [x] Ticket: Implement CartVendorGroup repo replace-on-quote with tx handle
* [x] Ticket: Implement atomic transaction wrapper across record + items + vendor groups
* [x] Ticket: Implement quote DTOs (`QuoteCartRequest`, `CartQuote`) and controller subfolder organization
* [x] Ticket: Refactor controller/service to quote-based contract (replace legacy upsert entrypoint)
* [x] Ticket: Remove client-supplied totals/pricing; quote becomes intent-only
* [x] Ticket: Implement vendor preload + product fetch pipeline grouped by vendor_store_id
* [x] Ticket: Implement qty normalization + item status/warnings (MOQ/max/availability/vendor mismatch)
* [x] Ticket: Implement pricing + volume discount resolution and persist quote fields
* [x] Ticket: Implement vendor group aggregation and persist cart_vendor_groups
* [x] Ticket: Compute/persist cart totals + valid_until
* [x] Ticket: Enforce quote-only invariant (no inventory mutation/reservation)
* [x] Ticket: Validate vendor promo ownership + apply to vendor group totals
* [x] Ticket: Emit vendor-level warnings for invalid promos (soft)
* [x] Ticket: Wire `POST /cart` endpoint to DTOs/service with validation
* [x] Ticket: Normalize HTTP semantics (hard errors vs soft warnings)
* [ ] Ticket: Implement cart quote mapping helpers (DB ↔ domain ↔ DTO) with unit tests
* [ ] Ticket: Implement header-based idempotency middleware for `POST /cart` (`Idempotency-Key`, scoped to buyer_store_id + endpoint)
* [ ] Ticket: Implement cart attribution token pass-through (validate signature/expiry only, persist on cart_records, echo in CartQuote)
* [ ] Ticket: Implement cart conversion readiness (active→converted state transition + generate/persist checkout_group_id at conversion)
* [ ] Ticket: Add structured logs and metrics in quote service (counts, warnings, duration)
* [ ] Ticket: Add rate limiting for `POST /cart`
* [ ] Ticket: Add regression tests for quote invariants (MOQ clamp, vendor invalid/mismatch, invalid promo, no inventory mutation)
* [ ] Ticket: Implement “converted cart” behavior guardrails (reject future quote-upserts if desired)

---

## Phase 7 — Checkout Refactor Safety Locks

**Goal:** Make the refactor safe to land incrementally without half-old/half-new behavior.

* [ ] Ticket: Write `CHECKOUT_REFACTOR.md` sequencing locks (what lands first, what’s forbidden until later)
* [ ] Ticket: Add feature flag/config guard for selecting new checkout flow entrypoint (if parallel flows exist)
* [ ] Ticket: Commit grep checklist of all references to `checkout_groups` and `AttributedAdClickID`

---

## Phase 8 — Checkout Schema Refactor: Drop `checkout_groups`

**Goal:** Remove persisted checkout_groups aggregate while keeping `checkout_group_id` as UUID anchor.

* [x] Ticket: Add Goose migration to drop `checkout_groups` table
* [x] Ticket: Remove FK constraints referencing `checkout_groups(id)` (e.g., vendor_orders.checkout_group_id FK)
* [x] Ticket: Ensure `vendor_orders.checkout_group_id` is UUID column (no FK)
* [x] Ticket: Ensure `cart_records.checkout_group_id` remains (nullable pre-conversion)
* [x] Ticket: Add/ensure index `idx_vendor_orders_checkout_group_id`
* [x] Ticket: Add/ensure index `idx_cart_records_checkout_group_id`
* [ ] Ticket: Validate migration against existing DB snapshot and resolve orphaned constraint issues

---

## Phase 9 — Checkout Schema: Persist Checkout-Confirmed Fields on Cart

**Goal:** Cart becomes auditable pre/post checkout (confirmed selections persisted).

* [x] Ticket: Add Goose migration for `cart_records.payment_method`
* [x] Ticket: Add Goose migration for `cart_records.shipping_line` (jsonb or explicit fields)
* [x] Ticket: Add Goose migration for `cart_records.converted_at`
* [x] Ticket: Update GORM `models.CartRecord` with new fields

---

## Phase 10 — Checkout Schema: Persist Checkout Snapshot on Vendor Orders

**Goal:** Orders persist checkout snapshot so downstream reads don’t recompute.

* [x] Ticket: Add Goose migration for `vendor_orders.cart_id` (UUID; backfill-safe)
* [x] Ticket: Add Goose migration for `vendor_orders.currency`
* [x] Ticket: Add Goose migration for `vendor_orders.shipping_address` (address_t)
* [x] Ticket: Add Goose migration for `vendor_orders.discounts_cents` (rename/standardize plan if needed)
* [x] Ticket: Add Goose migration for `vendor_orders.warnings` (jsonb)
* [x] Ticket: Add Goose migration for `vendor_orders.promo` (jsonb)
* [x] Ticket: Add Goose migration for `vendor_orders.payment_method`
* [x] Ticket: Add Goose migration for `vendor_orders.shipping_line` (jsonb or explicit fields)
* [x] Ticket: Add Goose migration for `vendor_orders.attributed_token` (jsonb nullable)
* [x] Ticket: Update GORM `models.VendorOrder` to match new columns + JSON serializers

---

## Phase 11 — Checkout Schema: Persist Snapshot + Attribution on Order Line Items

**Goal:** Line items become stable cart snapshots with deterministic attribution storage.

* [x] Ticket: Add Goose migration for `order_line_items.cart_item_id` (uuid nullable initially)
* [x] Ticket: Add Goose migration for `order_line_items.warnings` (jsonb)
* [x] Ticket: Add Goose migration for `order_line_items.applied_volume_discount` (jsonb)
* [x] Ticket: Add Goose migration for `order_line_items.moq` and `order_line_items.max_qty`
* [x] Ticket: Add Goose migration for `order_line_items.line_subtotal_cents`
* [x] Ticket: Add Goose migration for `order_line_items.attributed_token` (jsonb nullable)
* [x] Ticket: Update GORM `models.OrderLineItem` to match new columns + JSON serializers

---

## Phase 12 — Checkout API Boundary: DTO + Controller Wiring + Idempotency Header

**Goal:** Stabilize the HTTP contract before rewriting service internals.

* [x] Ticket: Implement checkout request DTO (`cart_id`, `shipping_address`, `payment_method`, optional `shipping_line`)
* [x] Ticket: Add checkout request validation (required fields + enums)
* [x] Ticket: Enforce idempotency header on `POST /checkout` (middleware or handler)
* [x] Ticket: Wire idempotency key from headers into checkout service input
* [x] Ticket: Remove `AttributedAdClickID` handling from controller/DTOs (use cart_records tokens only)
* [x] Ticket: Update checkout response mapping to include confirmed shipping/payment/shipping_line fields
* [x] Ticket: Refactor checkout controller DTOs/helpers into dedicated subfolder (mirror cart controller pattern)

---

## Phase 13 — Checkout Service Refactor: Load + Validate Cart by Buyer Store

**Goal:** Establish canonical checkout entrypoint and fail early before writes.

* [x] Ticket: Load CartRecord by `(buyer_store_id, cart_id)` inside DB transaction in checkout service
* [x] Ticket: Validate cart `status=active`
* [x] Ticket: Validate cart contains at least one orderable item (`cart_items.status=ok`)
* [x] Ticket: Return deterministic validation errors (wrong buyer_store, wrong status, no ok items)
* [x] Ticket: Add repo helper to load cart with ok-items count (single query, tx-safe)

---

## Phase 14 — Checkout Finalization: Convert Cart + Reuse Checkout Group ID

**Goal:** Cart is conversion source of truth; anchor created once and reused on retries.

* [x] Ticket: Persist `cart_records.shipping_address` from checkout input
* [x] Ticket: Persist `cart_records.payment_method` from checkout input
* [x] Ticket: Persist optional `cart_records.shipping_line` from checkout input
* [x] Ticket: Set `cart_records.converted_at`
* [x] Ticket: Transition `cart_records.status` from `active → converted`
* [x] Ticket: Generate `cart_records.checkout_group_id` if null
* [x] Ticket: Ensure retries reuse existing checkout_group_id (no regeneration)
* [x] Ticket: Add repo helper to finalize cart atomically (single guarded update)

---

## Phase 15 — Checkout Writes: Vendor Orders + Line Items + Payment Intents

**Goal:** Create vendor orders deterministically from cart snapshot and create payment intents per vendor order.

* [x] Ticket: Load `cart_vendor_groups` and group by `vendor_store_id`
* [x] Ticket: Create one `vendor_orders` row per vendor with eligible items
* [x] Ticket: Populate vendor order snapshot fields from cart + confirmed checkout fields
* [x] Ticket: Ensure vendor totals come from `cart_vendor_groups` (no recomputation)
* [x] Ticket: Bulk create vendor orders within checkout transaction (repo method)
* [x] Ticket: Create order_line_items from `cart_items.status=ok` with `cart_item_id` linkage
* [x] Ticket: Persist line item warnings + applied_volume_discount + moq/max_qty + line_subtotal snapshot fields
* [x] Ticket: Skip vendor order creation when vendor has zero `status=ok` items
* [x] Ticket: Surface vendor-level rejection reason in checkout response when vendor has no eligible items
* [x] Ticket: Bulk create order line items per vendor order (repo method)
* [ ] Ticket: Create one payment_intent per vendor order inside checkout transaction
* [ ] Ticket: Set payment intent amount from `vendor_orders.total_cents`
* [ ] Ticket: Set payment intent method from checkout-confirmed payment_method
* [ ] Ticket: Keep existing default payment status behavior (no processing changes)

---

## Phase 16 — Inventory Reservation at Checkout + Deterministic Totals

**Goal:** Reservation is checkout-only; failures map to line items and vendor-level rejection; pricing is cart-truth.

* [x] Ticket: Build reservation requests only for `cart_items.status=ok`
* [x] Ticket: Execute reservation inside checkout transaction
* [x] Ticket: Map reservation failures to `order_line_items.status=rejected` with reason/notes
* [x] Ticket: Mark vendor rejected in response when all items fail reservation (still return vendor card)
* [x] Ticket: Implement helper to apply reservation results and recompute affected totals
* [x] Ticket: Remove/bypass unit price/discount recomputation in checkout execution path
* [x] Ticket: Ensure vendor totals only change due to rejected items post-reservation
* [x] Ticket: Implement final totals recompute that subtracts rejected subtotals only (no tier/promo recompute)
* [x] Ticket: Ensure checkout response is deterministic for identical cart snapshot + reservation results

---

## Phase 17 — Checkout Idempotency + Exactly-Once-ish Outbox Emission

**Goal:** Safe retries: no duplicate orders/outbox rows; emit exactly two events on success.

* [ ] Ticket: Implement “already converted” early return path (lookup by checkout_group_id + load existing orders/lines/intents)
* [ ] Ticket: Prevent duplicate vendor orders on retry (uniqueness anchored on checkout_group_id+vendor_store_id and/or cart_id)
* [ ] Ticket: Prevent duplicate outbox rows on retry for same conversion anchor
* [ ] Ticket: Add repo helper to fetch full checkout result by checkout_group_id
* [ ] Ticket: Define/extend outbox payload for Notifications checkout-converted event
* [ ] Ticket: Define/extend outbox payload for Analytics checkout-converted event (cart totals + attribution refs/tokens)
* [ ] Ticket: Emit Notifications outbox event in same transaction as vendor order creation
* [ ] Ticket: Emit Analytics outbox event in same transaction as cart conversion
* [ ] Ticket: Add outbox payload versioning rules for these events

---

## Phase 18 — Checkout Attribution Materialization

**Goal:** Replace request-time attribution with deterministic selection from cart tokens.

* [ ] Ticket: Validate ad tokens during checkout (decode/verify signature, expiry, enums, buyer_store_id binding)
* [ ] Ticket: Define deterministic selection rules (click > impression, newest wins, stable tie-break)
* [ ] Ticket: Materialize order-level attribution into `vendor_orders.attributed_token`
* [ ] Ticket: Materialize line-item attribution into `order_line_items.attributed_token`
* [ ] Ticket: Persist attribution blobs within checkout transaction boundary

---

## Phase 19 — Checkout Legacy Cleanup

**Goal:** Remove deprecated artifacts end-to-end; one authoritative checkout flow.

* [x] Ticket: Remove `CreateCheckoutGroup` and related repo/service methods
* [x] Ticket: Remove remaining references to dropped `checkout_groups` table
* [x] Ticket: Remove `AttributedAdClickID` usage from DTOs/service inputs/outbox payloads
* [x] Ticket: Normalize imports/helpers to canonical models in `pkg/db/models/*`
* [x] Ticket: Delete dead structs/helpers tied to legacy checkout flow

---

## Phase 20 — Checkout Regression Test Suite

**Goal:** Prevent reintroducing old behavior and lock correctness of refactor.

* [ ] Ticket: Add regression test for idempotent checkout retry (no duplicate vendor orders, no duplicate outbox rows)
* [ ] Ticket: Add regression test for cart expired/already converted behavior
* [ ] Ticket: Add regression test for partial reservation failures (line rejected + vendor-level handling)
* [ ] Ticket: Add regression test for attribution selection rules (click/impression priority + tie-break)
* [ ] Ticket: Add regression test for exactly two outbox events on successful conversion
* [ ] Ticket: Add compile-level/schema drift tests for GORM model changes

---

## Phase 21 — Orders: Read APIs + Decisioning + Fulfillment + TTL

**Goal:** Post-checkout lifecycle: read APIs, vendor decisions, fulfillment, expiration, retries.

* [x] Ticket: Add missing order indexes for list/detail/action queues
* [x] Ticket: Implement buyer orders list repo/service (filters + pagination)
* [x] Ticket: Implement vendor orders list repo/service (filters + pagination)
* [x] Ticket: Implement order detail repo/service (preload line items + payment intent)
* [x] Ticket: Implement orders list endpoint (`GET /api/v1/orders`) for buyer/vendor perspective
* [x] Ticket: Implement order detail endpoint (`GET /api/v1/orders/{orderId}`)
* [x] Ticket: Implement vendor order decision endpoint (order-level decisioning + transitions)
* [x] Ticket: Implement vendor line-item decision endpoint (accept/reject + recompute + release inventory)
* [x] Ticket: Emit outbox events for order decisioning (order_decided with line-level detail)
* [x] Ticket: Implement buyer cancel endpoint (pre-transit) + release inventory + emit outbox
* [x] Ticket: Implement buyer nudge endpoint (notification event)
* [x] Ticket: Implement buyer retry endpoint (expired-only) to create new attempt
* [x] Ticket: Implement order TTL cron job (nudge → expire → inventory release) and emit outbox
* [ ] Ticket: Implement vendor fulfill endpoint (`POST /api/v1/vendor/orders/{orderId}/fulfill`) idempotently
* [ ] Ticket: Transition fulfilled orders into hold/ready-for-dispatch semantics
* [ ] Ticket: Emit outbox event `order_ready_for_dispatch` on fulfillment

---

## Phase 22 — Delivery & Agents

**Goal:** Agent queue + assignment + pickup/deliver + cash collection.

* [x] Ticket: Ensure `users.system_role=agent` auth path works end-to-end
* [x] Ticket: Add `/api/v1/agent/*` route group controllers/tests scaffold
* [x] Ticket: Add `order_assignments` migration
* [x] Ticket: Implement assignment creation for dispatchable orders (random auto-assign MVP)
* [x] Ticket: Implement agent queue endpoint (unassigned hold orders)
* [x] Ticket: Implement agent “my assignments” endpoint
* [x] Ticket: Implement agent pickup endpoint (status `in_transit`)
* [x] Ticket: Implement agent deliver endpoint (status `delivered`)
* [ ] Ticket: Implement agent cash-collected endpoint (`POST /api/v1/agent/orders/{orderId}/cash-collected`)
* [ ] Ticket: Append `ledger_events(cash_collected)` during cash-collected flow
* [ ] Ticket: Set `payment_intents.status=settled` + `cash_collected_at`
* [ ] Ticket: Emit outbox event `cash_collected`

---

## Phase 23 — Ledger, Payouts, Admin Ops

**Goal:** Append-only finance correctness + admin payout confirmation.

* [x] Ticket: Add `ledger_events` table + indexes (append-only)
* [x] Ticket: Implement ledger append helpers (repo + service)
* [x] Ticket: Implement admin payout queue endpoints
* [x] Ticket: Implement admin payout detail endpoint
* [x] Ticket: Implement admin confirm payout endpoint (ledger event + payment intent paid + order closed + outbox)
* [ ] Ticket: Implement optional vendor “confirm paid” endpoint (audited; non-authoritative)

---

## Phase 24 — Notifications: In-App

**Goal:** In-app notifications with read tracking and retention cleanup.

* [x] Ticket: Add notifications schema + indexes
* [x] Ticket: Implement notifications list service (cursor pagination + unread filter)
* [x] Ticket: Implement notifications list endpoint (`GET /api/v1/notifications`)
* [x] Ticket: Implement mark notification read endpoint (idempotent)
* [x] Ticket: Implement mark-all read endpoint (idempotent)
* [x] Ticket: Implement notifications cleanup scheduler (>30d)

---

## Phase 25 — Subscriptions & Billing

**Goal:** Vendor subscription gating + billing history.

* [x] Ticket: Bootstrap Stripe client + config/secrets
* [x] Ticket: Add billing/subscription schema (`subscriptions`, `payment_methods`, `charges`, `usage_charges`)
* [x] Ticket: Implement create subscription endpoint (idempotent)
* [x] Ticket: Implement cancel subscription endpoint (idempotent)
* [x] Ticket: Implement get subscription endpoint (single active)
* [x] Ticket: Implement Stripe webhook consumer to mirror subscription state to `stores.subscription_active`
* [x] Ticket: Implement vendor billing history endpoint (`GET /api/v1/vendor/billing/charges`)
* [x] Ticket: Enforce subscription gating across marketplace browse/search



---

## Phase 26 — BigQuery Analytics: Analytics Worker + Engine

**Goal:** BigQuery ingestion + vendor analytics endpoint for marketplace events. Idempotent analytics consumer with routing, BigQuery write layer, and ad/attribution expansions.

* [x] Ticket: Implement `pkg/bigquery` client bootstrap + readiness checks
* [x] Ticket: Finalize BigQuery dataset + tables (`marketplace_events`, `ad_event_facts`)
* [x] Ticket: Define BigQuery partitioning strategy (`DATE(occurred_at)`)
* [x] Ticket: Check in BigQuery schema definitions / CLI commands to repo
* [x] Ticket: Define canonical analytics event enums (order + ad events)
* [x] Ticket: Define Pub/Sub analytics envelope DTO
* [x] Ticket: Create Pub/Sub topic `analytics` via CLI
* [x] Ticket: Create Pub/Sub subscription `analytics-sub` via CLI
* [x] Ticket: Define BigQuery row DTOs (MarketplaceEventRow, AdEventFactRow)
* [x] Ticket: Define shared timestamp selection helpers (created vs paid vs cash)
* [x] Ticket: Create `cmd/analytics-worker` binary scaffold
* [x] Ticket: Initialize Pub/Sub client + subscription (`analytics-sub`)
* [x] Ticket: Implement Redis idempotency gate (`pf:evt:processed:analytics:<event_id>`)
* [x] Ticket: Define ACK/NACK retry policy for analytics consumer
* [x] Ticket: Add structured logging with event_id correlation
* [x] Ticket: Add graceful shutdown handling
* [x] Ticket: Implement analytics router (`switch(event_type)`) and validate supported types
* [x] Ticket: Create handler interfaces/contracts and stub handlers for all listed event types
* [x] Ticket: Implement BigQuery writer abstraction + retry for transient insert failures
* [x] Ticket: Implement JSON serialization helpers for `items` and `payload`
* [x] Ticket: Support inserts into `marketplace_events`
* [ ] Ticket: Support inserts into `ad_event_facts`
* [x] Ticket: Wire writer into analytics worker handlers
* [x] Ticket: Map `order_created` payload to `marketplace_events` row
* [x] Ticket: Build `items` JSON snapshot from order line items
* [x] Ticket: Extract buyer geo fields from shipping address snapshot
* [x] Ticket: Compute revenue fields (gross/net/discounts)
* [x] Ticket: Handle marketplace ingestion for `order_paid`, `cash_collected`, `order_canceled`, `order_expired`
* [ ] Ticket: Implement attribution token decode utilities for analytics
* [ ] Ticket: Implement deterministic token selection strategy (last-applicable)
* [ ] Ticket: Attribute store-level ads to full order revenue
* [ ] Ticket: Attribute product-level ads to matching line-item revenue
* [ ] Ticket: Emit `ad_event_facts` rows with `type=conversion`
* [ ] Ticket: Ensure marketplace events always write even without tokens
* [ ] Ticket: Emit `ad_daily_charge_recorded` from nightly billing job
* [ ] Ticket: Route ad daily charge events through analytics worker
* [ ] Ticket: Write `ad_event_facts` rows with `type=charge`
* [ ] Ticket: Normalize `occurred_at` for daily spend events
* [ ] Ticket: Ensure spend computable as `SUM(cost_cents)`
* [x] Ticket: Implement marketplace analytics query service (BQ-backed)
* [x] Ticket: Implement orders-over-time query
* [x] Ticket: Implement revenue-over-time query (paid/cash fallback logic)
* [x] Ticket: Implement discounts/net revenue/AOV computations
* [x] Ticket: Implement top products/categories queries via `UNNEST(items)`
* [x] Ticket: Implement top ZIPs query
* [x] Ticket: Implement new vs returning buyer computation
* [ ] Ticket: Implement ad analytics query service (spend/impressions/clicks/ROAS/time series)
* [x] Ticket: Implement marketplace analytics API endpoints
* [x] Ticket: Enforce store scoping via `activeStoreId`
* [x] Ticket: Implement timeframe validation + defaults
* [ ] Ticket: Implement ad analytics API endpoints
* [ ] Ticket: Enforce advertiser ownership for ads
* [ ] Ticket: Shape ad analytics responses to frontend dashboard contracts
* [ ] Ticket: Add unit tests for marketplace mapping logic
* [ ] Ticket: Add unit tests for attribution token decoding
* [ ] Ticket: Add unit tests for analytics SQL builders
* [ ] Ticket: Add integration test harness (handler → BQ writer)
* [ ] Ticket: Add one-time BigQuery backfill script from Postgres orders
* [ ] Ticket: Define deterministic event_id strategy for backfill runs

---

## Phase 28 — Media Attachments: Canonical Linking Layer

**Goal:** Normalize attachments across domains and enforce safe delete semantics.

* [x] Ticket: Finalize `media_attachments` table + indexes
* [x] Ticket: Define attachment lifecycle rules (docs/comments) and protected attachment semantics
* [ ] Ticket: Wire license ↔ media attachments
* [ ] Ticket: Wire product ↔ media attachments (gallery + COA)
* [ ] Ticket: Wire store ↔ media attachments (logo/banner)
* [ ] Ticket: Wire user ↔ media attachments (avatar)
* [ ] Ticket: Wire ad ↔ media attachments

---

## Phase 29 — Workers: Dedicated Binaries

**Goal:** One-responsibility workers for time-based jobs, outbox dispatch, and media processing/deletion.

* [x] Ticket: Implement `cmd/cron-worker` binary with scheduler registry + locking + metrics
* [x] Ticket: Implement license lifecycle jobs in cron worker (warn/expire/hard delete)
* [x] Ticket: Implement order TTL job in cron worker (nudge/expire/release)
* [x] Ticket: Implement notification cleanup job in cron worker
* [x] Ticket: Implement outbox cleanup job in cron worker
* [x] Ticket: Document concurrency model decision for cron worker
* [x] Ticket: Finalize `cmd/outbox-publisher` main loop (poll, `FOR UPDATE SKIP LOCKED`, sleep jitter)
* [x] Ticket: Make outbox batch size configurable
* [x] Ticket: Add structured logging per outbox publish attempt
* [x] Ticket: Implement bounded retry policy (retryable vs terminal)
* [x] Ticket: Persist attempts + last_error and enforce MAX_ATTEMPTS
* [x] Ticket: Mark published only after Pub/Sub ACK
* [x] Ticket: Implement DLQ write + mark terminal atomically
* [x] Ticket: Ensure dispatcher excludes terminal rows
* [ ] Ticket: Create `cmd/media-worker` binary
* [ ] Ticket: Subscribe media-worker to GCS finalize events (env-driven)
* [ ] Ticket: Implement image/video compression helper
* [ ] Ticket: Implement OCR provider abstraction (OpenAI vs Document AI)
* [ ] Ticket: Implement OCR text generation + store derived asset (`ocr.txt`)
* [ ] Ticket: Update media row with derived artifacts
* [ ] Ticket: Emit `media_processed` outbox event
* [x] Ticket: Create `cmd/media-delete-worker` binary
* [x] Ticket: Consume `media_deleted` events in media-delete-worker
* [ ] Ticket: Detach all attachment references by entity type prior to deletion
* [ ] Ticket: Delete GCS originals + derived artifacts (not-found treated as success)
* [ ] Ticket: Persist deletion outcomes/status updates
* [ ] Ticket: Implement stale `pending` media GC scheduler (row exists, no GCS object)

---

## Phase 30 — Integration Test Harness

**Goal:** Deterministic end-to-end validation using real HTTP calls.

* [x] Ticket: Create initial integration scaffold (partial)
* [x] Ticket: Implement initial register scripts (partial/dud per note)
* [ ] Ticket: Create `/scripts/integration/` scaffold and `make integration-test`
* [ ] Ticket: Implement shared HTTP client helper (base URL, retries, timeouts, JSON, assertions)
* [ ] Ticket: Implement colored JSON console logger (stdout-only)
* [ ] Ticket: Implement scripted register flow (buyer + vendor) and output IDs/tokens
* [ ] Ticket: Implement scripted login flow (buyer + vendor)
* [ ] Ticket: Implement in-memory token store helper for scripts
* [ ] Ticket: Implement auth header injection helper (no manual header wiring)
* [ ] Ticket: Add static media fixtures (`fixtures/media/*`) including image/video/PDF
* [ ] Ticket: Script media create (request presigned upload URL) and store media_id + signed URL
* [ ] Ticket: Script upload via signed URL (PUT stream to GCS)
* [ ] Ticket: Script poll media status until `uploaded` with timeout
* [ ] Ticket: Script create license using media_id and validate `pending`
* [ ] Ticket: Script admin approve/reject license (admin login + decision toggle)
* [ ] Ticket: Script create product with gallery + COA + set inventory
* [ ] Ticket: Script full happy-path: product → cart → checkout → agent deliver → payout

---

## Phase 31 — Notifications: Email Pipeline

**Goal:** Email notification delivery via adapter interface and SendGrid integration.

* [ ] Ticket: Define notification email templates
* [ ] Ticket: Define email sender interface
* [ ] Ticket: Implement stub email sender (log-only)
* [ ] Ticket: Implement SendGrid adapter (future swap)

---

## Phase 32 — COA → OpenAI Product Drafts

**Goal:** OCR/parse COA PDFs into structured product draft JSON and persist draft state.

* [ ] Ticket: Implement OpenAI client bootstrap
* [ ] Ticket: Implement COA OCR → structured parser
* [ ] Ticket: Implement product draft JSON generator from parsed COA
* [ ] Ticket: Persist product draft + status

---

## Phase 33 — Ads Engine

**Goal:** CPM ad engine with Redis counters + signed tokens, checkout-time attribution, daily rollups, and analytics/billing fanout.

* [ ] Ticket: Define ad engine constants + Redis key schema + TTL conventions
* [ ] Ticket: Add/confirm ad enums (status, placement, billing model, token event type, token target type)
* [ ] Ticket: Add migration for `cart_records.attribution_tokens` JSONB + indexes as needed
* [ ] Ticket: Add migration for `vendor_orders.attribution` JSONB + optional indexable fields
* [ ] Ticket: Add migration for `vendor_order_line_items.attribution` JSONB + optional indexable fields
* [ ] Ticket: Add `ad_daily_rollups` table schema (unique ad_id+day + indexes)
* [ ] Ticket: Ensure `usage_charges` uniqueness supports idempotent daily ad spend (store_id+type+for_date)
* [ ] Ticket: Define attribution token schema (versioning, size constraints, required fields)
* [ ] Ticket: Implement token signing + verification utility (HMAC/JWT HS256) with strict validation rules
* [ ] Ticket: Implement server-side token validation helper (signature/expiry/buyer_store match/enums/dedupe)
* [ ] Ticket: Define deterministic precedence rules (click>impression, recency, stable tie-break)
* [ ] Ticket: Implement repo to fetch eligible ads for `/ads/serve` (status/placement/time window + gating joins)
* [ ] Ticket: Implement service eligibility filters (subscription_active, kyc verified, status/time window, geo hook)
* [ ] Ticket: Implement Redis budget gate evaluation vs daily_budget_cents
* [ ] Ticket: Implement serve algorithm (highest bid wins + deterministic tie-break)
* [ ] Ticket: Implement serve DTOs returning creative + signed impression/click tokens + request_id
* [ ] Ticket: Implement route `GET /ads/serve`
* [ ] Ticket: Implement Redis impression dedupe helper (SETNX with TTL)
* [ ] Ticket: Implement route `POST /ads/impression` (verify token + increment Redis imps + spend with dedupe)
* [ ] Ticket: Implement Redis click dedupe helper (SETNX with TTL)
* [ ] Ticket: Implement route `GET /ads/click` (verify token + increment clicks + 302 redirect)
* [ ] Ticket: Update cart DTOs to accept bounded `attribution_tokens[]`
* [ ] Ticket: Normalize tokens on cart save (validate/dedupe/cap; keep most recent per key)
* [ ] Ticket: Persist normalized `cart_records.attribution_tokens` and return normalized set to client
* [ ] Ticket: Add guardrails for payload size + logs for invalid tokens; drop invalid tokens without failing checkout
* [ ] Ticket: Compute per-vendor-order attribution candidates from cart tokens at checkout-time
* [ ] Ticket: Materialize order-level attribution (store-only match by vendor_store_id) into vendor_orders.attribution
* [ ] Ticket: Materialize line-item attribution (product-only match by product_id) into line_items.attribution
* [ ] Ticket: Persist deterministic attribution reasons for debugging
* [ ] Ticket: Persist attribution within same checkout transaction (no partial writes)
* [ ] Ticket: Implement daily rollup job (read Redis day N → write ad_daily_rollups + usage_charges idempotently)
* [ ] Ticket: Implement deterministic rounding policy helper for spend calculations
* [ ] Ticket: Emit outbox event `ad_spend_rolled_up` after successful rollup transaction
* [ ] Ticket: Add Pub/Sub topic wiring and consumer skeletons for analytics + billing
* [ ] Ticket: Implement billing consumer to bridge daily usage_charges into Stripe usage/charges (stub allowed)
* [ ] Ticket: Define analytics payload contract for ads (rollups + checkout attribution snapshot)
* [ ] Ticket: Implement analytics consumer inserts for order attribution into BigQuery ad tables
* [ ] Ticket: Implement analytics consumer inserts for rollups into BigQuery rollup tables
* [ ] Ticket: Ensure one row per attributed ad per line item plus optional order-level store row
* [ ] Ticket: Define failure mode behavior for Redis outages (serve none; track fail closed) with structured logs
* [ ] Ticket: Add observability for serve/track (candidate counts, exclusion reasons, dedupe hits, winner, budget gating)
* [ ] Ticket: Add load shedding ticket (candidate limit + optional short-lived caching)
* [ ] Ticket: Add integration tests for serve→impression→click→cart token persist→checkout attribution→rollup→usage_charges

---

## Phase 34 — Ops, Observability, Hardening

**Goal:** Operational safety: visibility, recovery, and controlled rollout.

* [ ] Ticket: Implement worker metrics and DLQ visibility views/queries
* [ ] Ticket: Write replay and recovery runbooks (DLQ replay, idempotency expectations)
* [ ] Ticket: Add/standardize feature flags for risky rollouts
* [ ] Ticket: Perform backup/restore drills and document procedure
