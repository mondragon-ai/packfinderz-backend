-- +goose Up
ALTER TABLE IF EXISTS cart_items
  ADD COLUMN IF NOT EXISTS vendor_store_name TEXT DEFAULT '' NULL,
  ADD COLUMN IF NOT EXISTS unit unit NOT NULL DEFAULT 'unit';

UPDATE cart_items
SET vendor_store_name = ''
WHERE vendor_store_name IS NULL;

ALTER TABLE IF EXISTS cart_items
  ALTER COLUMN vendor_store_name SET NOT NULL;

-- +goose Down
ALTER TABLE IF EXISTS cart_items
  DROP COLUMN IF EXISTS unit,
  DROP COLUMN IF EXISTS vendor_store_name;
