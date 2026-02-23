-- +goose Up
-- Convert cart_records.tip from INT to NUMERIC(10,2) (if needed)

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'cart_records'
      AND column_name  = 'tip'
  ) THEN
    ALTER TABLE public.cart_records
      ALTER COLUMN tip TYPE NUMERIC(10,2)
      USING tip::numeric(10,2);
  END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- Revert cart_records.tip back to INT (rounded)

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'cart_records'
      AND column_name  = 'tip'
  ) THEN
    ALTER TABLE public.cart_records
      ALTER COLUMN tip TYPE INT
      USING ROUND(tip)::int;
  END IF;
END $$;