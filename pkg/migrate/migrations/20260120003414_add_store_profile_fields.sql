-- +goose Up
-- +goose StatementBegin

ALTER TABLE stores
  ADD COLUMN IF NOT EXISTS banner_url text NULL,
  ADD COLUMN IF NOT EXISTS logo_url text NULL,
  ADD COLUMN IF NOT EXISTS ratings jsonb NULL,
  ADD COLUMN IF NOT EXISTS categories text[] NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE stores
  DROP COLUMN IF EXISTS banner_url,
  DROP COLUMN IF EXISTS logo_url,
  DROP COLUMN IF EXISTS ratings,
  DROP COLUMN IF EXISTS categories;

-- +goose StatementEnd
