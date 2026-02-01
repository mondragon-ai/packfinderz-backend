-- +goose Up
ALTER TABLE cart_records
  ADD COLUMN IF NOT EXISTS payment_method payment_method;
ALTER TABLE cart_records
  ADD COLUMN IF NOT EXISTS shipping_line jsonb;
ALTER TABLE cart_records
  ADD COLUMN IF NOT EXISTS converted_at timestamptz;

-- +goose Down
ALTER TABLE cart_records DROP COLUMN IF EXISTS shipping_line;
ALTER TABLE cart_records DROP COLUMN IF EXISTS payment_method;
ALTER TABLE cart_records DROP COLUMN IF EXISTS converted_at;
