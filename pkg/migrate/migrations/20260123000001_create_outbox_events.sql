-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'event_type_enum') THEN
    CREATE TYPE event_type_enum AS ENUM (
      'order_created',
      'order_state_changed',
      'line_item_state_changed',
      'license_status_changed',
      'media_uploaded',
      'payment_settled',
      'cash_collected',
      'vendor_payout_recorded',
      'notification_requested',
      'order_expired',
      'order_canceled',
      'reservation_released',
      'ad_created',
      'ad_updated',
      'ad_paused',
      'ad_activated',
      'ad_expired',
      'ad_daily_rollup_ready'
    );
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'aggregate_type_enum') THEN
    CREATE TYPE aggregate_type_enum AS ENUM (
      'vendor_order',
      'checkout_group',
      'license',
      'store',
      'media',
      'ledger_event',
      'notification',
      'ad'
    );
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS outbox_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type event_type_enum NOT NULL,
  aggregate_type aggregate_type_enum NOT NULL,
  aggregate_id uuid NOT NULL,
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz NULL,
  attempt_count integer NOT NULL DEFAULT 0,
  last_error text NULL
);

CREATE INDEX IF NOT EXISTS outbox_events_published_idx ON outbox_events (published_at);
CREATE INDEX IF NOT EXISTS outbox_events_event_type_idx ON outbox_events (event_type);
CREATE INDEX IF NOT EXISTS outbox_events_aggregate_idx ON outbox_events (aggregate_type, aggregate_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS outbox_events;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'aggregate_type_enum') THEN
    DROP TYPE aggregate_type_enum;
  END IF;
END$$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'event_type_enum') THEN
    DROP TYPE event_type_enum;
  END IF;
END$$;

-- +goose StatementEnd
