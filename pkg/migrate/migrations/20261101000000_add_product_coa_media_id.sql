-- +goose Up
-- +goose StatementBegin

ALTER TABLE products
  ADD COLUMN IF NOT EXISTS coa_media_id uuid NULL;

ALTER TABLE products
  ADD CONSTRAINT IF NOT EXISTS products_coa_media_fk
    FOREIGN KEY (coa_media_id) REFERENCES media(id) ON DELETE SET NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_coa_media_fk;
ALTER TABLE products DROP COLUMN IF EXISTS coa_media_id;

-- +goose StatementEnd
