-- +goose Up
-- +goose StatementBegin

-- Create enum type if it doesn't exist (works across PG versions)
DO $$
BEGIN
  CREATE TYPE outbox_dlq_error_reason_enum AS ENUM (
    'max_attempts',
    'non_retryable'
  );
EXCEPTION
  WHEN duplicate_object THEN
    NULL;
END
$$;

CREATE TABLE IF NOT EXISTS outbox_dlq (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid NOT NULL,
  event_type event_type_enum NOT NULL,
  aggregate_type aggregate_type_enum NOT NULL,
  aggregate_id uuid NOT NULL,
  payload_json jsonb NOT NULL,
  error_reason outbox_dlq_error_reason_enum NOT NULL,
  error_message text,
  attempt_count int NOT NULL DEFAULT 0,
  failed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_outbox_dlq_event_id ON outbox_dlq(event_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS outbox_dlq;

-- Drop enum type if it exists
DO $$
BEGIN
  DROP TYPE outbox_dlq_error_reason_enum;
EXCEPTION
  WHEN undefined_object THEN
    NULL;
END
$$;

-- +goose StatementEnd
