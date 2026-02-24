-- +goose Up
ALTER TABLE IF EXISTS order_line_items
  ADD COLUMN IF NOT EXISTS thumbnail TEXT NULL;

-- +goose Down
ALTER TABLE IF EXISTS order_line_items
  DROP COLUMN IF EXISTS thumbnail;
