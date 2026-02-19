-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS wishlist_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id uuid NOT NULL,
    product_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE wishlist_items
ADD CONSTRAINT wishlist_items_store_product_key UNIQUE (store_id, product_id);

CREATE INDEX IF NOT EXISTS wishlist_items_store_id_idx
    ON wishlist_items (store_id);

CREATE INDEX IF NOT EXISTS wishlist_items_product_id_idx
    ON wishlist_items (product_id);

ALTER TABLE wishlist_items
ADD CONSTRAINT wishlist_items_store_id_fkey FOREIGN KEY (store_id) REFERENCES stores (id) ON DELETE CASCADE;

ALTER TABLE wishlist_items
ADD CONSTRAINT wishlist_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS wishlist_items;
-- +goose StatementEnd
