-- +goose Up
CREATE TYPE media_status AS ENUM (
  'pending',
  'uploaded',
  'processing',
  'ready',
  'failed',
  'delete_requested',
  'deleted',
  'delete_failed'
);

ALTER TABLE media
  ADD COLUMN status media_status NOT NULL DEFAULT 'pending',
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN uploaded_at timestamptz NULL,
  ADD COLUMN verified_at timestamptz NULL,
  ADD COLUMN processing_started_at timestamptz NULL,
  ADD COLUMN ready_at timestamptz NULL,
  ADD COLUMN failed_at timestamptz NULL,
  ADD COLUMN deleted_at timestamptz NULL;

-- +goose Down
ALTER TABLE media
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS uploaded_at,
  DROP COLUMN IF EXISTS verified_at,
  DROP COLUMN IF EXISTS processing_started_at,
  DROP COLUMN IF EXISTS ready_at,
  DROP COLUMN IF EXISTS failed_at,
  DROP COLUMN IF EXISTS deleted_at;

DROP TYPE media_status;
