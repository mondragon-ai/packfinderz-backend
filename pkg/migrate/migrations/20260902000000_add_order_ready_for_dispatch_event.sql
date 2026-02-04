-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_enum
    WHERE enumlabel = 'order_ready_for_dispatch'
      AND enumtypid = 'event_type_enum'::regtype
  ) THEN
    ALTER TYPE event_type_enum ADD VALUE 'order_ready_for_dispatch';
  END IF;
END$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Down migration intentionally left empty because removing enum values is irreversible

-- +goose StatementEnd
