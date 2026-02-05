-- +goose Up
ALTER TABLE subscriptions ADD COLUMN billing_plan_id TEXT;
ALTER TABLE subscriptions ADD COLUMN square_customer_id TEXT;
ALTER TABLE subscriptions ADD COLUMN square_card_id TEXT;
ALTER TABLE subscriptions ADD COLUMN paused_at TIMESTAMPTZ;
DROP INDEX IF EXISTS subscriptions_store_idx;
CREATE UNIQUE INDEX IF NOT EXISTS subscriptions_store_id_uq ON subscriptions (store_id);

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN IF EXISTS paused_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS square_card_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS square_customer_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS billing_plan_id;
DROP INDEX IF EXISTS subscriptions_store_id_uq;
CREATE INDEX IF NOT EXISTS subscriptions_store_idx ON subscriptions (store_id);
