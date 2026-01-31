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

* [X] Stripe client bootstrap + config/secrets
* [X] Migrations: `subscriptions`, `payment_methods`, `charges`, `usage_charges` (if not already applied) (models, repos, enums & services)

### 12B) Subscription flows

Combine these:
* [X] `POST /api/v1/vendor/subscriptions` (create subscription, idempotent)
* [X] `POST /api/v1/vendor/subscriptions/cancel` (idempotent)
* [X] `GET /api/v1/vendor/subscriptions` -> there should be a one to 0 or a one to one relationship (only return the single active sub)

seperate webhook endpoint we can link or subscribe to 
* [X] Webhook consumer (Stripe) updates subscription state + mirrors `stores.subscription_active`

### 12C) Billing history

* [X] `GET /api/v1/vendor/billing/charges` (ads + subscriptions)

* [X] Enforce gating everywhere:
  * [X] browse/search hides vendor listings if `subscription_active=false` (should be done already but verify)


---

## Phase 13 — Integration Test Harness (API-Level, Scripted)

**Goal:** Deterministic, repeatable, end-to-end validation using *real HTTP calls* (not DB seeding).

### 13A) Integration harness foundation (infra only)

* [ ] **Ticket:** Create `/scripts/integration/` scaffold
  *Purpose:* Dedicated integration test entrypoint
  *Notes:*

  * Callable via `make integration-test`
  * Reads `API_BASE_URL`, `[STORE|BUYER|ADMIN|AGENT]_EMAIL`, `[STORE|BUYER|ADMIN|AGENT]_PASSWORD` from env for passowrd where the email could be a var set inside the script
  * No domain logic yet
  * Load the env vars to be used in the scripts

* [ ] **Ticket:** Implement shared HTTP client helper
  *Scope:* one file
  *Includes:* base URL, retries, timeout, JSON encode/decode, status assertions

* [ ] **Ticket:** Implement colored JSON console logger
  *Purpose:* readable stdout for CI + humans
  *Explicitly:* no business logic, logging only

---

### 13B) Auth flows (script-only, no backend changes)

* [ ] **Ticket:** Scripted register flow (buyer + vendor flags)
  *Creates:* buyer store, vendor store
  *Outputs:* store IDs, user IDs, access token + refresh token

* [ ] **Ticket:** Scripted login flow (buyer + vendor flags)
  *Consumes:* email (inline)/password (env var)
  *Outputs:* access token + refresh token

* [ ] **Ticket:** In-memory token store helper
  *Purpose:* persist token across script steps
  *Explicit:* no file persistence yet store in terminal var to be used echo. 

* [ ] **Ticket:** Auth header injection helper
  *Guarantee:* no script manually sets headers token from the previous ticket

---

### 13C) Media seeding (pure media pipeline validation)

* [ ] **Ticket:** Add static media fixtures
  *Path:* `fixtures/media/*`
  *Includes:* image, video, PDF (COA-like) (image for product/ad, video for prodcut/ad, image for avatar, image for logo, image for store banner, PDF for license PDF for coa)

* [ ] **Ticket:** Script: request presigned upload URL
  *Scope:* media create endpoint only
  *Outputs:* media_id + signed URL -> store that signed URL to be used in the next ticket

* [ ] **Ticket:** Script: stream file upload to GCS
  *Explicit:* upload via signed URL (PUT)
  *No polling yet*

* [ ] **Ticket:** Script: poll media status until `uploaded`
  *Stops at:* timeout or success
  *Guarantee:* downstream steps only run with valid media IDs

---

### 13D) Domain seeding (linear, dependency-aware)

* [ ] **Ticket:** Script: create license using media_id
  *Consumes:* media IDs from 13C
  *Validates:* license is `pending`

* [ ] **Ticket:** Script: admin approve / reject license
  *Includes:* admin login + decision toggle

* [ ] **Ticket:** Script: create product with gallery + COA + set inventory
  *Consumes:* multiple media IDs | null
  *Validates:* product visible after approval

* [ ] **Ticket:** Script: product(s) → cart → checkout → order → agent deliver → payout
  *Guarantee:* full happy-path money flow works

---

## Phase 14 — Media Attachments (Canonical Linking Layer)

**Goal:** One normalized attachment model with safe delete semantics.

### 14A) Attachment schema & rules (data only)

* [X] **Ticket:** Finalize `media_attachments` table
  *Fields:* media_id, entity_type, entity_id, store_id, created_at, gcs_key
  *Indexes:* entity lookup + media lookup

* [X] **Ticket:** Define attachment lifecycle rules (code comments + docs)
  *Explicit:*

  * One attachment row per usage
  * Media delete requires zero “protected” attachments (licenses)

---

### 14B) Domain integrations (split intentionally)

> These are intentionally **separate tickets** to avoid cross-domain edits.

* [ ] **Ticket:** License ↔ media attachment wiring
* [ ] **Ticket:** Product ↔ media attachment wiring (gallery + COA)
* [ ] **Ticket:** Store ↔ media attachment wiring (logo/banner)
* [ ] **Ticket:** User ↔ media attachment wiring (avatar)
* [ ] **Ticket:** Ad ↔ media attachment wiring

---

## Phase 15 — Workers (Dedicated Binaries, One Responsibility Each)

---

### 15A) Daily Cron Worker (time-based orchestration only)

**Goal:** All time-based invariants, zero Pub/Sub.

* [X] **Ticket:** Create `cmd/cron-worker` binary
  *Includes:* scheduler registry, locking, metrics

* [X] **Ticket:** License lifecycle jobs
  *Jobs:* 14d warning, expired, >30d hard delete

* [X] **Ticket:** Order TTL job
  *Jobs:* nudge → expire → inventory release

* [X] **Ticket:** Notification cleanup job (>30d)

* [X] **Ticket:** Outbox cleanup job (>30d published)

* [X] **Ticket:** Concurrency model decision
  *Explicit:* sequential vs goroutines (documented rationale)

---

### 15B) Outbox Dispatcher Worker

**Goal:** Translate DB events → Pub/Sub messages safely.

Most services, repos, etc live and new ones will also live here `pkg/outbox/**/*`

* [ ] **Ticket:** Finalize `cmd/outbox-publisher` binary
* [ ] **Ticket:** Event → topic routing registry switch case per event_type & Typed payload validation per event_type
* [ ] **Ticket:** Retry + max-attempt policy defined
* [ ] **Ticket:** DLQ model + migration + repo defined
* [ ] **Ticket:** DLQ publish on terminal failure

---

### 15C) Media Processing Worker (GCS-triggered)

**Goal:** Heavy processing off request path.

* [ ] **Ticket:** Create `cmd/media-worker` binary
* [ ] **Ticket:** Subscribe to GCS finalize events (env-driven)
* [ ] **Ticket:** Image/video compression helper
* [ ] **Ticket:** OCR provider abstraction (OpenAI vs Document AI)
* [ ] **Ticket:** OCR text generation + storage (`ocr.txt`)
* [ ] **Ticket:** Update media row with derived assets
* [ ] **Ticket:** Emit `media_processed` outbox event

---

### 15D) Media Deletion Worker  (GCS-triggered || outbox event from DELETE /media/{mediaID})

**Goal:** Safe cascading deletes after API validation.

* [X] **Ticket:** Create `cmd/media-delete-worker` binary
* [X] **Ticket:** Consume `media_deleted` events
* [ ] **Ticket:** Resolve and delete all attachment references -> currently no opp need the actual detatch per entity 
* [ ] **Ticket:** Delete GCS originals + derived artifacts

---

## Phase 16 — Analytics Engine (Dedicated Consumer)

---

### 16A) Analytics Worker

* [ ] **Ticket:** Create `cmd/analytics-worker` binary
* [ ] **Ticket:** Finalize BigQuery schemas (marketplace_events + ad_events)
* [ ] **Ticket:** Order event ingestion (geo, category, product, )
* [ ] **Ticket:** ZIP/geo derivation strategy (documented)
* [ ] **Ticket:** Ad impression/click ingestion 
* [ ] **Ticket:** Aggregated KPI derivation (AOV, ROAS, top ZIPs)
* [ ] **Ticket:** Admin analytics endpoint (non-MVP)

---

## Phase 17 — Notifications (Email Pipeline)

* [ ] **Ticket:** Notification template definitions
* [ ] **Ticket:** Email sender interface
* [ ] **Ticket:** Stub email sender (log-only)
* [ ] **Ticket:** SendGrid adapter (future swap)

---

## Phase 18 — COA → OpenAI Product Drafts

* [ ] **Ticket:** OpenAI client bootstrap
* [ ] **Ticket:** COA OCR → structured parser
* [ ] **Ticket:** Product draft JSON generator
* [ ] **Ticket:** Persist draft + status

---

## Phase 19 — Ads Engine (Serve + Track + Token Attribution + Rollup + Bill Bridge)

**Goal:** Ship a production-viable CPM ad engine with request-time serving, Redis counters + dedupe guards, signed client-side attribution tokens frozen at checkout (order + line-item attribution), daily Postgres rollups, and billing bridge via `usage_charges` + Pub/Sub fanout.

### 19A) Core constants, models, schema

* [ ] Ticket: Define ad engine constants + Redis key schema + TTL conventions (imps/clicks/spend + dedupe keys)
* [ ] Ticket: Add/confirm enums: ad status, placement, billing model (CPM), token event type (impression|click), token target type (store|product)
* [ ] Ticket: Add Postgres migration: `cart_records.attribution_tokens` JSONB (bounded array) + indexes as needed (store_id, updated_at)
* [ ] Ticket: Add Postgres migration: `vendor_orders.attribution` JSONB (order-level) + indexable columns if desired (attributed_ad_id, attribution_type)
* [ ] Ticket: Add Postgres migration: `vendor_order_line_items.attribution` JSONB nullable (line-item level) + indexable columns if desired (attributed_ad_id)
* [ ] Ticket: Postgres schema: `ad_daily_rollups` table (per ad per day: imps, clicks, spend_cents) + unique(ad_id, day) + indexes
* [ ] Ticket: Postgres schema: ensure `usage_charges` uniqueness supports idempotent daily ad spend charges (store_id + type + for_date)

### 19B) Attribution token system (server-signed, client-carried)

* [ ] Ticket: Define token schema (“attribution receipt”): fields + versioning + size constraints (ad_id, creative_id, placement, target_type, target_id, buyer_store_id, occurred_at, expires_at, event_type, request_id/nonce)
* [ ] Ticket: Implement token signing + verification utility (HMAC/JWT HS256) in shared pkg (strict validation + clock skew rules)
* [ ] Ticket: Add server-side “token validation” helper: verify signature, expiry, buyer_store match, enum sanity, and dedupe rules (token_id/request_id)
* [ ] Ticket: Define token precedence rules (deterministic): click > impression; for order-level store attribution only target_type=store; for line-item attribution only target_type=product; most-recent wins; stable tie-break hash

### 19C) Ad serving & tracking APIs (serve, impression, click)

* [ ] Ticket: Repo layer: fetch eligible candidate ads for `/ads/serve` (status=active, placement match, time window, joins for store gating)
* [ ] Ticket: Service: implement eligibility filter pipeline (subscription_active + kyc verified + status/time window + geo hook)
* [ ] Ticket: Redis helper: budget gate read (imps/clicks/spend today) + “exhausted” evaluation vs daily_budget_cents
* [ ] Ticket: Serving algorithm: highest bid wins selector + deterministic tie-break (hash(request_id + placement + day + ad_id))
* [ ] Ticket: DTOs: `ServeAdRequest/Response` includes creative payload + **signed impression token** + **signed click token** + request_id
* [ ] Ticket: Controller + route: `GET /ads/serve` returns winning creative + tokens (hero placement still targets store or product)
* [ ] Ticket: Redis dedupe helper: impression dedupe via `SETNX` keyed by (request_id + placement + ad_id) with TTL
* [ ] Ticket: Controller + route: `POST /ads/impression` verifies token + increments Redis imps + spend (CPM bid_cents/1000) with dedupe guard
* [ ] Ticket: Redis dedupe helper: click dedupe via `SETNX` keyed by (request_id + ad_id) with TTL
* [ ] Ticket: Controller + route: `GET /ads/click` verifies token + increments Redis clicks (no Postgres click row) + 302 redirect to destination URL

### 19D) Cart persistence of tokens (client → server)

* [ ] Ticket: Update cart DTOs to accept `attribution_tokens[]` from client (bounded list)
* [ ] Ticket: Cart service: normalize tokens on cart save (validate signature/expiry/store match; dedupe; cap to max N; keep most recent per (ad_id,event_type,target_id))
* [ ] Ticket: Cart repo: persist normalized `cart_records.attribution_tokens` and return the normalized set to client (so client converges)
* [ ] Ticket: Add guardrails: reject unreasonably large payloads; structured logs for invalid tokens; do not hard-fail checkout if tokens invalid (drop tokens)

### 19E) Checkout-time attribution materialization (immediate during checkout validation)

* [ ] Ticket: Checkout validation: load `cart_records.attribution_tokens` and compute per-vendor-order candidate sets (by vendor_store_id + product_ids)
* [ ] Ticket: Compute **order-level attribution** (store-only): choose best token where target_type=store AND target_id==vendor_store_id using click>impression + recency + tie-break; set `vendor_orders.attribution`
* [ ] Ticket: Compute **line-item attribution** (product-only): for each line item choose best token where target_type=product AND target_id==product_id using click>impression + recency + tie-break; set `vendor_order_line_items.attribution` (or null)
* [ ] Ticket: Ensure deterministic attribution reasons are stored (e.g., `reason=store_click_match`, `reason=product_impression_match`, `reason=none`) for analytics/debuggability
* [ ] Ticket: Wire attribution persistence into the same transaction that creates checkout_group + vendor_orders + line_items (no partial writes)

### 19F) Daily rollup + billing bridge + outbox fanout

* [ ] Ticket: Scheduler job: daily ad rollup reads Redis counters for day N and writes `ad_daily_rollups` + `usage_charges` (idempotent)
* [ ] Ticket: Rollup rounding policy: deterministic conversion of spend float → cents (document + implement helper; avoid drift)
* [ ] Ticket: Outbox emission: write `OutboxEvent` for `ad_spend_rolled_up` after successful rollup transaction
* [ ] Ticket: Pub/Sub topic wiring: publish rollup outbox events and add consumer skeletons for analytics + billing
* [ ] Ticket: Billing consumer: bridge daily `usage_charges` into `charges` / Stripe-metered usage interface (stub if not shipping Stripe yet)

### 19G) Analytics propagation (BigQuery)

* [ ] Ticket: Define analytics payload contract for ads: include rollups + checkout attribution snapshot (order + line-item attributions)
* [ ] Ticket: Analytics consumer: on order/checkout events, extract vendor_order.attribution + line_item.attribution and insert rows into BigQuery ad attribution table(s)
* [ ] Ticket: Analytics consumer: on rollup events, insert/update BigQuery ad daily rollup table(s) (partitioning + clustering consistent with your BQ design)
* [ ] Ticket: Ensure “EVERY ad associated with the order” is emitted: one row per attributed ad per line item (product) plus optional order-level store row (store)

### 19H) Failure modes, observability, tests

* [ ] Ticket: Failure mode behavior: Redis unavailable => serve no ads; impression/click endpoints fail closed; structured logs
* [ ] Ticket: Observability: metrics/logs for serve decisions (candidate counts, exclusion reasons, dedupe hits, winner, budget-gated)
* [ ] Ticket: Load-shedding: per-request candidate limit + optional short-lived caching of candidate IDs per placement/state (separate small ticket)
* [ ] Ticket: Integration tests: serve→impression→click counters→cart token persist→checkout attribution (order + line-item)→rollup→usage_charges (deterministic tie-break coverage)


---

## Phase 20 — Ops, Observability, Hardening

* [ ] Worker metrics + DLQ visibility
* [ ] Replay & recovery runbooks
* [ ] Feature flags
* [ ] Backup/restore drills

---

## Phase 21 — Deferred / Explicitly Parked

* ACH
* MFA / TOTP
* Seed-to-sale
* Address validation
* Blockchain / NFTs

---

### Final note (important)

At this point, **the biggest risk is no longer missing tickets** — it’s *ticket blast radius*.
This rewrite ensures:

* each ticket ≈ **one concern**
* each worker binary evolves independently
* an LLM cannot “helpfully” refactor half the repo in one go

If you want next:

* I can **annotate which Go packages each ticket should touch**, or
* Convert **Phase 13 or Phase 15** directly into **LLM-safe Jira tickets**.

You’re now designing *systems that survive assistants*.
