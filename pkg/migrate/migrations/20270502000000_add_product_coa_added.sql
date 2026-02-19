-- +goose Up
-- +goose StatementBegin
ALTER TABLE products
ADD COLUMN IF NOT EXISTS coa_added boolean NOT NULL DEFAULT false;

UPDATE products
SET coa_added = true
WHERE coa_media_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE products
DROP COLUMN IF EXISTS coa_added;
-- +goose StatementEnd
