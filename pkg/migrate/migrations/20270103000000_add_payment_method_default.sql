-- +goose Up
-- +goose StatementBegin

ALTER TABLE payment_methods
    ADD COLUMN IF NOT EXISTS is_default boolean NOT NULL DEFAULT false;

CREATE UNIQUE INDEX IF NOT EXISTS payment_methods_store_default_idx ON payment_methods (store_id)
    WHERE is_default;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS payment_methods_store_default_idx;
ALTER TABLE payment_methods DROP COLUMN IF EXISTS is_default;

-- +goose StatementEnd
