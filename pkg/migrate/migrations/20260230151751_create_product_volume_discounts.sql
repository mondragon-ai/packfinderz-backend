-- +goose Up
-- If you use gen_random_uuid(), ensure pgcrypto exists.
CREATE TABLE IF NOT EXISTS product_volume_discounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  min_qty integer NOT NULL CHECK (min_qty >= 1),
  unit_price_cents integer NOT NULL CHECK (unit_price_cents >= 0),
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Prevent duplicate tiers for the same product.
CREATE UNIQUE INDEX IF NOT EXISTS ux_product_volume_discounts_product_min_qty
  ON product_volume_discounts(product_id, min_qty);

-- Query helper (optional)
CREATE INDEX IF NOT EXISTS ix_product_volume_discounts_product
  ON product_volume_discounts(product_id);

-- +goose Down
DROP TABLE IF EXISTS product_volume_discounts;
