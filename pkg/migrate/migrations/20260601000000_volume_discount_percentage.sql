-- +goose Up
ALTER TABLE product_volume_discounts
  ADD COLUMN discount_percent numeric(7,4) NOT NULL DEFAULT 0;

UPDATE product_volume_discounts v
SET discount_percent = CASE
  WHEN p.price_cents <= 0 THEN 0
  ELSE ROUND((GREATEST(p.price_cents - v.unit_price_cents, 0)::numeric / NULLIF(p.price_cents, 0)) * 100, 4)
END
FROM products p
WHERE p.id = v.product_id;

ALTER TABLE product_volume_discounts
  DROP COLUMN unit_price_cents;

ALTER TABLE product_volume_discounts
  ADD CONSTRAINT chk_product_volume_discount_percent
  CHECK (discount_percent >= 0 AND discount_percent <= 100);

-- +goose Down
ALTER TABLE product_volume_discounts
  ADD COLUMN unit_price_cents integer NOT NULL DEFAULT 0;

UPDATE product_volume_discounts v
SET unit_price_cents = GREATEST(
  p.price_cents - CAST(ROUND(p.price_cents * v.discount_percent / 100, 0) AS integer),
  0
)
FROM products p
WHERE p.id = v.product_id;

ALTER TABLE product_volume_discounts
  DROP CONSTRAINT IF EXISTS chk_product_volume_discount_percent;

ALTER TABLE product_volume_discounts
  DROP COLUMN discount_percent;
