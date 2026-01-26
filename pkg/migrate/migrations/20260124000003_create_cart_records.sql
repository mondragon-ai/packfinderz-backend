-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  CREATE TYPE cart_status AS ENUM (
    'active',
    'converted'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE cart_level_discount AS (
    "type" text,
    title text,
    id uuid,
    value text,
    value_type text,
    vendor_id uuid
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS cart_records (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_store_id uuid NOT NULL,
  session_id text NULL,
  status cart_status NOT NULL DEFAULT 'active',
  shipping_address address_t NULL,
  total_discount int NOT NULL DEFAULT 0,
  fees int NOT NULL DEFAULT 0,
  subtotal_cents int NOT NULL DEFAULT 0,
  total_cents int NOT NULL DEFAULT 0,
  cart_level_discount cart_level_discount[] NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cart_records_buyer_fk FOREIGN KEY (buyer_store_id) REFERENCES stores(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_cart_records_buyer_status ON cart_records (buyer_store_id, status);
CREATE INDEX IF NOT EXISTS idx_cart_records_session ON cart_records (session_id);

CREATE TABLE IF NOT EXISTS cart_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cart_id uuid NOT NULL,
  product_id uuid NOT NULL,
  vendor_store_id uuid NOT NULL,
  qty int NOT NULL,
  product_sku text NOT NULL,
  unit unit NOT NULL,
  unit_price_cents int NOT NULL,
  compare_at_unit_price_cents int NULL,
  applied_volume_tier_min_qty int NULL,
  applied_volume_tier_unit_price_cents int NULL,
  discounted_price int NULL,
  sub_total_price int NULL,
  featured_image text NULL,
  moq int NULL,
  thc_percent numeric(5,2) NULL,
  cbd_percent numeric(5,2) NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cart_items_cart_fk FOREIGN KEY (cart_id) REFERENCES cart_records(id) ON DELETE CASCADE,
  CONSTRAINT cart_items_product_fk FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT,
  CONSTRAINT cart_items_vendor_fk FOREIGN KEY (vendor_store_id) REFERENCES stores(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id ON cart_items (cart_id);
CREATE INDEX IF NOT EXISTS idx_cart_items_vendor_store ON cart_items (vendor_store_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS cart_records;
DROP TYPE IF EXISTS cart_level_discount;
DROP TYPE IF EXISTS cart_status;

-- +goose StatementEnd
