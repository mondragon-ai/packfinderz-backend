# Outbox Implementation

## Overview
Outbox events provide append-only, transaction-aware messaging for cross-service flows (orders, licenses, media, ads, etc.). This implementation establishes the locked schema, canonical enums, payload envelope, repository/service layer, and decoder registry that every domain will reuse before publishing to Pub/Sub or other systems.

## Schema + Migration
`pkg/migrate/migrations/20260123000001_create_outbox_events.sql` defines:

* Postgres enums:
  * `event_type_enum` (core events like `order_created`, `media_uploaded`, optional `ad_*` values).
  * `aggregate_type_enum` (`vendor_order`, `checkout_group`, `license`, `store`, `media`, `ledger_event`, `notification`, `ad`).
* Table `outbox_events` with columns:
  * `id` (UUID pk, default `gen_random_uuid()`).
  * `event_type`, `aggregate_type`, `aggregate_id`, `payload` JSONB, `created_at`.
  * Mutable fields: `published_at`, `attempt_count`, `last_error`.
* Indexes on `(published_at)`, `(event_type)`, `(aggregate_type, aggregate_id)`.

## GORM Model
`pkg/db/models/outbox_event.go` mirrors the migration (enum-typed fields) and uses `json.RawMessage` for the payload.

## Enums
`pkg/enums/outbox.go` exposes:

* `OutboxEventType` constants and validation/parse helpers.
* `OutboxAggregateType` constants and helpers.

These types are referenced across services when constructing events or filtering queries.

## Payload Envelope
`pkg/outbox/envelope.go` defines:

* `ActorRef` (user/store/role) and
* `PayloadEnvelope` with `Version`, `EventID`, `OccurredAt`, optional `Actor`, and raw `Data`.

Services embed domain payloads into this envelope before persistence.

## Repository
`pkg/outbox/repository.go` allows transactional access:

* `Insert(tx *gorm.DB, event models.OutboxEvent)`: requires an open transaction.
* `FetchUnpublished(limit int)`: reads pending events (null `published_at`), ordered oldest-first.
* `MarkPublished(id uuid.UUID)`: sets `published_at` to `now`.
* `MarkFailed(id uuid.UUID, err error)`: increments `attempt_count` and records `last_error`.

## Service
`pkg/outbox/service.go` exposes `Emit(tx, DomainEvent)`:

1. Marshals domain payload to JSON.
2. Wraps it into a `PayloadEnvelope` (auto-generates `EventID`, fills `OccurredAt` if missing).
3. Persists via the repository inside the provided transaction.

DomainEvent includes `EventType`, `AggregateType/ID`, optional `Actor`, `Data`, `Version`, `OccurredAt`.

## Registry
`pkg/outbox/registry.go` maintains a `(event_type, version)` → decoder mapping:

* `Register` stores decoder functions.
* `Decode` looks up and runs the decoder.
* The registry uses `sync.RWMutex` to allow concurrent reads.

`pkg/outbox/registry_test.go` exercises registration/decoding for `license_status_changed`.

## Usage
1. Domain code uses the shared enums when constructing a `DomainEvent`.
2. Calls `outbox.Service.Emit(tx, event)` within the same DB transaction that changes domain state.
3. Worker/dispatcher reads unpublished events via `Repository.FetchUnpublished`, decodes the payload using the registry, dispatches to Pub/Sub, and marks published/failed via the repository.

## Idempotency

Outbox delivery is at-least-once, so duplicates are expected. Each consumer must call `pkg/eventing/idempotency.Manager.CheckAndMarkProcessed` before running any side effects. The helper checks Redis keys named `pf:evt:processed:<consumer>:<event_id>` and only allows the first handler to proceed; duplicates are short-circuited. Configure the TTL with `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (defaults to `720h`) so the key lives long enough to outlast retries.

```go
func handleOutboxEvent(ctx context.Context, manager *idempotency.Manager, consumerName string, eventID uuid.UUID) error {
    alreadyProcessed, err := manager.CheckAndMarkProcessed(ctx, consumerName, eventID)
    if err != nil {
        return err
    }
    if alreadyProcessed {
        log.Printf("outbox event %s already handled", eventID)
        return nil
    }

    // run side effects (publish to Pub/Sub, trigger notifications, etc.)
    return nil
}
```

Publishing code already logs `event_id`, `event_type`, `aggregate_type`, and `aggregate_id` for observability, which makes troubleshooting idempotent replays easier.

## Documentation
`docs/REUSABLE.md` now mentions the outbox enums, envelope, repository, service, and registry to keep the contracts discoverable by other contributors.

## Next Steps
* Integrate with order/license/media/etc. flows to emit events through this stack.
* Implement worker/publisher that reads `outbox_events` and sends to Pub/Sub using the decoder registry.
