-- +goose Up
-- +goose StatementBegin

-- =========================
-- ENUM / TYPE DEFINITIONS
-- =========================

DO $$
BEGIN
  CREATE TYPE vendor_order_status AS ENUM (
    'created_pending',
    'accepted',
    'partially_accepted',
    'rejected',
    'fulfilled',
    'partially_fulfilled',
    'hold',
    'hold_on_payment',
    'hold_on_pickup',
    'in_transit',
    'delivered',
    'closed',
    'canceled',
    'expired'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE refund_status AS ENUM (
    'none',
    'partial',
    'full'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE line_item_status AS ENUM (
    'pending',
    'accepted',
    'rejected',
    'fulfilled',
    'hold'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE payment_method AS ENUM (
    'cash',
    'ach'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE payment_status AS ENUM (
    'unpaid',
    'pending',
    'settled',
    'paid'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

-- =========================
-- TABLES
-- =========================

-- NOTE: We intentionally DO NOT create an FK to ad_clicks here, because that
-- table does not exist yet in your schema (caused SQLSTATE 42P01).
-- We keep the column for future attribution and will add the FK later when ads ship.

CREATE TABLE IF NOT EXISTS checkout_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_store_id uuid NOT NULL,
  cart_id uuid NULL,
  attributed_ad_click_id uuid NULL,
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT checkout_groups_buyer_fk
    FOREIGN KEY (buyer_store_id) REFERENCES stores(id) ON DELETE RESTRICT,

  CONSTRAINT checkout_groups_cart_fk
    FOREIGN KEY (cart_id) REFERENCES cart_records(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_checkout_groups_buyer_created
  ON checkout_groups (buyer_store_id, created_at DESC);

CREATE TABLE IF NOT EXISTS vendor_orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  checkout_group_id uuid NOT NULL,
  buyer_store_id uuid NOT NULL,
  vendor_store_id uuid NOT NULL,

  status vendor_order_status NOT NULL DEFAULT 'created_pending',
  refund_status refund_status NOT NULL DEFAULT 'none',

  subtotal_cents int NOT NULL,
  discount_cents int NOT NULL DEFAULT 0,
  tax_cents int NOT NULL DEFAULT 0,
  transport_fee_cents int NOT NULL DEFAULT 0,
  total_cents int NOT NULL,
  balance_due_cents int NOT NULL DEFAULT 0,

  notes text NULL,
  internal_notes text NULL,

  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  fulfilled_at timestamptz NULL,
  delivered_at timestamptz NULL,
  canceled_at timestamptz NULL,
  expired_at timestamptz NULL,

  CONSTRAINT vendor_orders_checkout_fk
    FOREIGN KEY (checkout_group_id) REFERENCES checkout_groups(id) ON DELETE CASCADE,

  CONSTRAINT vendor_orders_buyer_fk
    FOREIGN KEY (buyer_store_id) REFERENCES stores(id) ON DELETE RESTRICT,

  CONSTRAINT vendor_orders_vendor_fk
    FOREIGN KEY (vendor_store_id) REFERENCES stores(id) ON DELETE RESTRICT,

  CONSTRAINT vendor_orders_buyer_vendor_diff
    CHECK (buyer_store_id <> vendor_store_id)
);

CREATE INDEX IF NOT EXISTS idx_vendor_orders_buyer_created
  ON vendor_orders (buyer_store_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_vendor_orders_vendor_created
  ON vendor_orders (vendor_store_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_vendor_orders_status
  ON vendor_orders (status);

CREATE UNIQUE INDEX IF NOT EXISTS ux_vendor_orders_group_vendor
  ON vendor_orders (checkout_group_id, vendor_store_id);

CREATE TABLE IF NOT EXISTS order_line_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id uuid NOT NULL,
  product_id uuid NULL,

  name text NOT NULL,
  category text NOT NULL,
  strain text NULL,
  classification text NULL,

  unit unit NOT NULL,
  unit_price_cents int NOT NULL,
  qty int NOT NULL,
  discount_cents int NOT NULL DEFAULT 0,
  total_cents int NOT NULL,

  status line_item_status NOT NULL DEFAULT 'pending',
  notes text NULL,

  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT order_line_items_order_fk
    FOREIGN KEY (order_id) REFERENCES vendor_orders(id) ON DELETE CASCADE,

  CONSTRAINT order_line_items_product_fk
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_order_line_items_order_id
  ON order_line_items (order_id);

CREATE INDEX IF NOT EXISTS idx_order_line_items_product_id
  ON order_line_items (product_id);

CREATE INDEX IF NOT EXISTS idx_order_line_items_status
  ON order_line_items (status);

CREATE TABLE IF NOT EXISTS payment_intents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id uuid NOT NULL,

  method payment_method NOT NULL DEFAULT 'cash',
  status payment_status NOT NULL DEFAULT 'unpaid',
  amount_cents int NOT NULL,

  cash_collected_at timestamptz NULL,
  vendor_paid_at timestamptz NULL,

  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT payment_intents_order_fk
    FOREIGN KEY (order_id) REFERENCES vendor_orders(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_intents_order_id
  ON payment_intents (order_id);

CREATE INDEX IF NOT EXISTS idx_payment_intents_status
  ON payment_intents (status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS payment_intents;
DROP TABLE IF EXISTS order_line_items;
DROP TABLE IF EXISTS vendor_orders;
DROP TABLE IF EXISTS checkout_groups;

-- Types are shared by these tables; safe to drop after dropping tables.
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS line_item_status;
DROP TYPE IF EXISTS refund_status;
DROP TYPE IF EXISTS vendor_order_status;

-- +goose StatementEnd
