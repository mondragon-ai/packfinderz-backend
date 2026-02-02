-- +goose Up
ALTER TABLE users
DROP COLUMN IF EXISTS store_ids;

-- +goose Down
ALTER TABLE users
ADD COLUMN store_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[];
