-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS inventory_items (
    product_id uuid PRIMARY KEY,
    available_qty int NOT NULL DEFAULT 0,
    reserved_qty int NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT inventory_items_product_fk FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
    CONSTRAINT inventory_items_available_check CHECK (available_qty >= 0),
    CONSTRAINT inventory_items_reserved_check CHECK (reserved_qty >= 0)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS inventory_items;

-- +goose StatementEnd
