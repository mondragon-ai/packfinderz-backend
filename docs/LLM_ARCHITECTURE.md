## API service
- `cmd/api/main` loads config, runs dev migrations (`MaybeRunDev`), boots Postgres/Redis/GCS, session manager, domain/internal services, and exposes `routes.NewRouter` on `http.Server.ListenAndServe` (cmd/api/main.go:1-134; pkg/migrate/autorun.go:12-34).
- `routes.NewRouter` wires `Recoverer`, `RequestID`, `Logging`, `Auth`, `StoreContext`, `Idempotency`, and `RateLimit` middleware, then mounts health, public, `/api` (store/media/licenses), `/api/admin`, and `/api/agent` groups (api/routes/router.go:17-106).
- `POST /api/v1/checkout` uses the `pkg/checkout.ValidateMOQ` helper before reserving inventory/orders; failure to meet a product's `MOQ` results in `pkg/errors.CodeStateConflict` (mapped to HTTP `422`) plus a `violations` array (`product_id`, optional `product_name`, `required_qty`, `requested_qty`) so clients can highlight the offending line items (pkg/checkout/validation.go:11-43).
- Checkout orchestration leans on deterministic helpers in `internal/checkout/helpers` to group `CartItem`s by `vendor_store_id`, recompute per-vendor totals (subtotals/discounts/totals), and validate buyer/vendor eligibility (store type, subscription, state, MOQ) before invoking the persistence/services that convert the cart snapshot into checkout entities (`internal/checkout/helpers/grouping.go` & `internal/checkout/helpers/validation.go`).
- Checkout execution hinges on new order data models (`checkout_groups`, `vendor_orders`, `order_line_items`, `payment_intents`) that store the CartRecord snapshot before reservation logic runs, following the schema defined in Doc 4 and built in PF-077.
- `PUT /api/v1/cart` is protected by `middleware.Idempotency` (24h TTL) and calls `internal/cart.Service.UpsertCart`; the service validates the buyer store (verified buyer), vendor visibility (verified + subscription+state), inventory availability, MOQ + volume tiers, subtotal/total math, and cart-level discounts before atomically writing the `cart_record` + `cart_items` snapshot that checkout later consumes (`api/middleware/idempotency.go:45-208`; `internal/cart/service.go:39-209`).
- Buyer product listings (`GET /api/v1/products` and `GET /api/v1/products/{productId}`) reuse `pkg/visibility.EnsureVendorVisible` so only vendors with `stores.kyc_status=verified`, `subscription_active=true`, and `address.state` matching the requested (and buyer's) state are returned; mismatches raise `pkg/errors.CodeValidation`/HTTP `422` or `pkg/errors.CodeNotFound`/HTTP `404`, keeping hidden vendors out of search/detail endpoints (pkg/visibility/visibility.go:11-46).
- `middleware.Auth` validates bearer JWTs via `pkg/auth.ParseAccessToken`, ensures refresh session exists, and injects `user_id`, `store_id`, and `role` into context for the `/api` group (api/middleware/auth.go:23-80).

## Worker loop
- `cmd/worker/main` mirrors API bootstrapping (config, logger, DB, Redis, Pub/Sub, GCS) then builds a `Service` that runs until cancellation (cmd/worker/main.go:1-74).
- `cmd/worker/service.go` `ensureReadiness` pings DB/Redis/PubSub/GCS, then `Run` spawns `media.Consumer.Run` and `notificationConsumer.Run`, monitors errors, and beats a simple ticker while honoring context cancellation (cmd/worker/service.go:20-110).
- `internal/schedulers/licenses.Service` is started from the worker to run every 24h, warn stores 14d ahead of `expiration_date`, expire licenses when due, mirror store KYC, and emit `license_status_changed` outbox events for both warnings and expirations (`internal/schedulers/licenses/service.go:1-220`).
- `internal/media/consumer.Consumer` listens to `pubsub.MediaSubscription()`, decodes `OBJECT_FINALIZE` JSON payloads, and marks matching media rows uploaded, nacking on transient DB timeouts (internal/media/consumer/consumer.go:30-235).
- `internal/notifications/consumer.Consumer` (wired in `cmd/worker/main.go` with the domain subscription and `pkg/outbox/idempotency.Manager`) watches `license_status_changed` events, deduplicates via Redis, and creates `NotificationTypeCompliance` rows for pending, verified, and rejected statuses so admins/stores get compliance notices (internal/notifications/consumer.go:18-197; cmd/worker/main.go:83-116).

## Outbox publisher
- `cmd/outbox-publisher/main` boots config/logging/DB/PubSub, instantiates `outbox.Repository`, and runs the publisher service until interrupted (cmd/outbox-publisher/main.go:1-72).
- `cmd/outbox-publisher/service.go` `Run` loops: `processBatch` fetches `outbox_events` rows inside `db.WithTx`, publishes via `pubsub.DomainPublisher()`, marks success/failure, and backs off with jitter (`sleep`, `nextBackoff`) when no work or errors occur (cmd/outbox-publisher/service.go:66-235).
- `publishRow` marshals stored `PayloadEnvelope`, attaches metadata attributes, and waits on Pub/Sub publish result before marking the row published (cmd/outbox-publisher/service.go:128-185).

## Outbox pattern
- `pkg/outbox.DomainEvent` + `PayloadEnvelope` capture aggregate/event metadata; `Service.Emit` marshals the payload, assigns `event_id`, and queues an `OutboxEvent` row via `Repository.Insert` (pkg/outbox/service.go:1-98; pkg/outbox/envelope.go:9-21).
- `Repository.FetchUnpublishedForPublish` locks `published_at IS NULL` rows (SKIP LOCKED), `MarkPublishedTx` stamps `published_at`, and `MarkFailedTx` increments `attempt_count` while truncating `last_error` (pkg/outbox/repository.go:20-101).
- `DecoderRegistry` registers custom decoders for consumed events, enabling deterministic payload parsing downstream (pkg/outbox/registry.go:1-32).
- `pkg/outbox/idempotency.Manager` paired with `cfg.Eventing.OutboxIdempotencyTTL` prevents duplicate consumer side effects via `pf:idempotency:evt:processed:<consumer>:<event_id>` keys (pkg/outbox/idempotency/idempotency.go:1-66; pkg/config/config.go:131-181).
- `license_status_changed` events flow through the domain topic so the compliance consumer can branch between admin notifications for pending uploads and store notifications for verified/rejected licenses while honoring the idempotency key tracking (`internal/notifications/consumer.go:71-186`).
- Admin license decisions recompute `stores.kyc_status` inside the same transaction by scanning all licenses and calling `DetermineStoreKYCStatus`, ensuring the mirror flips to `verified`, `rejected`, or `expired` before the outbox event fires (`internal/licenses/service.go:385-425`).

## Session & Idempotency
- `pkg/auth/session.Manager` ensures refresh TTL exceeds access TTL, stores refresh tokens keyed by `AccessSessionKey`, rotates/revokes tokens, and supports middleware `HasSession` checks (pkg/auth/session/manager.go:45-154).
- `middleware.Idempotency` hashes request bodies, requires `Idempotency-Key` for configured routes, replays stored responses on retries, and stores records using `redis.IdempotencyStore.SetNX` (api/middleware/idempotency.go:37-208).
- `middleware.StoreContext` rejects `/api` requests lacking a store in context, keeping responses consistent (api/middleware/store.go:6-16).

## Media ingestion
- `internal/media/service.PresignUpload` validates uploader role/kind/size, persists a `Media` row with status `pending`, and signs a PUT URL with the GCS client before the object hits storage (internal/media/service.go:94-195).
- `ListMedia`/`buildReadURL` apply cursor pagination, filters, and attach signed GET URLs for `uploaded` or `ready` media before returning `ListResult` (internal/media/list.go:15-139).
- `DeleteMedia` checks ownership/status, deletes the GCS object, and marks the row `deleted` after `DeleteObject` succeeds (internal/media/service.go:242-284).
- `internal/media/consumer` picks up GCS `OBJECT_FINALIZE` events via Pub/Sub, finds the row by GCS key, and calls `MarkUploaded` so subsequent reads expose the download URL (internal/media/consumer/consumer.go:30-235).

## Dependencies & tooling
- `pkg/pubsub.NewClient` verifies every configured subscription exists before returning publishers/subscribers used by the worker and publisher (pkg/pubsub/client.go:18-202).
- `pkg/storage/gcs.NewClient` fetches credentials (JSON/service account/metadata), pings the bucket, and exposes `SignedURL`, `SignedReadURL`, and `DeleteObject` used by media/license flows (pkg/storage/gcs/client.go:35-506).
- `pkg/migrate.MaybeRunDev` auto-runs Goose migrations when `PACKFINDERZ_AUTO_MIGRATE` plus dev env are enabled, keeping service schema in sync (pkg/migrate/autorun.go:12-34).
