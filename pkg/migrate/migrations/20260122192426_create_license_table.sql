-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'license_status') THEN
    CREATE TYPE license_status AS ENUM (
      'pending',
      'verified',
      'rejected',
      'expired'
    );
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'license_type') THEN
    CREATE TYPE license_type AS ENUM (
      'producer',
      'grower',
      'dispensary',
      'merchant'
    );
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS licenses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores (id),
  user_id uuid NOT NULL REFERENCES users (id),
  status license_status NOT NULL DEFAULT 'pending',
  media_id uuid NOT NULL REFERENCES media (id),
  issuing_state text NOT NULL,
  issue_date timestamptz,
  expiration_date timestamptz,
  type license_type NOT NULL,
  number text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS licenses_store_status_idx ON licenses (store_id, status);
CREATE INDEX IF NOT EXISTS licenses_expiration_idx ON licenses (expiration_date);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS licenses;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'license_type') THEN
    DROP TYPE license_type;
  END IF;
END$$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'license_status') THEN
    DROP TYPE license_status;
  END IF;
END$$;

-- +goose StatementEnd
