-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------------------
-- Enums (idempotent)
-- -------------------------------------------------------------------

DO $$
BEGIN
  -- Ensure cart_status has required values. If cart_status already exists with other values,
  -- you may need a separate migration to alter it safely.
  CREATE TYPE cart_status AS ENUM ('active','converted');
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE cart_item_status AS ENUM (
    'ok',
    'not_available',
    'invalid'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE cart_item_warning_type AS ENUM (
    'clamped_to_moq',
    'clamped_to_max',
    'price_changed',
    'not_available',
    'vendor_invalid',
    'vendor_mismatch',
    'invalid_promo'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE vendor_group_status AS ENUM (
    'ok',
    'invalid'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE vendor_group_warning_type AS ENUM (
    'vendor_invalid',
    'vendor_suspended',
    'license_invalid',
    'invalid_promo'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

-- -------------------------------------------------------------------
-- cart_records changes
-- -------------------------------------------------------------------

-- Drop legacy index (session_id will be removed)
DROP INDEX IF EXISTS idx_cart_records_session;

-- Add new fields (safe defaults for existing rows)
ALTER TABLE cart_records
  ADD COLUMN IF NOT EXISTS checkout_group_id uuid NULL,
  ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'USD',
  ADD COLUMN IF NOT EXISTS valid_until timestamptz NOT NULL DEFAULT (now() + interval '15 minutes'),
  ADD COLUMN IF NOT EXISTS discounts_cents int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS ad_tokens text[] NULL;

-- Drop FK that references checkout_groups if it exists (we are removing checkout_groups)
ALTER TABLE cart_records DROP CONSTRAINT IF EXISTS cart_records_checkout_group_fk;

-- Remove deprecated columns
ALTER TABLE cart_records
  DROP COLUMN IF EXISTS session_id,
  DROP COLUMN IF EXISTS fees,
  DROP COLUMN IF EXISTS total_discount,
  DROP COLUMN IF EXISTS cart_level_discount;

-- Helpful index for conversion correlation
CREATE INDEX IF NOT EXISTS idx_cart_records_checkout_group ON cart_records (checkout_group_id);

-- -------------------------------------------------------------------
-- cart_items changes
-- -------------------------------------------------------------------

-- Rename for canonical persistence shape
ALTER TABLE cart_items RENAME COLUMN qty TO quantity;
ALTER TABLE cart_items RENAME COLUMN sub_total_price TO line_subtotal_cents;

-- Add quote artifacts
ALTER TABLE cart_items
  ADD COLUMN IF NOT EXISTS max_qty int NULL,
  ADD COLUMN IF NOT EXISTS applied_volume_discount jsonb NULL,
  ADD COLUMN IF NOT EXISTS status cart_item_status NOT NULL DEFAULT 'ok',
  ADD COLUMN IF NOT EXISTS warnings jsonb NULL;

-- Drop UI snapshot fields that are not part of authoritative quote persistence
ALTER TABLE cart_items
  DROP COLUMN IF EXISTS product_sku,
  DROP COLUMN IF EXISTS unit,
  DROP COLUMN IF EXISTS compare_at_unit_price_cents,
  DROP COLUMN IF EXISTS discounted_price,
  DROP COLUMN IF EXISTS featured_image,
  DROP COLUMN IF EXISTS thc_percent,
  DROP COLUMN IF EXISTS cbd_percent;

-- Enforce MOQ not null safely
UPDATE cart_items SET moq = 1 WHERE moq IS NULL;
ALTER TABLE cart_items ALTER COLUMN moq SET DEFAULT 1;
ALTER TABLE cart_items ALTER COLUMN moq SET NOT NULL;

-- Enforce line_subtotal_cents not null safely
-- Prefer: quantity * unit_price_cents when null (more correct than 0)
UPDATE cart_items
SET line_subtotal_cents = (quantity * unit_price_cents)
WHERE line_subtotal_cents IS NULL;

ALTER TABLE cart_items ALTER COLUMN line_subtotal_cents SET DEFAULT 0;
ALTER TABLE cart_items ALTER COLUMN line_subtotal_cents SET NOT NULL;

-- Add missing index
CREATE INDEX IF NOT EXISTS idx_cart_items_product_id ON cart_items (product_id);

-- -------------------------------------------------------------------
-- cart_vendor_groups (new)
-- -------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS cart_vendor_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cart_id uuid NOT NULL REFERENCES cart_records(id) ON DELETE CASCADE,
  vendor_store_id uuid NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
  status vendor_group_status NOT NULL DEFAULT 'ok',
  warnings jsonb NULL,
  subtotal_cents int NOT NULL DEFAULT 0,
  promo jsonb NULL,
  total_cents int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cart_vendor_groups_cart_vendor_uniq UNIQUE (cart_id, vendor_store_id)
);

CREATE INDEX IF NOT EXISTS idx_cart_vendor_groups_cart_id ON cart_vendor_groups (cart_id);
CREATE INDEX IF NOT EXISTS idx_cart_vendor_groups_vendor_store_id ON cart_vendor_groups (vendor_store_id);

-- -------------------------------------------------------------------
-- Remove checkout_groups table entirely
-- -------------------------------------------------------------------

-- Drop the table; CASCADE ensures any dependent constraints are removed.
-- If you want stricter safety, explicitly drop known FKs first.
DROP TABLE IF EXISTS checkout_groups CASCADE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- NOTE: Down migrations here restore the old cart schema + checkout_groups table shell.
-- If you had additional columns/constraints on checkout_groups previously, re-create them explicitly.

-- Re-create checkout_groups table (minimal) so old FKs can be restored if needed
CREATE TABLE IF NOT EXISTS checkout_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cart_id uuid NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Restore cart_records legacy columns
ALTER TABLE cart_records
  ADD COLUMN IF NOT EXISTS session_id text NULL,
  ADD COLUMN IF NOT EXISTS fees int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS total_discount int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cart_level_discount cart_level_discount[] NULL;

-- Remove new cart_records columns
DROP INDEX IF EXISTS idx_cart_records_checkout_group;

ALTER TABLE cart_records
  DROP COLUMN IF EXISTS checkout_group_id,
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS valid_until,
  DROP COLUMN IF EXISTS discounts_cents,
  DROP COLUMN IF EXISTS ad_tokens;

-- Recreate session index
CREATE INDEX IF NOT EXISTS idx_cart_records_session ON cart_records (session_id);

-- cart_items: add back removed columns (minimal types; align with your original schema)
ALTER TABLE cart_items
  ADD COLUMN IF NOT EXISTS product_sku text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS unit unit NULL,
  ADD COLUMN IF NOT EXISTS compare_at_unit_price_cents int NULL,
  ADD COLUMN IF NOT EXISTS discounted_price int NULL,
  ADD COLUMN IF NOT EXISTS featured_image text NULL,
  ADD COLUMN IF NOT EXISTS thc_percent numeric(5,2) NULL,
  ADD COLUMN IF NOT EXISTS cbd_percent numeric(5,2) NULL;

-- Drop new cart_items columns
DROP INDEX IF EXISTS idx_cart_items_product_id;

ALTER TABLE cart_items
  DROP COLUMN IF EXISTS max_qty,
  DROP COLUMN IF EXISTS applied_volume_discount,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS warnings;

-- Relax constraints (best-effort)
ALTER TABLE cart_items ALTER COLUMN moq DROP NOT NULL;
ALTER TABLE cart_items ALTER COLUMN moq DROP DEFAULT;

ALTER TABLE cart_items ALTER COLUMN line_subtotal_cents DROP NOT NULL;
ALTER TABLE cart_items ALTER COLUMN line_subtotal_cents DROP DEFAULT;

-- Rename back
ALTER TABLE cart_items RENAME COLUMN line_subtotal_cents TO sub_total_price;
ALTER TABLE cart_items RENAME COLUMN quantity TO qty;

-- Drop vendor groups table
DROP TABLE IF EXISTS cart_vendor_groups;
DROP INDEX IF EXISTS idx_cart_vendor_groups_cart_id;
DROP INDEX IF EXISTS idx_cart_vendor_groups_vendor_store_id;

-- Drop enums (warning: only safe if no other columns depend on them)
DROP TYPE IF EXISTS vendor_group_warning_type;
DROP TYPE IF EXISTS vendor_group_status;
DROP TYPE IF EXISTS cart_item_warning_type;
DROP TYPE IF EXISTS cart_item_status;

-- We do NOT drop cart_status here because it may be used elsewhere already.

-- +goose StatementEnd
