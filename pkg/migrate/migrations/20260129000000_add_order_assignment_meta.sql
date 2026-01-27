-- +goose Up
-- +goose StatementBegin

ALTER TABLE order_assignments
  ADD COLUMN IF NOT EXISTS pickup_time timestamptz NULL,
  ADD COLUMN IF NOT EXISTS delivery_time timestamptz NULL,
  ADD COLUMN IF NOT EXISTS cash_pickup_time timestamptz NULL,
  ADD COLUMN IF NOT EXISTS pickup_signature_gcs_key text NULL,
  ADD COLUMN IF NOT EXISTS delivery_signature_gcs_key text NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE order_assignments
  DROP COLUMN IF EXISTS delivery_signature_gcs_key,
  DROP COLUMN IF EXISTS pickup_signature_gcs_key,
  DROP COLUMN IF EXISTS cash_pickup_time,
  DROP COLUMN IF EXISTS delivery_time,
  DROP COLUMN IF EXISTS pickup_time;

-- +goose StatementEnd
