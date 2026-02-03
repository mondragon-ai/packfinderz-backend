-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_enum
    WHERE enumlabel = 'failed'
      AND enumtypid = 'payment_status'::regtype
  ) THEN
    ALTER TYPE payment_status ADD VALUE 'failed';
  END IF;
  IF NOT EXISTS (
    SELECT 1
    FROM pg_enum
    WHERE enumlabel = 'rejected'
      AND enumtypid = 'payment_status'::regtype
  ) THEN
    ALTER TYPE payment_status ADD VALUE 'rejected';
  END IF;
END$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Down migration intentionally left empty because removing enum values is irreversible

-- +goose StatementEnd
