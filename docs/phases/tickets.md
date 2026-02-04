## Stage 1 — Completed

* **Phase 1 — Repo, Infra, CI, Foundations**
  **Goal:** Deployable, observable Go monolith + worker binaries with core infra.

  * [x] Ticket [PF-000]: Initialize repository and standard Go monolith layout (`cmd/api`, `cmd/worker`, `pkg`, `internal`)
  * [x] Ticket [PF-001]: Implement config + env loading
  * [x] Ticket [PF-002]: Implement structured JSON logging with request/job correlation
  * [x] Ticket [PF-003]: Standardize API error codes + response envelopes
  * [x] Ticket [PF-004]: Wire Chi router + middleware stack
  * [x] Ticket [PF-005]: Implement standard request validation layer
  * [x] Ticket [PF-006]: Bootstrap Postgres for Cloud SQL + Heroku compatibility
  * [x] Ticket [PF-007]: Bootstrap Redis for sessions, rate limits, idempotency
  * [x] Ticket [PF-008]: Add Goose migrations runner + hybrid policy conventions
  * [x] Ticket [PF-009]: Add Dockerfile + Heroku Procfile (web + worker)
  * [x] Ticket [PF-010]: Add GitHub Actions CI (lint, test, build)
  * [x] Ticket [PF-011]: Wire worker bootstrap dependency graph

* **Phase 2 — Identity, Auth, Tenancy, RBAC**
  **Goal:** Multi-store auth with revocation + role-based access.

  * [x] Ticket [PF-020]: Implement User model with Argon2id password hashing
  * [x] Ticket [PF-021]: Implement Store model with address shape
  * [x] Ticket [PF-022]: Implement StoreMembership model with role enum
  * [x] Ticket [PF-023]: Implement JWT minting/parsing with enforced claims
  * [x] Ticket [PF-024]: Implement refresh token storage + rotation in Redis
  * [x] Ticket [PF-025]: Implement login endpoint (email/password)
  * [x] Ticket [PF-026]: Implement register endpoint (user + first store + owner membership)
  * [x] Ticket [PF-027]: Implement logout + refresh endpoints
  * [x] Ticket [PF-028]: Implement active store switching via refresh/JWT
  * [x] Ticket [PF-032]: Implement login brute-force rate limiting (Redis)
  * [x] Ticket [PF-033]: Implement store profile read endpoint
  * [x] Ticket [PF-034]: Implement store profile update endpoint
  * [x] Ticket [PF-035]: Implement store user list endpoint
  * [x] Ticket [PF-036]: Implement store invite user endpoint
  * [x] Ticket [PF-037]: Implement store remove user endpoint
  * [x] Ticket [PF-124]: Implement admin auth model + storeless admin role
  * [x] Ticket [PF-125]: Implement admin login endpoint (storeless) + token issuance
  * [x] Ticket [PF-126]: Implement dev-only admin register endpoint

* **Phase 3 — Media System**
  **Goal:** Reusable media pipeline with upload, read, list, and safe delete.

  * [x] Ticket [PF-040]: Implement media table + enums + GORM model for upload lifecycle
  * [x] Ticket [PF-038]: Implement canonical media metadata persistence
  * [x] Ticket [PF-039]: Bootstrap GCS client (API + worker)
  * [x] Ticket [PF-041]: Implement presigned PUT upload flow (create media row + signed PUT)
  * [x] Ticket [PF-042]: Implement Pub/Sub consumer for GCS `OBJECT_FINALIZE` to mark uploaded
  * [x] Ticket [PF-055]: Implement presigned READ URL generation (TTL-based)
  * [x] Ticket [PF-053]: Implement media list endpoint (store-scoped, paginated)
  * [x] Ticket [PF-054]: Implement media delete endpoint (reference-aware)
  * [x] Ticket [PF-134]: Enforce protected attachment checks on media delete

* **Phase 4 — Compliance & Licensing**
  **Goal:** License lifecycle, admin verification, store compliance gating, notifications.

  * [x] Ticket [PF-044]: Add licenses schema + GORM model (media_id required)
  * [x] Ticket [PF-045]: Implement license create endpoint (atomic metadata + media_id)
  * [x] Ticket [PF-046]: Implement license list endpoint (store-scoped)
  * [x] Ticket [PF-048]: Implement admin approve/reject license endpoint
  * [x] Ticket [PF-047]: Implement license delete endpoint (expired-only semantics)
  * [x] Ticket [PF-049]: Mirror license status to store KYC/compliance status
  * [x] Ticket [PF-050]: Emit license status outbox events + compliance notifications
  * [x] Ticket [PF-051]: Implement license expiry scheduler (14-day warn + expire)
  * [x] Ticket [PF-137]: Wire license lifecycle cron jobs into cron worker

* **Phase 5 — Async Backbone: Outbox + DLQ**
  **Goal:** Reliable side-effects with idempotency, retries, DLQ, and retention.

  * [x] Ticket [PF-060]: Implement outbox table/enums/envelope + migration
  * [x] Ticket [PF-062]: Implement outbox DTOs/registry/repo/service + wiring
  * [x] Ticket [PF-066]: Implement Redis-backed idempotency strategy (publisher + consumers)
  * [x] Ticket [PF-067]: Implement outbox dispatcher worker binary
  * [x] Ticket [PF-142]: Implement typed event→topic routing registry + payload validation
  * [x] Ticket [PF-143]: Implement DLQ table + model + repository
  * [x] Ticket [PF-145]: Publish terminal failures to DLQ
  * [x] Ticket [PF-140]: Implement outbox retention cleanup job (>30d published)
  * [x] Ticket [PF-139]: Implement notifications retention cleanup job (>30d)
  * [x] Ticket [PF-136]: Wire worker bootstrap dependency graph for scheduler/locking/metrics (cron worker base)
  * [x] Ticket [PF-141]: Finalize outbox dispatcher binary main loop

* **Phase 6 — Products, Inventory, Pricing**
  **Goal:** Vendor listings with gating and basic product CRUD.

  * [x] Ticket [PF-056]: Add products schema + GORM model + repos
  * [x] Ticket [PF-057]: Add inventory schema + GORM model + repos
  * [x] Ticket [PF-058]: Add volume discounts schema + GORM model + repos
  * [x] Ticket [PF-068]: Implement vendor create product endpoint
  * [x] Ticket [PF-069]: Implement vendor update product endpoint
  * [x] Ticket [PF-070]: Implement vendor delete product endpoint
  * [x] Ticket [PF-072]: Implement MOQ validation in product flows
  * [x] Ticket [PF-073]: Enforce visibility gating (license + subscription + state)
  * [x] Ticket [PF-128]: Patch product create route for discount type + migration alignment

* **Phase 7 — Cart Quote (Authoritative, Server-Computed)**
  **Goal:** Server-authoritative quote persisted as auditable snapshot.

  * [x] Ticket [PF-146]: Migrate cart schema to authoritative quote model (cart_records/cart_items/cart_vendor_groups)
  * [x] Ticket [PF-147]: Update/add GORM models for cart quote schema (CartRecord/CartItem/CartVendorGroup + enums)
  * [x] Ticket [PF-148]: Implement CartRecord repo upsert (active cart per buyer_store)
  * [x] Ticket [PF-149]: Implement CartItem repo replace-on-quote (delete+insert) with tx handle
  * [x] Ticket [PF-150]: Implement CartVendorGroup repo replace-on-quote with tx handle
  * [x] Ticket [PF-151]: Implement atomic transaction wrapper across record + items + vendor groups
  * [x] Ticket [PF-152]: Implement quote DTOs (`QuoteCartRequest`, `CartQuote`) + controller organization
  * [x] Ticket [PF-154]: Refactor controller/service to quote-based contract (replace legacy upsert entrypoint)
  * [x] Ticket [PF-155]: Remove client-supplied totals/pricing; quote becomes intent-only
  * [x] Ticket [PF-156]: Implement vendor preload + product fetch pipeline grouped by vendor_store_id
  * [x] Ticket [PF-157]: Implement qty normalization + item status/warnings (MOQ/max/availability/vendor mismatch)
  * [x] Ticket [PF-158]: Implement pricing + volume discount resolution and persist quote fields
  * [x] Ticket [PF-159]: Implement vendor group aggregation and persist cart_vendor_groups
  * [x] Ticket [PF-160]: Compute/persist cart totals + valid_until
  * [x] Ticket [PF-161]: Enforce quote-only invariant (no inventory mutation/reservation)
  * [x] Ticket [PF-162]: Validate vendor promo ownership + apply to vendor group totals
  * [x] Ticket [PF-163]: Emit vendor-level warnings for invalid promos (soft)
  * [x] Ticket [PF-164]: Wire `POST /cart` endpoint to DTOs/service with validation
  * [x] Ticket [PF-166]: Normalize HTTP semantics (hard errors vs soft warnings)

* **Phase 8 — Checkout Schema Refactor + Checkout Core**
  **Goal:** Cart-truth checkout with deterministic snapshots and stable schema.

  * [x] Ticket [PF-168]: Add Goose migration to drop `checkout_groups` table
  * [x] Ticket [PF-169]: Remove FK constraints referencing `checkout_groups(id)`
  * [x] Ticket [PF-170]: Ensure `vendor_orders.checkout_group_id` is UUID column (no FK)
  * [x] Ticket [PF-171]: Ensure `cart_records.checkout_group_id` remains (nullable pre-conversion)
  * [x] Ticket [PF-172]: Add/ensure indexes for checkout_group_id anchors
  * [x] Ticket [PF-173]: Add Goose migrations for `cart_records` checkout-confirmed fields (payment_method/shipping_line/converted_at)
  * [x] Ticket [PF-174]: Add Goose migrations for `vendor_orders` checkout snapshot fields + update GORM model
  * [x] Ticket [PF-175]: Add Goose migrations for `order_line_items` snapshot + attribution columns + update GORM model
  * [x] Ticket [PF-172B]: Implement checkout request DTO + controller wiring + idempotency header enforcement
  * [x] Ticket [PF-173B]: Load + validate CartRecord by `(buyer_store_id, cart_id)` inside DB tx
  * [x] Ticket [PF-174B]: Finalize cart during checkout (persist confirmed fields, converted_at, status, checkout_group_id anchor)
  * [x] Ticket [PF-175B]: Create vendor orders + line items deterministically from cart snapshot
  * [x] Ticket [PF-178]: Reserve inventory at checkout only + map failures to line items + deterministic totals recompute
  * [x] Ticket [PF-180]: Remove legacy checkout artifacts and fully migrate to cart-truth checkout flow

* **Phase 9 — Orders Lifecycle + Agents + Ledger + Notifications**
  **Goal:** Post-checkout reads, decisioning, delivery, payout ops, and in-app notifications.

  * [x] Ticket [PF-084]: Add missing order indexes for list/detail/action queues
  * [x] Ticket [PF-085]: Implement buyer orders list repo/service (filters + pagination)
  * [x] Ticket [PF-086]: Implement vendor orders list repo/service (filters + pagination)
  * [x] Ticket [PF-087]: Implement order detail repo/service (preload line items + payment intent)
  * [x] Ticket [PF-088]: Implement orders list endpoint (`GET /api/v1/orders`)
  * [x] Ticket [PF-089]: Implement order detail endpoint (`GET /api/v1/orders/{orderId}`)
  * [x] Ticket [PF-089B]: Implement vendor order decision endpoint (order-level transitions)
  * [x] Ticket [PF-090]: Implement vendor line-item decision endpoint (accept/reject + recompute + release inventory)
  * [x] Ticket [PF-091]: Emit outbox events for order decisioning (order_decided with line-level detail)
  * [x] Ticket [PF-092]: Implement buyer cancel endpoint (pre-transit) + inventory release + emit outbox
  * [x] Ticket [PF-092B]: Implement buyer nudge endpoint (notification event)
  * [x] Ticket [PF-092C]: Implement buyer retry endpoint (expired-only) to create new attempt
  * [x] Ticket [PF-138]: Implement order TTL cron job (nudge → expire → inventory release) and emit outbox
  * [x] Ticket [PF-093]: Ensure `users.system_role=agent` auth path works end-to-end
  * [x] Ticket [PF-095]: Add `order_assignments` migration
  * [x] Ticket [PF-096]: Implement assignment creation for dispatchable orders (random auto-assign MVP)
  * [x] Ticket [PF-097]: Implement agent queue endpoint (unassigned hold orders)
  * [x] Ticket [PF-098]: Implement agent “my assignments” endpoint
  * [x] Ticket [PF-099]: Implement agent pickup endpoint (status `in_transit`)
  * [x] Ticket [PF-100]: Implement agent deliver endpoint (status `delivered`)
  * [x] Ticket [PF-102]: Add `ledger_events` table + indexes (append-only)
  * [x] Ticket [PF-103]: Implement ledger append helpers (repo + service)
  * [x] Ticket [PF-104]: Implement admin payout queue endpoints
  * [x] Ticket [PF-105]: Implement admin payout detail endpoint
  * [x] Ticket [PF-105B]: Implement admin confirm payout endpoint (ledger event + payment intent paid + order closed + outbox)
  * [x] Ticket [PF-107]: Add notifications schema + indexes
  * [x] Ticket [PF-108]: Implement notifications list endpoint (`GET /api/v1/notifications`) with unread filter
  * [x] Ticket [PF-109]: Implement mark notification read + mark-all read endpoints
  * [x] Ticket [PF-139B]: Implement notifications retention cleanup scheduler (>30d)

* **Phase 10 — Subscriptions & Billing (Stripe)**
  **Goal:** Vendor subscription gating + billing history + webhook sync.

  * [x] Ticket [PF-113]: Bootstrap Stripe client + config/secrets
  * [x] Ticket [PF-114]: Add billing/subscription schema (`subscriptions`, `payment_methods`, `charges`, `usage_charges`)
  * [x] Ticket [PF-115]: Implement create/cancel/get subscription endpoints (idempotent)
  * [x] Ticket [PF-116]: Implement Stripe webhook consumer to mirror subscription state to `stores.subscription_active`
  * [x] Ticket [PF-117]: Implement vendor billing history endpoint (`GET /api/v1/vendor/billing/charges`)
  * [x] Ticket [PF-118]: Enforce subscription gating across marketplace browse/search

* **Phase 11 — BigQuery Analytics (Marketplace) + Analytics Worker (Partial)**
  **Goal:** Marketplace event ingestion and vendor analytics endpoints.

  * [x] Ticket [PF-110]: Implement BigQuery client bootstrap + readiness checks
  * [x] Ticket [PF-111]: Implement outbox consumers that insert BigQuery rows for `order_created`, `cash_collected`, `order_paid`
  * [x] Ticket [PF-112]: Implement vendor analytics endpoint (`GET /api/v1/vendor/analytics`) with presets + KPIs + series
  * [x] Ticket [PF-181]: Clean up logging for workers
  * [x] Ticket [PF-182]: Define canonical analytics event enums + Pub/Sub envelope DTO + BQ row DTOs + helpers
  * [x] Ticket [PF-183]: Create `cmd/analytics-worker` Pub/Sub consumer with Redis idempotency gate + ACK/NACK policy + shutdown
  * [x] Ticket [PF-184]: Implement analytics router (`switch(event_type)`) + handler interfaces/contracts
  * [x] Ticket [PF-185]: Implement BigQuery writer abstraction + retry + JSON serialization helpers
  * [x] Ticket [PF-186]: Map `order_created` payload to `marketplace_events` row + item snapshots + geo extraction + revenue fields
  * [x] Ticket [PF-188]: Handle marketplace ingestion for `order_paid`, `cash_collected`
  * [x] Ticket [PF-189]: Handle marketplace ingestion for `order_canceled`, `order_expired`
  * [x] Ticket [PF-191]: Implement marketplace analytics query service (BQ-backed) with series + KPIs + top queries
  * [x] Ticket [PF-193]: Implement marketplace analytics API endpoints backed by BigQuery query service

* **Phase 12 — Media Attachments + Workers (Partial)**
  **Goal:** Canonical attachment table and core worker binaries (cron/outbox/media-delete).

  * [x] Ticket [PF-129]: Finalize `media_attachments` table + indexes
  * [x] Ticket [PF-131]: Implement canonical media attachment reconciliation helper
  * [x] Ticket [PF-136B]: Implement `cmd/cron-worker` binary with scheduler registry + locking + metrics + concurrency decision doc
  * [x] Ticket [PF-137B]: Implement license lifecycle jobs in cron worker (warn/expire)
  * [x] Ticket [PF-138B]: Implement order TTL job in cron worker (nudge/expire/release)
  * [x] Ticket [PF-139C]: Implement notification cleanup job in cron worker
  * [x] Ticket [PF-140B]: Implement outbox cleanup job in cron worker
  * [x] Ticket [PF-141B]: Finalize `cmd/outbox-publisher` main loop (`FOR UPDATE SKIP LOCKED`, jitter) + batch size config + retries + MAX_ATTEMPTS + DLQ write
  * [x] Ticket [PF-133]: Create `cmd/media-delete-worker` binary and consume `media_deleted` events

* **Phase 13 — Integration Harness (Partial Scaffold)**
  **Goal:** Initial integration scaffold exists (not complete).

  * [x] Ticket [PF-119]: Create initial integration test scaffold under scripts
  * [x] Ticket [PF-120]: Implement initial register scripts (partial/dud per note)

---

## Stage 2 — MVP Next

* **Phase 1 — Auth & Contract Hardening**
  **Goal:** Close auth gaps, remove deprecated behaviors, and lock contracts with tests.

  * [x] Ticket [PF-195]: Add "names" field to the agent registration DTO
  * [x] Ticket [PF-196]: Add auth middleware tests (missing/expired token, revoked session, missing activeStoreId)
  * [x] Ticket [PF-197]: Add RBAC guard tests for `/api/admin/*` and `/api/v1/agent/*`
  * [x] Ticket [PF-198]: Remove store IDs from User model/object (Goose migration + models + helpers)
  * [x] Ticket [PF-199]: Remove access + refresh token from login/register response bodies; return only via headers
  * [x] Ticket [PF-200]: Modify register flow to allow creating a store when user already exists (user can own 0..N stores)

* **Phase 2 — Media System Correctness + Lifecycle Jobs**
  **Goal:** Fix media edge cases and make deletion/upload lifecycle operationally safe.

  * [x] Ticket [PF-201]: Prevent generating READ URLs for `media.status=pending` responses
  * [x] Ticket [PF-202]: Change GCS object key to `{storeId}/{media_kind}/{mediaId}.{ext}` (stop using filename)

  * [x] Ticket [PF-203]: Add delete media worker to Docker/Heroku/Make targets for deploy parity
  * [x] Ticket [PF-204]: Extend (`cmd/cron-worker/main.go`)  to Implement stale pending media deletion pending uploads after 7 days
  * [ ] Ticket [PF-205]: Detach all attachment references by entity type (`cmd/media_deleted_worker/main.go`) & if necessary Delete GCS originals + derived artifacts if no longer attatched. 

  * [x] Ticket [PF-206]: Fix media delete returning 200 but not deleting media or gcs object (end-to-end verification + logs + worker outcomes)

* **Phase 3 — Compliance + Admin Ops Gaps**
  **Goal:** Finish compliance retention, admin queues, and auditability needed for real ops.

  * [ ] Ticket [PF-207]: Implement admin license queue list endpoint (pending verification, paginated)
  * [ ] Ticket [PF-208]: Add audit log rows for admin verify/reject + scheduler expiry flip

* **Phase 4 — Products & Inventory Completion**
  **Goal:** Finish vendor inventory management endpoints and missing product constraints.

  * [x] Ticket [PF-209]: Implement product list endpoint (buyer/vendor) with pagination + product summary (this is the product grid UI view) (`GET /v1/vendor/products`). It will be qith additional queries to filter by category, thc / cbs range, price range, classification, has promo (volume discount), search (q=..)
  * [x] Ticket [PF-210]: Implement product list endpoint (vendor-only) with pagination + product summary (this is the table view of the UI) (`GET /v1/products`). It will be qith additional queries to filter by category, thc / cbs range, price range, classification, has promo (volume discount), search (q=..)
  * [ ] Ticket [PF-211]: Add VendorSummary to product browse/detail (join stores + optional logo attachment)

  * [x] Ticket [PF-212]: Add `low_stock_threshold` to inventory model (migration + model + DTOs) & Add `max_qty` to product model (migration + model + DTOs + validations)
  * [x] Ticket [PF-213]: Volume discount from `unit_price_cents` to percentage & all areas here used (cart/checkout etc). Its to be a percentage value discounted from the line item (product) not a dollar amount (fixed). 
  * [x] Ticket [PF-214]: Add `media_id` to product media model (migration + model + DTOs + validations)

  * [ ] Ticket [PF-215]: Implement audit action schema/helper for product/inventory actions
  * [ ] Ticket [PF-216]: Emit audit rows on product create/update/delete + inventory set

* **Phase 5 — Cart Quote Guardrails + Idempotency + Attribution Pass-through**
  **Goal:** Make cart quoting robust (idempotency, mapping helpers, expiry behavior, attribution token plumbing).

  * [x] Ticket [PF-217]: Implement cart attribution token pass-through (validate signature/expiry only, persist on cart_records, echo in CartQuote)
  * [x] Ticket [PF-218]: Enforce `valid_until` guardrail: if expired, require re-quote before checkout (15m from quote/fetch response)
  * [ ] Ticket [PF-219]: Implement cart conversion readiness (active→converted transition + generate/persist checkout_group_id at conversion) & Implement “converted cart” behavior guardrails (reject future quote-upserts if desired)
  * [ ] Ticket [PF-220]: Add rate limiting for `POST /cart`

  * [ ] Ticket [PF-221]: Implement cart quote mapping helpers (DB ↔ domain ↔ DTO) with unit tests
  * [ ] Ticket [PF-222]: Add structured logs + metrics in quote service (counts, warnings, duration)

* **Phase 6 — Checkout Completion: Payment Intents + Retry Safety + Outbox Exactly-Once-ish**
  **Goal:** Close the remaining checkout core so retries are safe and downstream systems receive canonical events.

  * [x] Ticket [PF-234]: Create one payment_intent per vendor order inside checkout transaction
  * [x] Ticket [PF-235]: Set payment intent amount from `vendor_orders.total_cents` & Set payment intent `payment_method` from checkout-confirmed `payment_method`
  * [x] Ticket [PF-236]: Add repo helper to fetch full checkout result by checkout_group_id & endpoint

  * [x] Ticket [PF-237]: Define/extend outbox payload for Notifications checkout-converted event & Emit Notifications outbox event in same transaction as vendor order creation
  * [x] Ticket [PF-238]: Define/extend outbox payload for Analytics checkout-converted event (cart totals + attribution `ad_tokens`) & Emit Analytics outbox event in same transaction as cart conversion

  * [x] Ticket [PF-239] - [PF-240]: Prevent duplicate vendor orders on retry (uniqueness anchored on checkout_group_id+vendor_store_id and/or cart_id) & Prevent duplicate outbox rows on retry for same conversion anchor

  * [ ] Ticket [PF-241]: Add outbox payload versioning rules for these events
  * [ ] Ticket [PF-242]: Add checkout regression tests (idempotent retry, expired/already converted behavior, exactly two outbox events)

* **Phase 7 — Orders + Fulfillment + Cash Collection Completion**
  **Goal:** Finish the operational lifecycle for vendors/agents and cash settlement.

  * [ ] Ticket [PF-243]: Transition fulfilled orders into hold/ready-for-dispatch semantics when all items in the order are no longer pending & then Emit outbox event `order_ready_for_dispatch` on fulfillment for admin and agents (one dispatch for both)

  * [x] Ticket [PF-244]: Implement agent cash-collected endpoint (`POST /api/v1/agent/orders/{orderId}/cash-collected`) & Append `ledger_events(cash_collected)` during cash-collected flow
  * [ ] Ticket [PF-245]: Set `payment_intents.status=settled` + `cash_collected_at` & Emit outbox event `cash_collected` & update the order states too.
  * [ ] Ticket [PF-246]: Reject cash collection when order is not dispatch-ready & create endpoint (`POST /agent/orders/{orderId}/cash-collected`)
  * [ ] Ticket [PF-247]: Prevent duplicate cash collection on already settled or paid orders
  * [ ] Ticket [PF-248]: Mark payment intent as failed when cash collection validation fails
  * [ ] Ticket [PF-249]: Support rejected payment state for future ACH or admin-declined settlements
  * [ ] Ticket [PF-250]: Emit payment_failed or payment_rejected outbox events for downstream consumers

* **Phase 8 — Attachment Wiring for Core Domains**
  **Goal:** Make attachments usable across MVP surfaces and keep delete semantics correct.

  * [ ] Ticket [PF-XXX]: Wire license ↔ media attachments
  * [ ] Ticket [PF-XXX]: Wire product ↔ media attachments (gallery + COA)
  * [ ] Ticket [PF-XXX]: Wire store ↔ media attachments (logo/banner)
  * [ ] Ticket [PF-XXX]: Wire user ↔ media attachments (avatar)

* **Phase 9 — Analytics MVP Completion (Admin View + Test Coverage)**
  **Goal:** Provide global analytics and lock ingestion/query correctness.

  * [ ] Ticket [PF-XXX]: Implement admin analytics endpoint (`GET /api/v1/admin/analytics`) for global KPIs
  * [ ] Ticket [PF-XXX]: Add unit tests for marketplace mapping logic
  * [ ] Ticket [PF-XXX]: Add unit tests for analytics SQL builders
  * [ ] Ticket [PF-XXX]: Add integration test harness (handler → BQ writer)

* **Phase 10 — DLQ + Ops Runbooks (Minimum Viable)**
  **Goal:** Make failures recoverable and observable by an operator.

  * [ ] Ticket [PF-XXX]: Document and implement DLQ retry policy + MAX_ATTEMPTS conventions (minimal hooks)
  * [ ] Ticket [PF-XXX]: Create DLQ replay tooling/runbook (safe requeue + idempotency expectations)


* **Phase 11 — Subscription Finalization**
  **Goal:** 

> Subscription Plan Catalog (Multi-Plan, Config-Driven) Support 2 monthly + 2 annual plans (annual = monthly-equivalent with % discount), and make plan changes safe without code rewrites.

  * [ ] Ticket [PF-XXX]: Define canonical “plan catalog” shape (PlanID, name, interval, price_cents, discount_percent, stripe_price_id, is_active, sort_order)
  * [ ] Ticket [PF-XXX]: Implement config loader for plan catalog (env/JSON) with schema validation + startup fail-fast on invalid config
  * [ ] Ticket [PF-XXX]: Add persistence strategy for plan catalog (decision + wiring): **config-only** vs **DB-backed mirror** (pick one and implement)
  * [ ] Ticket [PF-XXX]: Add `subscriptions.plan_id` (and/or `stripe_price_id`) and store the chosen plan at purchase time
  * [ ] Ticket [PF-XXX]: Implement plan resolution helper (given plan_id → returns Stripe price id + normalized billing interval metadata)
  * [ ] Ticket [PF-XXX]: Add endpoint to list available subscription plans (`GET /api/v1/subscriptions/plans`) store-scoped and filtered by `is_active`

> Subscription API & Service Refactor (Plan-Aware) Make create/cancel/get subscription endpoints handle any number of plans without special-casing.

  * [ ] Ticket [PF-XXX]: Update subscription create DTO to accept `plan_id` only (remove Stripe IDs from request contract)
  * [ ] Ticket [PF-XXX]: Update subscription service to resolve plan via catalog and pass only server-resolved Stripe identifiers to Stripe client
  * [ ] Ticket [PF-XXX]: Implement subscription “change plan” behavior decision (MVP: disallow vs allow). If allow: add endpoint + service flow.
  * [ ] Ticket [PF-XXX]: Update subscription repo queries/indexes to support plan_id + store_id lookup patterns cleanly
  * [ ] Ticket [PF-XXX]: Add migration/index to enforce “one active subscription per store” (or explicitly allow multiple if intended; pick and enforce)

> Payment Method Sourcing (DB-First, No Client Pass-Through) Pull payment method from DB using `activeStoreId`; never accept StripeCustomerID / StripePaymentMethodID from request.

  * [ ] Ticket [PF-XXX]: Update `payment_methods` table usage to support “default payment method on file” per store (fields + index if missing)
  * [ ] Ticket [PF-XXX]: Implement repo helper: fetch StripeCustomerID + default StripePaymentMethodID by `store_id` (active store)
  * [ ] Ticket [PF-XXX]: Remove these fields from create-subscription request handling:

  * `payload.StripeCustomerID`
  * `payload.StripePaymentMethodID`
  * [ ] Ticket [PF-XXX]: Update subscription create flow to:

  * load store billing identity (customer id)
  * load default payment method id
  * fail with typed error if missing/invalid
  * [ ] Ticket [PF-XXX]: Add endpoint to “set default payment method” (if not already present) using existing Stripe customer/payment method objects

> Edge Cases & Failure Modes (PM Missing/Invalid/Expired) Robust behavior when billing data is missing, stale, or rejected by Stripe.

  * [ ] Ticket [PF-XXX]: Define error taxonomy for billing failures (no PM on file, invalid PM, customer missing, Stripe hard decline, Stripe transient)
  * [ ] Ticket [PF-XXX]: Implement “no default PM on file” handling (clear response + next-step messaging)
  * [ ] Ticket [PF-XXX]: Implement “payment method invalid” handling (detect via Stripe error codes; mark PM as invalid in DB if appropriate)
  * [ ] Ticket [PF-XXX]: Add retry-safe idempotency behavior for create subscription when Stripe returns ambiguous/transient errors
  * [ ] Ticket [PF-XXX]: Add webhook-driven reconciliation rule: if Stripe says payment method detached/expired → update local payment_methods state

> Webhooks & State Sync Hardening (Plans + Billing Identity) Ensure `stores.subscription_active` and subscription rows remain correct across plan changes, cancellations, and Stripe-side updates.

  * [ ] Ticket [PF-XXX]: Extend webhook consumer mapping to persist: plan_id/stripe_price_id, interval, current_period_end, cancel_at_period_end, status
  * [ ] Ticket [PF-XXX]: Implement “catalog mismatch” guard: webhook arrives with unknown stripe_price_id → log + DLQ (don’t silently corrupt state)
  * [ ] Ticket [PF-XXX]: Add reconciliation job: periodically re-fetch subscription from Stripe for “recently failing/updated” records (minimal MVP cadence)

> Tests & Integration Coverage (Billing Contracts Locked) Prevent regressions while you add plan variants and DB-sourced payment methods.

  * [ ] Ticket [PF-XXX]: Unit tests: plan catalog parsing/validation (missing fields, duplicate ids, invalid discounts)
  * [ ] Ticket [PF-XXX]: Unit tests: plan resolution helper (plan_id → stripe price id; annual discount logic invariants)
  * [ ] Ticket [PF-XXX]: Unit tests: subscription create service uses DB PM and rejects request-supplied Stripe IDs
  * [ ] Ticket [PF-XXX]: Unit tests: edge cases (no PM, invalid PM, customer missing)
  * [ ] Ticket [PF-XXX]: Integration script: create store → attach payment method → create monthly → cancel → create annual (happy path)

> Ops Guardrails (Minimum Viable) Make billing debuggable in production without “SSH and pray”.

  * [ ] Ticket [PF-XXX]: Add structured logs for billing actions (store_id, subscription_id, plan_id, stripe ids redacted, idempotency key)
  * [ ] Ticket [PF-XXX]: Add metrics counters for billing outcomes (created/canceled/failed + reason buckets)
  * [ ] Ticket [PF-XXX]: Add admin-only endpoint to view store billing identity summary (customer exists, default PM exists, last charge outcome)

* **Phase 12 — Integration Test Harness (End-to-End Happy Path)**
  **Goal:** Deterministic scripts that validate the full MVP pipeline on real deployments.

  * [x] Ticket [PF-232]: Implement scripted register/login flows (buyer + vendor) and token store/header injection helpers (happy/failure paths)
  * [x] Ticket [PF-233]: Implement scripted to build and create new products + set inventory (happy/failure paths)
  * [ ] Ticket [PF-XXX]: Implement Script media create presigned URL & upload (via files from our `fixtures/media/*` folder) including image/video/PDF (`product`, `license_doc`, `coa`) & polling until uplaoded (GCS PUT) is possible (happy/failure paths)
  * [ ] Ticket [PF-XXX]: Implement Script create license from `license_doc` media uplaoded (happy/failure paths) + admin login -> approve/reject (happy/failure paths)
  * [ ] Ticket [PF-XXX]: Implement Script cart → checkout → orders (happy/failure paths)
  * [ ] Ticket [PF-XXX]: Implement Script orders → vendor approve/reject + fulfill/reject line item(s) + emit agent dispatch order state change (happy/failure paths)
  * [ ] Ticket [PF-XXX]: Implement Script orders → agent deliver pickup + drop off + cash collected → payout (happy/failure paths) 
  * [ ] Ticket [PF-XXX]: Implement Script orders → Vendor/Buyer cancel order (happy/failure paths) 
  * [ ] Ticket [PF-XXX]: Implement Script orders → Expire order + nudge vendor + expire order  & release inventory (happy/failure paths) 

---

## Stage 3 — Post-MVP Nice-to-have

* **Phase 1 — Media Processing Worker (Compression + OCR)**
  **Goal:** Automated derived assets and OCR pipeline (beyond core upload/delete).

  * [ ] Ticket [PF-279]: Create `cmd/media-worker` binary
  * [ ] Ticket [PF-280]: Subscribe media-worker to GCS finalize events (env-driven)
  * [ ] Ticket [PF-281]: Implement image/video compression helper
  * [ ] Ticket [PF-282]: Implement OCR provider abstraction (OpenAI vs Document AI)
  * [ ] Ticket [PF-283]: Implement OCR text generation + store derived asset (`ocr.txt`)
  * [ ] Ticket [PF-284]: Update media row with derived artifacts
  * [ ] Ticket [PF-285]: Emit `media_processed` outbox event

* **Phase 2 — Notifications: Email Pipeline**
  **Goal:** Send email notifications via adapter interface (SendGrid later).

  * [ ] Ticket [PF-286]: Set up sendgrid client and wire up to notifications pubsub consumer
  * [ ] Ticket [PF-286]: Define notification email templates
  * [ ] Ticket [PF-287]: Define email sender interface
  * [ ] Ticket [PF-288]: Impliment 
  * [ ] Ticket [PF-289]: Implement SendGrid adapter (future swap)

* **Phase 3 — COA → OpenAI Product Drafts**
  **Goal:** Parse COA PDFs into structured product drafts.

  * [ ] Ticket [PF-290]: Implement OpenAI client bootstrap
  * [ ] Ticket [PF-291]: Implement COA OCR → structured parser
  * [ ] Ticket [PF-292]: Implement product draft JSON generator from parsed COA
  * [ ] Ticket [PF-293]: Persist product draft + status

* **Phase 4 — Checkout Refactor Safety Locks (Docs/Flags)**
  **Goal:** Formalize sequencing and guardrails for future checkout iterations.

  * [ ] Ticket [PF-294]: Write `CHECKOUT_REFACTOR.md` sequencing locks
  * [ ] Ticket [PF-295]: Add feature flag/config guard for selecting checkout flow entrypoint (if parallel flows exist)
  * [ ] Ticket [PF-296]: Commit grep checklist for all references to `checkout_groups` and `AttributedAdClickID`

* **Phase 5 — Optional Admin/Vendor Finance UX**
  **Goal:** Convenience endpoints that don’t affect authoritative finance state.

  * [ ] Ticket [PF-297]: Implement optional vendor “confirm paid” endpoint (audited; non-authoritative)

---

## Stage 4 — Deferred / Parked

* **Phase 1 — Ads Engine (Full CPM System)**
  **Goal:** Full CPM ad engine with serve/track, tokens, checkout attribution, rollups, billing fanout, analytics.

  * [ ] Ticket [PF-300]: Define ad engine constants + Redis key schema + TTL conventions
  * [ ] Ticket [PF-301]: Add/confirm ad enums (status, placement, billing model, token event type, token target type)
  * [ ] Ticket [PF-302]: Add migration for `cart_records.attribution_tokens` JSONB + indexes
  * [ ] Ticket [PF-303]: Add migration for `vendor_orders.attribution` JSONB + optional indexable fields
  * [ ] Ticket [PF-304]: Add migration for `vendor_order_line_items.attribution` JSONB + optional indexable fields
  * [ ] Ticket [PF-305]: Add `ad_daily_rollups` table schema (unique ad_id+day + indexes)
  * [ ] Ticket [PF-306]: Ensure `usage_charges` uniqueness supports idempotent daily ad spend (store_id+type+for_date)
  * [ ] Ticket [PF-307]: Define attribution token schema (versioning, size constraints, required fields)
  * [ ] Ticket [PF-308]: Implement token signing + verification utility (HMAC/JWT HS256) with strict validation rules
  * [ ] Ticket [PF-309]: Implement server-side token validation helper (signature/expiry/buyer_store match/enums/dedupe)
  * [ ] Ticket [PF-310]: Define deterministic precedence rules (click>impression, recency, stable tie-break)
  * [ ] Ticket [PF-311]: Implement repo to fetch eligible ads for `/ads/serve` (status/placement/time window + gating joins)
  * [ ] Ticket [PF-312]: Implement service eligibility filters (subscription_active, kyc verified, status/time window, geo hook)
  * [ ] Ticket [PF-313]: Implement Redis budget gate evaluation vs daily_budget_cents
  * [ ] Ticket [PF-314]: Implement serve algorithm (highest bid wins + deterministic tie-break)
  * [ ] Ticket [PF-315]: Implement serve DTOs (creative + signed impression/click tokens + request_id)
  * [ ] Ticket [PF-316]: Implement route `GET /ads/serve`
  * [ ] Ticket [PF-317]: Implement Redis impression dedupe helper (SETNX with TTL)
  * [ ] Ticket [PF-318]: Implement route `POST /ads/impression` (verify token + increment Redis imps + spend with dedupe)
  * [ ] Ticket [PF-319]: Implement Redis click dedupe helper (SETNX with TTL)
  * [ ] Ticket [PF-320]: Implement route `GET /ads/click` (verify token + increment clicks + 302 redirect)
  * [ ] Ticket [PF-321]: Update cart DTOs to accept bounded `attribution_tokens[]`
  * [ ] Ticket [PF-322]: Normalize tokens on cart save (validate/dedupe/cap; keep most recent per key)
  * [ ] Ticket [PF-323]: Persist normalized `cart_records.attribution_tokens` and return normalized set to client
  * [ ] Ticket [PF-324]: Add guardrails for payload size + logs for invalid tokens; drop invalid tokens without failing checkout
  * [ ] Ticket [PF-325]: Compute per-vendor-order attribution candidates from cart tokens at checkout-time
  * [ ] Ticket [PF-326]: Materialize order-level attribution into vendor_orders.attribution (vendor_store_id match)
  * [ ] Ticket [PF-327]: Materialize line-item attribution into line_items.attribution (product_id match)
  * [ ] Ticket [PF-328]: Persist deterministic attribution reasons for debugging
  * [ ] Ticket [PF-329]: Persist attribution within same checkout transaction (no partial writes)
  * [ ] Ticket [PF-330]: Implement daily rollup job (read Redis day N → write ad_daily_rollups + usage_charges idempotently)
  * [ ] Ticket [PF-331]: Implement deterministic rounding policy helper for spend calculations
  * [ ] Ticket [PF-332]: Emit outbox event `ad_spend_rolled_up` after successful rollup transaction
  * [ ] Ticket [PF-333]: Add Pub/Sub topic wiring and consumer skeletons for analytics + billing
  * [ ] Ticket [PF-334]: Implement billing consumer to bridge daily usage_charges into Stripe usage/charges (stub allowed)
  * [ ] Ticket [PF-335]: Define analytics payload contract for ads (rollups + checkout attribution snapshot)
  * [ ] Ticket [PF-336]: Implement analytics consumer inserts for order attribution into BigQuery ad tables
  * [ ] Ticket [PF-337]: Implement analytics consumer inserts for rollups into BigQuery rollup tables
  * [ ] Ticket [PF-338]: Ensure one row per attributed ad per line item plus optional order-level store row
  * [ ] Ticket [PF-339]: Define failure mode behavior for Redis outages (serve none; fail closed) with structured logs
  * [ ] Ticket [PF-340]: Add observability for serve/track (candidate counts, exclusion reasons, dedupe hits, winner, budget gating)
  * [ ] Ticket [PF-341]: Add load shedding (candidate limit + optional short-lived caching)
  * [ ] Ticket [PF-342]: Add integration tests for serve→impression→click→cart token persist→checkout attribution→rollup→usage_charges

* **Phase 2 — Analytics Ads Extensions (Without Full Ads Engine)**
  **Goal:** Prepare ad analytics plumbing and attribution utilities without requiring ads for MVP.

  * [ ] Ticket [PF-271]: Support inserts into `ad_event_facts`
  * [ ] Ticket [PF-272]: Implement attribution token decode utilities for analytics
  * [ ] Ticket [PF-273]: Implement deterministic token selection strategy (last-applicable)
  * [ ] Ticket [PF-274]: Attribute store-level ads to full order revenue
  * [ ] Ticket [PF-275]: Attribute product-level ads to matching line-item revenue
  * [ ] Ticket [PF-276]: Emit `ad_event_facts` rows with `type=conversion`
  * [ ] Ticket [PF-277]: Implement ad analytics query service (spend/impressions/clicks/ROAS/time series)
  * [ ] Ticket [PF-278]: Implement ad analytics API endpoints + response shaping + advertiser ownership enforcement

* **Phase 3 — Ops, Observability, Hardening**
  **Goal:** Operational safety improvements beyond MVP.

  * [ ] Ticket [PF-343]: Implement worker metrics and DLQ visibility views/queries
  * [ ] Ticket [PF-344]: Write replay and recovery runbooks (DLQ replay, idempotency expectations)
  * [ ] Ticket [PF-345]: Add/standardize feature flags for risky rollouts
  * [ ] Ticket [PF-346]: Perform backup/restore drills and document procedure
