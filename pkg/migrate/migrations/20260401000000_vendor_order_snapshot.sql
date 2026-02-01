-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------------------
-- Step 1: rename existing discount column to match cart naming
-- (Postgres does NOT support RENAME COLUMN IF EXISTS)
-- -------------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'vendor_orders'
      AND column_name  = 'discount_cents'
  ) AND NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'vendor_orders'
      AND column_name  = 'discounts_cents'
  ) THEN
    EXECUTE 'ALTER TABLE public.vendor_orders RENAME COLUMN discount_cents TO discounts_cents';
  END IF;
END $$;

-- -------------------------------------------------------------------
-- Step 2: new vendor order columns
-- -------------------------------------------------------------------
ALTER TABLE vendor_orders
  ADD COLUMN IF NOT EXISTS cart_id uuid NULL,
  ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'USD',
  ADD COLUMN IF NOT EXISTS shipping_address address_t NULL,
  ADD COLUMN IF NOT EXISTS warnings jsonb NULL,
  ADD COLUMN IF NOT EXISTS promo jsonb NULL,
  ADD COLUMN IF NOT EXISTS payment_method payment_method NOT NULL DEFAULT 'cash',
  ADD COLUMN IF NOT EXISTS shipping_line jsonb NULL,
  ADD COLUMN IF NOT EXISTS attributed_token jsonb NULL;

-- -------------------------------------------------------------------
-- Step 3: new order line item columns
-- -------------------------------------------------------------------
ALTER TABLE order_line_items
  ADD COLUMN IF NOT EXISTS cart_item_id uuid NULL,
  ADD COLUMN IF NOT EXISTS warnings jsonb NULL,
  ADD COLUMN IF NOT EXISTS applied_volume_discount jsonb NULL,
  ADD COLUMN IF NOT EXISTS moq int NULL,
  ADD COLUMN IF NOT EXISTS max_qty int NULL,
  ADD COLUMN IF NOT EXISTS line_subtotal_cents int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS attributed_token jsonb NULL;

-- -------------------------------------------------------------------
-- Step 4: populate cart_id references
-- -------------------------------------------------------------------
UPDATE vendor_orders vo
SET cart_id = cr.id
FROM cart_records cr
WHERE cr.checkout_group_id IS NOT NULL
  AND cr.checkout_group_id = vo.checkout_group_id
  AND vo.cart_id IS NULL;

DO $$
BEGIN
  IF to_regclass('public.checkout_groups') IS NOT NULL THEN
    UPDATE vendor_orders vo
    SET cart_id = ck.cart_id
    FROM checkout_groups ck
    WHERE vo.checkout_group_id = ck.id
      AND vo.cart_id IS NULL;
  END IF;
END $$;

-- Fail fast if we couldn't backfill cart_id everywhere.
DO $$
DECLARE
  missing int;
BEGIN
  SELECT COUNT(*) INTO missing
  FROM vendor_orders
  WHERE cart_id IS NULL;

  IF missing > 0 THEN
    RAISE EXCEPTION 'vendor_orders.cart_id backfill failed: % rows still NULL', missing;
  END IF;
END $$;

ALTER TABLE vendor_orders
  ALTER COLUMN cart_id SET NOT NULL;

-- -------------------------------------------------------------------
-- Step 5: indexes + constraints
-- -------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_vendor_orders_cart_id ON vendor_orders (cart_id);
CREATE INDEX IF NOT EXISTS idx_vendor_orders_checkout_group_id ON vendor_orders (checkout_group_id);
CREATE INDEX IF NOT EXISTS idx_order_line_items_cart_item_id ON order_line_items (cart_item_id);

-- Add FK only if it doesn't already exist
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'order_line_items_cart_item_fk'
  ) THEN
    EXECUTE 'ALTER TABLE public.order_line_items
             ADD CONSTRAINT order_line_items_cart_item_fk
             FOREIGN KEY (cart_item_id) REFERENCES public.cart_items(id) ON DELETE SET NULL';
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'vendor_orders_cart_fk'
  ) THEN
    EXECUTE 'ALTER TABLE public.vendor_orders
             ADD CONSTRAINT vendor_orders_cart_fk
             FOREIGN KEY (cart_id) REFERENCES public.cart_records(id) ON DELETE CASCADE';
  END IF;
END $$;

ALTER TABLE vendor_orders
  DROP CONSTRAINT IF EXISTS vendor_orders_checkout_fk;

-- -------------------------------------------------------------------
-- Step 6: drop checkout_groups table
-- -------------------------------------------------------------------
DROP TABLE IF EXISTS checkout_groups CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- -------------------------------------------------------------------
-- Step 1: recreate checkout_groups table
-- -------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS checkout_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_store_id uuid NOT NULL,
  cart_id uuid NULL,
  attributed_ad_click_id uuid NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT checkout_groups_buyer_fk FOREIGN KEY (buyer_store_id) REFERENCES stores(id) ON DELETE RESTRICT,
  CONSTRAINT checkout_groups_cart_fk FOREIGN KEY (cart_id) REFERENCES cart_records(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_checkout_groups_buyer_created ON checkout_groups (buyer_store_id, created_at DESC);

-- -------------------------------------------------------------------
-- Step 2: restore vendor order constraints and columns
-- (No "ADD CONSTRAINT IF NOT EXISTS" in Postgres)
-- -------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'vendor_orders_checkout_fk'
  ) THEN
    EXECUTE 'ALTER TABLE public.vendor_orders
             ADD CONSTRAINT vendor_orders_checkout_fk
             FOREIGN KEY (checkout_group_id) REFERENCES public.checkout_groups(id) ON DELETE CASCADE';
  END IF;
END $$;

ALTER TABLE vendor_orders
  DROP CONSTRAINT IF EXISTS vendor_orders_cart_fk;

ALTER TABLE vendor_orders
  DROP COLUMN IF EXISTS cart_id,
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS shipping_address,
  DROP COLUMN IF EXISTS warnings,
  DROP COLUMN IF EXISTS promo,
  DROP COLUMN IF EXISTS payment_method,
  DROP COLUMN IF EXISTS shipping_line,
  DROP COLUMN IF EXISTS attributed_token;

CREATE INDEX IF NOT EXISTS idx_vendor_orders_buyer_created ON vendor_orders (buyer_store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_vendor_orders_vendor_created ON vendor_orders (vendor_store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_vendor_orders_status ON vendor_orders (status);
CREATE UNIQUE INDEX IF NOT EXISTS ux_vendor_orders_group_vendor ON vendor_orders (checkout_group_id, vendor_store_id);

-- Rename discounts_cents back to discount_cents safely
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'vendor_orders'
      AND column_name  = 'discounts_cents'
  ) AND NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'vendor_orders'
      AND column_name  = 'discount_cents'
  ) THEN
    EXECUTE 'ALTER TABLE public.vendor_orders RENAME COLUMN discounts_cents TO discount_cents';
  END IF;
END $$;

-- -------------------------------------------------------------------
-- Step 3: restore order_line_items
-- -------------------------------------------------------------------
ALTER TABLE order_line_items
  DROP CONSTRAINT IF EXISTS order_line_items_cart_item_fk;

ALTER TABLE order_line_items
  DROP COLUMN IF EXISTS cart_item_id,
  DROP COLUMN IF EXISTS warnings,
  DROP COLUMN IF EXISTS applied_volume_discount,
  DROP COLUMN IF EXISTS moq,
  DROP COLUMN IF EXISTS max_qty,
  DROP COLUMN IF EXISTS line_subtotal_cents,
  DROP COLUMN IF EXISTS attributed_token;

DROP INDEX IF EXISTS idx_order_line_items_cart_item_id;

-- -------------------------------------------------------------------
-- Step 4: reset cart_records checkout_group_id (best effort)
-- -------------------------------------------------------------------
UPDATE cart_records SET checkout_group_id = NULL;

-- +goose StatementEnd
