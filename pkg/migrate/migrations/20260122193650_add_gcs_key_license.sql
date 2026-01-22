-- +goose Up
-- +goose StatementBegin

ALTER TABLE licenses
  ADD COLUMN IF NOT EXISTS gcs_key text NOT NULL UNIQUE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE licenses
  DROP COLUMN IF EXISTS gcs_key;

-- +goose StatementEnd
