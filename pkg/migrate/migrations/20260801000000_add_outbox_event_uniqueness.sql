-- +goose Up
-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS ix_outbox_events_aggregate
  ON outbox_events (aggregate_type, aggregate_id, created_at);

CREATE INDEX IF NOT EXISTS ix_outbox_events_unpublished
  ON outbox_events (published_at)
  WHERE published_at IS NULL;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_outbox_events_event_aggregate;

-- +goose StatementEnd
