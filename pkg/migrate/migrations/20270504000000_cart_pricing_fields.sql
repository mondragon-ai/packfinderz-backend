-- +goose Up
ALTER TABLE IF EXISTS cart_vendor_groups
  ADD COLUMN IF NOT EXISTS line_discounts_cents INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS promo_discount_cents INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS discounts_cents INTEGER NOT NULL DEFAULT 0;

ALTER TABLE IF EXISTS cart_items
  ADD COLUMN IF NOT EXISTS title TEXT DEFAULT '' NULL,
  ADD COLUMN IF NOT EXISTS thumbnail TEXT NULL,
  ADD COLUMN IF NOT EXISTS effective_unit_price_cents INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS line_discounts_cents INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS line_total_cents INTEGER NOT NULL DEFAULT 0;

UPDATE cart_items
SET title = ''
WHERE title IS NULL;

ALTER TABLE IF EXISTS cart_items
  ALTER COLUMN title SET NOT NULL;

-- +goose Down
ALTER TABLE IF EXISTS cart_items
  DROP COLUMN IF EXISTS line_total_cents,
  DROP COLUMN IF EXISTS line_discounts_cents,
  DROP COLUMN IF EXISTS effective_unit_price_cents,
  DROP COLUMN IF EXISTS thumbnail,
  DROP COLUMN IF EXISTS title;

ALTER TABLE IF EXISTS cart_vendor_groups
  DROP COLUMN IF EXISTS discounts_cents,
  DROP COLUMN IF EXISTS promo_discount_cents,
  DROP COLUMN IF EXISTS line_discounts_cents;
