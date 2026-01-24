# ✅ HOW THIS LIST IS DERIVED (IMPORTANT)

**Source of truth used:**

* Your `git log --oneline` (PF-000 → PF-067)
* Your locked Master Context + Architecture docs

**What I did:**

* Grouped commits into **completed phases**
* Removed already-implemented work from future phases
* Re-numbered phases logically (not PF numbers)
* Marked **“DONE” vs “REMAINING”** explicitly
* Avoided inventing scope that isn’t implied

---

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
* [ ] Inventory set endpoint (idempotent)
* [x] MOQ validation (client + server)
* [x] Vendor visibility gating (license + subscription)
* [ ] Product audit logging

---

## **Phase 6 — Cart & Checkout** ❌ NOT STARTED

**Goal:** Multi-vendor checkout with atomic reservation

**Planned tickets:**

* [ ] CartRecord + CartItem models
* [ ] Cart upsert endpoint
* [ ] Cart fetch endpoint
* [ ] Checkout submission endpoint
* [ ] CheckoutGroup creation
* [ ] VendorOrder creation per vendor
* [ ] Atomic inventory reservation logic
* [ ] Partial checkout semantics
* [ ] Checkout idempotency enforcement
* [ ] Emit `order_created` outbox event

---

## **Phase 7 — Orders, Fulfillment, TTL** ❌ NOT STARTED

**Goal:** Vendor decisioning + lifecycle automation

**Planned tickets:**

* [ ] Buyer order list + detail endpoints
* [ ] Vendor order list + detail endpoints
* [ ] Vendor accept/reject order
* [ ] Vendor line-item decision endpoint
* [ ] Vendor fulfill endpoint
* [ ] Buyer cancel (pre-transit)
* [ ] Buyer nudge vendor
* [ ] Buyer retry expired order
* [ ] Reservation TTL scheduler (nudge + expire)
* [ ] Inventory release on expiration
* [ ] Order state transition outbox events

---

## **Phase 8 — Delivery, Agents, Cash Collection** ❌ NOT STARTED

**Goal:** Internal logistics + cash MVP

**Planned tickets:**

* [ ] Agent role + auth gating
* [ ] Dispatch queue endpoint
* [ ] Agent assigned orders endpoint
* [ ] Agent pickup endpoint
* [ ] Agent delivery confirmation
* [ ] Cash collected endpoint
* [ ] LedgerEvent: `cash_collected`
* [ ] PaymentIntent settlement
* [ ] Delivery outbox events

---

## **Phase 9 — Ledger, Payouts, Admin Ops** ❌ NOT STARTED

**Goal:** Financial correctness + auditability

**Planned tickets:**

* [ ] LedgerEvent model (append-only)
* [ ] Admin payout queue endpoint
* [ ] Admin confirm payout endpoint
* [ ] LedgerEvent: `vendor_payout`
* [ ] PaymentIntent → paid
* [ ] Order close logic
* [ ] Financial audit logging
* [ ] Exportable summaries

---

## **Phase 10 — Notifications** ❌ NOT STARTED

**Goal:** User awareness without polling

**Planned tickets:**

* [ ] Notification model + migrations
* [ ] Notification worker
* [ ] List notifications endpoint
* [ ] Mark read / mark all read
* [ ] Notification cleanup scheduler

---

## **Phase 11 — Analytics (Marketplace)** ❌ NOT STARTED

**Goal:** Vendor + admin insight via BigQuery

**Planned tickets:**

* [ ] BigQuery datasets + tables
* [ ] Marketplace event schema
* [ ] Analytics ingestion worker
* [ ] Emit order / cash / payout events
* [ ] Vendor analytics endpoint
* [ ] Admin analytics endpoint
* [ ] Preset aggregations (7d/30d/90d)

---

## **Phase 12 — Ads & Attribution** ❌ NOT STARTED

**Goal:** Monetization via demand capture

**Planned tickets:**

* [ ] Ad model + creatives (media_id required)
* [ ] Ad CRUD endpoints
* [ ] Ad eligibility gating
* [ ] Redis ad counters
* [ ] Impression + click tracking
* [ ] Last-click attribution at checkout
* [ ] Daily spend rollups
* [ ] Ad analytics endpoint

---

## **Phase 13 — Subscriptions & Billing** ❌ NOT STARTED

**Goal:** Vendor monetization enforcement

**Planned tickets:**

* [ ] Stripe subscription integration
* [ ] Subscription create / cancel endpoints
* [ ] Enforce vendor subscription gating
* [ ] Billing history endpoints
* [ ] Usage charge records
* [ ] Billing audit logging

---

## **Phase 14 — Security, Ops, Hardening** ❌ NOT STARTED

**Goal:** Production resilience

**Planned tickets:**

* [ ] Rate limiting (critical endpoints)
* [ ] Metrics (API + workers)
* [ ] Alert thresholds
* [ ] Backup & restore runbook
* [ ] Migration rollback strategy
* [ ] Feature flags
* [ ] MFA adapter hooks
