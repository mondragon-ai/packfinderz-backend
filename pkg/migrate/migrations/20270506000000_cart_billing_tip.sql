-- +goose Up
ALTER TABLE IF EXISTS cart_records
  ADD COLUMN IF NOT EXISTS billing_address address_t,
  ADD COLUMN IF NOT EXISTS tip INTEGER NOT NULL DEFAULT 0;

UPDATE cart_records
SET tip = 0
WHERE tip IS NULL;

-- +goose Down
ALTER TABLE IF EXISTS cart_records
  DROP COLUMN IF EXISTS tip,
  DROP COLUMN IF EXISTS billing_address;
