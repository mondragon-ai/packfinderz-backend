## [PF-000] Initialize Go Monorepo with API and Worker Binaries

## Type

Infra

## Description

Initialize a single Go monorepo with multiple binaries to support the PackFinderz API and background workers. This establishes the foundation for synchronous API logic and async/event-driven processing.

## Scope

* Create Go module
* Add `cmd/api` binary for HTTP API
* Add `cmd/worker` binary for async workers (outbox publisher, schedulers, consumers)
* Shared module versioning and dependency management

## Acceptance Criteria

* [ ] `go mod init` completed with reproducible builds
* [ ] `cmd/api/main.go` builds and starts
* [ ] `cmd/worker/main.go` builds and starts
* [ ] Binaries can be built independently

## Dependencies

* Blocked by: none
* Blocks: all other Phase 0 tickets

## Technical Notes

* Go version pinned (e.g. 1.22.x)
* No business logic yet
* **ASSUMPTION:** single repo, no microservices split

## Out of Scope

* Any API routes
* Any workers beyond placeholders

---

## [PF-001] Define Canonical Project Layout and Module Boundaries

## Type

Task

## Description

Define and document the canonical PackFinderz Go project layout to enforce consistency across domains, binaries, and shared infrastructure.

## Scope

* Define directories:

  * `cmd/`
  * `internal/`
  * `pkg/`
  * `api/`
* Establish ownership rules for shared vs domain-specific code
* Add README explaining structure

## Acceptance Criteria

* [ ] Directory structure committed
* [ ] README documents allowed imports and boundaries
* [ ] No circular dependencies between internal modules

## Dependencies

* Blocked by: PF-000
* Blocks: all feature work

## Technical Notes

* `internal/<domain>` holds services, repos, DTOs
* `pkg/` holds shared infra (db, redis, logging, auth utils)
* `api/` holds HTTP wiring only (handlers, middleware)

## Out of Scope

* Business logic implementation

---

## [PF-002] Environment-Only Typed Config Loader

## Type

Task

## Description

Add a typed configuration system that loads **only from environment variables**, shared across API and worker binaries.

## Scope

* Typed `Config` struct
* Env parsing with validation
* Fail-fast on missing required vars
* Separate config sections (DB, Redis, JWT, GCP, Stripe)

## Acceptance Criteria

* [ ] App fails on startup if required env missing
* [ ] Config is injectable into binaries
* [ ] No `.env` files checked into repo

## Dependencies

* Blocked by: PF-001
* Blocks: DB, Redis, API startup

## Technical Notes

* Use `envconfig` or equivalent
* **ASSUMPTION:** no dynamic config reload in MVP

## Out of Scope

* Secrets management implementation (handled by platform)

---

## [PF-003] Define Environment Variable Manifest (Dev vs Prod)

## Type

Chore

## Description

Create a documented manifest of required environment variables for development and production.

## Scope

* List required env vars
* Indicate dev-only vs prod-only
* Document example values (non-secret)

## Acceptance Criteria

* [ ] `ENV_VARS.md` exists
* [ ] Variables grouped by subsystem
* [ ] Matches config loader exactly

## Dependencies

* Blocked by: PF-002
* Blocks: CI, onboarding contributors

## Technical Notes

* Do not include secrets
* Reference Heroku/GCP conventions

## Out of Scope

* Automated env provisioning

---

## [PF-004] Structured JSON Logger with Request ID Propagation

## Type

Task

## Description

Implement a structured JSON logger used across API and workers, with request/job correlation via `request_id`.

## Scope

* JSON log format
* Logger initialization in binaries
* Request ID middleware for API
* Propagation into workers via payload

## Acceptance Criteria

* [ ] All logs are structured JSON
* [ ] API logs include `request_id`
* [ ] Worker logs include originating `request_id` when available

## Dependencies

* Blocked by: PF-002
* Blocks: observability, debugging

## Technical Notes

* Use `zap` or `zerolog`
* **ASSUMPTION:** request_id generated at edge if missing

## Out of Scope

* Log aggregation setup

---

## [PF-005] Standard API Error & Response Envelope Helpers

## Type

Task

## Description

Create reusable helpers to enforce the canonical API response and error envelope across all handlers.

## Scope

* Success response helpers
* Error response helpers
* Mapping internal errors → HTTP codes
* Consistent JSON schema

## Acceptance Criteria

* [ ] All handlers return standard envelope
* [ ] Error codes match API spec
* [ ] No raw errors leaked to clients

## Dependencies

* Blocked by: PF-001
* Blocks: API handlers

## Technical Notes

* Centralize in `api/respond`
* Error codes as constants/enums

## Out of Scope

* Localization or error translation

---

## [PF-006] HTTP Server Bootstrap with Chi + Middleware Stack

## Type

Task

## Description

Set up the Chi-based HTTP server with a layered middleware stack supporting auth, idempotency, validation, and rate limiting.

## Scope

* Chi router setup
* Global middleware:

  * request_id
  * logging
  * panic recovery
* Route-grouped middleware:

  * JWT auth
  * permissions
  * idempotency
  * rate limiting
* Separation of protected vs unprotected routes

## Acceptance Criteria

* [ ] Auth routes accessible without JWT
* [ ] Protected routes enforce JWT + store context
* [ ] Middleware ordering documented and tested

## Dependencies

* Blocked by: PF-004, PF-005
* Blocks: all API endpoints

## Technical Notes

* Middleware split into files by concern
* **ASSUMPTION:** Chi v5

## Out of Scope

* Business authorization rules

---

## [PF-007] Request Validation Layer (Body + Query)

## Type

Task

## Description

Add a standardized validation layer for request bodies and query params to prevent invalid input and injection vectors.

## Scope

* Struct-based validation
* Query param validation helpers
* Early rejection before handler logic

## Acceptance Criteria

* [ ] Invalid payloads return 400 with details
* [ ] Validators reusable across endpoints
* [ ] No handler manually validates raw input

## Dependencies

* Blocked by: PF-006
* Blocks: endpoint implementation

## Technical Notes

* Use `go-playground/validator`
* Sanitize strings where applicable

## Out of Scope

* Advanced WAF or IDS logic

---

## [PF-008] Health Endpoints for API Binary

## Type

Infra

## Description

Expose liveness and readiness endpoints for the API binary.

## Scope

* `/health/live`
* `/health/ready`
* Readiness checks DB + Redis connectivity

## Acceptance Criteria

* [ ] Liveness returns 200 if process alive
* [ ] Readiness fails if DB unavailable
* [ ] No auth required

## Dependencies

* Blocked by: PF-009, PF-010
* Blocks: deployment

## Technical Notes

* Fast checks only
* **ASSUMPTION:** no deep dependency checks

## Out of Scope

* Kubernetes-specific probes

---

## [PF-009] Postgres Bootstrap (Cloud SQL Compatible)

## Type

Infra

## Description

Add Postgres connection bootstrap compatible with local Docker and Cloud SQL.

## Scope

* Connection pooling
* SSL config
* Health check integration

## Acceptance Criteria

* [ ] API and worker can connect to Postgres
* [ ] Connection config driven by env
* [ ] Graceful shutdown closes pools

## Dependencies

* Blocked by: PF-002
* Blocks: migrations, repos

## Technical Notes

* Use `pgx` under GORM
* **ASSUMPTION:** Cloud SQL Proxy used in dev/prod

## Out of Scope

* Read replicas

---

## [PF-010] Redis Client Bootstrap (Idempotency + Counters)

## Type

Infra

## Description

Initialize Redis client for idempotency keys, rate limits, and counters.

## Scope

* Redis connection bootstrap
* Namespaced key helpers
* Health check integration

## Acceptance Criteria

* [ ] Redis reachable from API and worker
* [ ] Connection reused safely
* [ ] Timeouts configured

## Dependencies

* Blocked by: PF-002
* Blocks: idempotency, ads

## Technical Notes

* Use `go-redis`
* **ASSUMPTION:** single Redis instance

## Out of Scope

* Redis clustering

---

## [PF-011] Docker Compose for Local Dev (Postgres + Redis)

## Type

Infra

## Description

Provide Docker Compose configuration for local development dependencies.

## Scope

* Postgres service
* Redis service
* Volume persistence
* Port mappings

## Acceptance Criteria

* [ ] `docker compose up` starts services
* [ ] Matches prod-compatible versions
* [ ] Devs can run API locally

## Dependencies

* Blocked by: none
* Blocks: local dev

## Technical Notes

* Use official images only

## Out of Scope

* App containers

---

## [PF-012] Goose Migration Runner Binary (cmd/migrate)

## Type

Task

## Description

Add a dedicated migration runner binary to apply DB migrations safely.

## Scope

* `cmd/migrate` binary
* Up/down support
* Env-driven DB config

## Acceptance Criteria

* [ ] Migrations runnable locally and in CI
* [ ] Fails loudly on error
* [ ] Compatible with Cloud SQL

## Dependencies

* Blocked by: PF-009
* Blocks: schema setup

## Technical Notes

* Goose SQL migrations only

## Out of Scope

* Auto-migrate at app startup

---

## [PF-013] Base DB Migrations (pgcrypto, postgis)

## Type

Task

## Description

Add base Postgres extensions required by the system.

## Scope

* Enable `pgcrypto`
* Enable `postgis`

## Acceptance Criteria

* [ ] Extensions enabled via Goose
* [ ] Safe to re-run

## Dependencies

* Blocked by: PF-012
* Blocks: schema migrations

## Technical Notes

* First migration only

## Out of Scope

* Any tables

---

## [PF-014] Add GORM ORM and Base Repo Pattern

## Type

Task

## Description

Integrate GORM and establish a base repository pattern for domain persistence.

## Scope

* GORM setup
* Base repo helpers
* Transaction helpers

## Acceptance Criteria

* [ ] GORM initialized with Postgres
* [ ] Transactions usable across services
* [ ] No models yet required

## Dependencies

* Blocked by: PF-009
* Blocks: domain work

## Technical Notes

* Disable auto-migrate
* **ASSUMPTION:** GORM v2

## Out of Scope

* Model definitions

---

## [PF-015] Makefile + Dev Tooling Targets

## Type

Chore

## Description

Add Makefile targets to standardize local development and CI commands.

## Scope

* build
* test
* lint
* migrate
* run-api
* run-worker

## Acceptance Criteria

* [ ] One-command local startup
* [ ] Targets documented

## Dependencies

* Blocked by: previous infra tickets
* Blocks: CI

## Technical Notes

* Use gcloud + Cloud SQL Proxy where needed

## Out of Scope

* Windows support

---

## [PF-016] CI Pipeline + Secret Scanning

## Type

Infra

## Description

Add CI pipeline to enforce quality and prevent secret leakage.

## Scope

* Lint
* Tests
* Build
* Secret scanning
* PR gating

## Acceptance Criteria

* [ ] CI fails on lint/test failure
* [ ] Secrets cause build failure
* [ ] Runs on PRs

## Dependencies

* Blocked by: PF-015
* Blocks: team scaling

## Technical Notes

* GitHub Actions
* **ASSUMPTION:** use Gitleaks or similar

## Out of Scope

* CD to prod

---

## [PF-017] Heroku Procfile + Release Checklist

## Type

Infra

## Description

Prepare Heroku deployment config and operational checklist.

## Scope

* Procfile (web + worker)
* Release checklist doc

## Acceptance Criteria

* [ ] Heroku can run API + worker
* [ ] Checklist covers migrations, envs

## Dependencies

* Blocked by: infra setup
* Blocks: first deploy

## Technical Notes

* Separate dynos for worker

## Out of Scope

* Staging environment

---

## [PF-018] Internal CLI Scripts for Flow Testing (Dev)

## Type

Task

## Description

Add internal CLI scripts to exercise core flows without UI.

## Scope

* Login
* Checkout
* Order transitions

## Acceptance Criteria

* [ ] Scripts run against dev API
* [ ] Useful for smoke testing

## Dependencies

* Blocked by: API scaffolding
* Blocks: QA velocity

## Technical Notes

* Bash or Go-based CLI
* **ASSUMPTION:** no auth UI yet

## Out of Scope

* End-user tooling

---

## [PF-019] Admin & Agent CLI Utilities (Temporary Ops)

## Type

Task

## Description

Provide CLI utilities for admin and agent actions until UI exists.

## Scope

* Inspect outbox
* Inspect DLQ
* Adjust licenses
* Confirm payouts

## Acceptance Criteria

* [ ] Admin actions possible without UI
* [ ] All actions audited

## Dependencies

* Blocked by: core infra
* Blocks: ops readiness

## Technical Notes

* Guarded by admin credentials
* **ASSUMPTION:** temporary tooling

## Out of Scope

* Long-term admin UX
