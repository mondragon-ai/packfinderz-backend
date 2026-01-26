-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  CREATE TYPE vendor_order_fulfillment_status AS ENUM (
    'pending',
    'partial',
    'fulfilled'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE vendor_order_shipping_status AS ENUM (
    'pending',
    'dispatched',
    'in_transit',
    'delivered'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

CREATE SEQUENCE IF NOT EXISTS vendor_order_number_seq START 1;

ALTER TABLE vendor_orders
  ADD COLUMN IF NOT EXISTS fulfillment_status vendor_order_fulfillment_status NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS shipping_status vendor_order_shipping_status NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS order_number bigint NOT NULL DEFAULT nextval('vendor_order_number_seq');

CREATE UNIQUE INDEX IF NOT EXISTS ux_vendor_orders_order_number ON vendor_orders (order_number);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_vendor_orders_order_number;

ALTER TABLE vendor_orders
  DROP COLUMN IF EXISTS order_number,
  DROP COLUMN IF EXISTS shipping_status,
  DROP COLUMN IF EXISTS fulfillment_status;

DROP SEQUENCE IF EXISTS vendor_order_number_seq;

DROP TYPE IF EXISTS vendor_order_shipping_status;
DROP TYPE IF EXISTS vendor_order_fulfillment_status;

-- +goose StatementEnd
