-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_event_type_enum') THEN
    CREATE TYPE ledger_event_type_enum AS ENUM (
      'cash_collected',
      'vendor_payout',
      'adjustment',
      'refund'
    );
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS ledger_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id uuid NOT NULL,
  buyer_store_id uuid NOT NULL,
  vendor_store_id uuid NOT NULL,
  actor_user_id uuid NOT NULL,
  type ledger_event_type_enum NOT NULL,
  amount_cents integer NOT NULL,
  metadata jsonb NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ledger_events_order_fk FOREIGN KEY (order_id) REFERENCES vendor_orders(id) ON DELETE RESTRICT,
  CONSTRAINT ledger_events_buyer_store_fk FOREIGN KEY (buyer_store_id) REFERENCES stores(id) ON DELETE RESTRICT,
  CONSTRAINT ledger_events_vendor_store_fk FOREIGN KEY (vendor_store_id) REFERENCES stores(id) ON DELETE RESTRICT,
  CONSTRAINT ledger_events_actor_fk FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS ledger_events_order_created_idx ON ledger_events (order_id, created_at);
CREATE INDEX IF NOT EXISTS ledger_events_type_created_idx ON ledger_events (type, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS ledger_events;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_event_type_enum') THEN
    DROP TYPE ledger_event_type_enum;
  END IF;
END$$;

-- +goose StatementEnd
