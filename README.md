

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

### Payments & Ledger

* MVP: **cash at delivery**
* `LedgerEvent` is append-only
* Payment lifecycle:
  `unpaid → settled → paid`

### Async & Eventing

* API writes business data + `OutboxEvent` in same transaction
* Worker publishes to Pub/Sub
* Consumers **must be idempotent**

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
* Products, inventory, orders
* Payments, ledger events
* Ads, subscriptions
* Outbox events
* Audit logs

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

### Health

```bash
GET /health
GET /health/ready
```

### API Versioning

* All endpoints under `/api/v1`
* Breaking changes require `/api/v2`

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
```

> **Rule:** Do not add new env vars without documentation.

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

**AGENTS.md** is authoritative for repo-aware edits.

---

## Makefile Reference

### Development

```makefile
make dev        # Run API + worker
make api        # Run API only
make worker     # Run worker only
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
```
