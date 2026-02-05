-- +goose Up
ALTER TABLE stores ADD COLUMN square_customer_id TEXT;

-- +goose Down
ALTER TABLE stores DROP COLUMN IF EXISTS square_customer_id;
