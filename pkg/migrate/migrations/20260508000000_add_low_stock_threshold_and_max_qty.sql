-- +goose Up
-- +goose StatementBegin

ALTER TABLE inventory_items
ADD COLUMN IF NOT EXISTS low_stock_threshold integer NOT NULL DEFAULT 0;

ALTER TABLE products
ADD COLUMN IF NOT EXISTS max_qty integer NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE inventory_items
DROP COLUMN IF EXISTS low_stock_threshold;

ALTER TABLE products
DROP COLUMN IF EXISTS max_qty;

-- +goose StatementEnd
