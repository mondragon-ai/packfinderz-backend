# Outbox Publisher

## Purpose

The outbox pattern keeps domain truth in Postgres while asynchronously publishing the same intent to Pub/Sub row-by-row. Business code writes both the domain record and the matching `outbox_events` row in a single transaction, so **Postgres remains the primary source of truth**, and every Outbox row is a durable promise to publish that envelope later.

This document explains how `cmd/outbox-publisher` claims rows, publishes them, and reports outcomes so the system is repeatable, observable, and safe to operate.

## Transactional flow

1. Domain services build an `outbox.DomainEvent` and call `outbox.Service.Emit(tx, event)` inside the same transaction that mutates business tables.
2. The event is stored with an envelope (`event_id`, `version`, `occurred_at`, `actor`, `Data`) inside the `payload` column; timestamps like `created_at`, `attempt_count`, `last_error`, and `published_at` live alongside it.
3. The publisher worker reads `outbox_events` rows where `published_at IS NULL`, publishes to Pub/Sub, and marks success or increments failures before committing, keeping the transactional boundary as tight as possible.

## Worker behavior

`cmd/outbox-publisher` is a standalone binary that:

* Boots config, structured logging, the shared GORM DB client, and the Pub/Sub client (the same readiness checks as API/worker: `db.Ping` + `pubsub.Ping`).
* Claims batches with `FOR UPDATE SKIP LOCKED` ordered by `created_at ASC` so multiple publisher instances can safely run simultaneously without stepping on each other.
* Publishes the `payload` JSON (`PayloadEnvelope`) as the Pub/Sub message body and attaches attributes for `event_id`, `event_type`, `aggregate_type`, `aggregate_id`, and `created_at`. The topic defaults to `PF_DOMAIN_TOPIC` (`PACKFINDERZ_PUBSUB_DOMAIN_TOPIC`) with the `event_type` attribute so downstream filtering can stay light.
* On success, updates `published_at` so the row is no longer considered pending.
* On failure, increments `attempt_count`, sets a bounded-length `last_error`, and leaves `published_at` null so retries can continue until max attempts is reached; the query filters out rows with attempt count ≥ the configured limit, avoiding hot loops on poisoned rows.
* Sleeps between loops using `PACKFINDERZ_OUTBOX_PUBLISH_POLL_MS` (defaults to 500 ms) plus ~250 ms of jitter. Pub/Sub errors trigger exponential backoff doubling the sleep time up to 10 s before retrying.
* Logs each attempt with `event_id`, `event_type`, `aggregate_type`, `aggregate_id`, and `batch_size` for observability.

## Configuration knobs

| Env var | Default | Description |
| --- | --- | --- |
| `PACKFINDERZ_OUTBOX_PUBLISH_BATCH_SIZE` | `50` | How many rows to claim per transaction/poll.
| `PACKFINDERZ_OUTBOX_PUBLISH_POLL_MS` | `500` | Base sleep when no rows were claimed or after a healthy batch.
| `PACKFINDERZ_OUTBOX_MAX_ATTEMPTS` | `25` | Rows with `attempt_count ≥ max` are skipped and left for manual inspection.
| `PACKFINDERZ_PUBSUB_DOMAIN_TOPIC` | `pf-domain-events` | The Pub/Sub topic the worker publishes to; attribute `event_type` enables filtering.
| `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` | `720h` | Redis TTL for `pf:evt:processed:<consumer>:<event_id>` keys; shared with consumers for idempotency (same env already documented for the manager). |

## Deployment & local runs

* `make run-outbox-publisher` boots the worker with `PACKFINDERZ_SERVICE_KIND=outbox-publisher` (after loading `.env` if present).
* `make build-outbox-publisher` compiles `./cmd/outbox-publisher` into `./bin/outbox-publisher` for Heroku/container images (`Dockerfile` now builds and copies `/bin/outbox-publisher`, and `chmod`+ ensures the binary is executable).
* `heroku.yml` now declares a third dyno: `outbox-publisher: sh -lc 'PACKFINDERZ_SERVICE_KIND=outbox-publisher /bin/outbox-publisher'`, letting us run the publisher alongside `api` and `worker` in production.

## Messaging semantics

The worker publishes the raw envelope to Pub/Sub and tags messages with attributes for quick filtering. Consumers should:

1. Parse the envelope, log the `event_id`, `event_type`, `aggregate_type`, `aggregate_id`, and `created_at` fields.
2. Call `pkg/eventing/idempotency.Manager.CheckAndMarkProcessed(ctx, consumerName, eventID)` before any side effect. That manager checks/sets the Redis key `pf:evt:processed:<consumer>:<event_id>` and respects `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` to keep duplicate deliveries (at-least-once) from running the same logic twice.
3. Act idempotently after the manager returns `false`; duplicate deliveries are expected and acceptable, which is why logs surface the `event_id` for troubleshooting.

## Failure, retries, & max-attempt handling

* Every failure increments `attempt_count` and captures a truncated `last_error`. No row is marked as published until Pub/Sub returns success.
* The publisher stops claiming rows once `attempt_count ≥ PACKFINDERZ_OUTBOX_MAX_ATTEMPTS`, so poisoned data requires manual investigation.
* Backoff on errors: each Pub/Sub failure doubles the sleep time (caps at 10 s) and adds jitter (~0–250 ms). Healthy loops use the base poll interval + jitter.

## Why `FOR UPDATE SKIP LOCKED`?

Locking keeps the claim in the originating transaction: rows are locked until the publish attempt either sets `published_at` (commit) or aborts (rollback). This ensures:

* Multiple publisher instances can run concurrently without publishing the same row twice.
* If the process crashes mid-publish, the transaction rolls back, the lock is released, and another publisher can pick the row up.

## Why duplicates are acceptable?

The system targets **at-least-once delivery**. A transient failure (e.g., the publisher crashes after Pub/Sub ack but before updating `published_at`) can cause another worker to see the row again. Consumers rely on `pkg/eventing/idempotency.Manager` and the Redis key pattern to detect duplicates, so they can safely ignore repeats.

## Why Redis idempotency?

Redis keys `pf:evt:processed:<consumer>:<event_id>` record the fact that a specific consumer handled `event_id`. The TTL (`PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL`, default `720h`) keeps the lock long enough to survive retries while also expiring so events can eventually be garbage collected. Without Redis dedup, we would risk double side effects (emails, billing, notifications) whenever the publisher retries or the consumer gets the same Pub/Sub message twice.

## Crash scenarios & safety

* If the worker crashes while publishing, the in-flight transaction rolls back because we never updated `published_at`/`attempt_count`. The next loop reclaims the row thanks to `FOR UPDATE SKIP LOCKED` releasing the lock.
* If Pub/Sub acknowledgment succeeds but the worker crashes before updating `published_at`, the row is still pending and will be retried. Consumers must still treat duplicates gracefully.
* If a row hits `attempt_count ≥ max`, the worker stops reclaiming it. These rows stay in Postgres for manual inspection, so alerting should monitor the backlog.

This setup keeps the pipeline retriable, observable, and safe as long as downstream consumers honor the idempotency contract.
