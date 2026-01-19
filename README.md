# PackFinderz Go Layout

## Directory ownership

| Directory | Purpose | Import rules |
| --- | --- | --- |
| `cmd/` | Entrypoints for each binary (API server, worker, etc). | May only import from `api/`, `internal/`, and `pkg/`. Should not contain business logic. |
| `api/` | HTTP wiring: routers, handlers, middleware, validators, responses, and adapters to domain services. | May import `internal/*` services and `pkg/*` helpers. No command-specific logic. |
| `internal/<domain>/` | Domain services, repositories, DTO mappings, and any package that should stay private to this module. | Only imports `pkg/*` or sibling `internal/*` packages; never import from `cmd/` or `api/`. |
| `pkg/` | Shared infrastructure (database, redis, logging, config, bootstrap helpers). | Leaf layer—no imports from `internal/` or `api/`. |

## Module boundaries

* `cmd/<binary>/` should only bootstrap its dependencies and exit quickly, delegating HTTP/shutdown logic to `api/` and domain work to `internal/`.
* `api/` does not reach into `cmd/`, keeping HTTP wiring reusable across binaries. Shared middleware should live under `api/middleware` when a router is present.
* Domain packages under `internal/` can depend on each other but must avoid circular references by keeping public interfaces coarse and layering dependencies carefully.
* `pkg/` exposes only primitives that `internal/`, `api/`, and `cmd/` can compose; it never imports domain-specific code.

## Allowed imports checklist

1. `cmd/` → `api/`, `internal/`, `pkg/`
2. `api/` → `internal/`, `pkg/`
3. `internal/` → `pkg/`, other `internal/` packages
4. `pkg/` → standard library only

## Getting started

1. Add domain packages under `internal/` with `service.go`, `repo.go`, `dto.go`, and `mapper.go` before wiring them in `api/`.
2. Keep shared utilities in `pkg/` so they can be reused across binaries without creating circular dependencies.
3. If you need a new directory pattern, document it here and update this README.

## Canonical Responses

- Use `pkg/errors` to build typed errors (`pkg/errors.New` / `pkg/errors.Wrap`) so metadata (http status, retryable flag, safe public message, optional `details`) routes responses consistently.
- Shared shapes in `pkg/types` (`SuccessEnvelope`, `ErrorEnvelope`, `APIError`) back the JSON contracts; `api/responses.WriteSuccess` and `WriteError` set HTTP headers/status and enforce the envelope.
- Validation/auth/conflict/internal codes map to `400`/`401`/`403`/`409`/`422`/`500` respectively, never leaking internal stack traces. A demo handler at `/demo-error` exercises the canonical flow.

## API Routing & Validation

- The API is driven by `chi` and `api/routes.NewRouter`, which mounts `/health/*`, `/api/public/*`, `/api/private/*`, `/api/admin/*`, and `/api/agent/*` groups with group-specific middleware (auth, store context, role checks, idempotency/rate-limit placeholders). 
- Controllers live under `api/controllers`, validators under `api/validators`, and responses under `api/responses`. Each controller starts by validating body/query inputs via `validators.DecodeJSONBody`/`ParseQueryInt`, sanitizes strings, and then calls the shared responses helpers.
- Invalid input always raises `pkg/errors.CodeValidation`, so the API returns a canonical `{"error":{...}}` payload with field-level details and `400` status instead of panics or ad-hoc parsing.

## Structured Logging

- `pkg/logger` exposes context-aware helpers (`Info`, `Warn`, `Error`) and can attach fields like `request_id`, `user_id`, `store_id`, and `actor_role`.
- API middleware generates `request_id`, sends it back via `X-Request-Id`, and ensures all logs for a request include method, path, and duration.
- Workers reuse the same logger, enrich contexts with job metadata, and can include upstream `request_id` if available.
- Control verbosity with `PACKFINDERZ_LOG_LEVEL` (default `info`) and enable warning stacks via `PACKFINDERZ_LOG_WARN_STACK`.
