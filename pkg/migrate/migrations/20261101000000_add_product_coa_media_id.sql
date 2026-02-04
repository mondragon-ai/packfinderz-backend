-- 20261101000000_add_product_coa_media_id.sql

-- +goose Up
-- +goose StatementBegin

ALTER TABLE products
  ADD COLUMN IF NOT EXISTS coa_media_id uuid NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint c
    WHERE c.conname = 'products_coa_media_fk'
      AND c.conrelid = 'products'::regclass
  ) THEN
    ALTER TABLE products
      ADD CONSTRAINT products_coa_media_fk
        FOREIGN KEY (coa_media_id) REFERENCES media(id) ON DELETE SET NULL;
  END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_coa_media_fk;
ALTER TABLE products DROP COLUMN IF EXISTS coa_media_id;

-- +goose StatementEnd
