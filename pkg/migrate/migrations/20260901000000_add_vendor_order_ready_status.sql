-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_enum
    WHERE enumlabel = 'ready_for_dispatch'
      AND enumtypid = 'vendor_order_status'::regtype
  ) THEN
    ALTER TYPE vendor_order_status ADD VALUE 'ready_for_dispatch';
  END IF;
END$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Down migration intentionally left empty because removing enum values is irreversible

-- +goose StatementEnd
