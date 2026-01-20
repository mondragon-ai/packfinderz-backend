-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'store_type') THEN
    CREATE TYPE store_type AS ENUM ('buyer', 'vendor');
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'kyc_status') THEN
    CREATE TYPE kyc_status AS ENUM (
      'pending_verification',
      'verified',
      'rejected',
      'expired',
      'suspended'
    );
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'address_t') THEN
    CREATE TYPE address_t AS (
      line1 text,
      line2 text,
      city text,
      state text,
      postal_code text,
      country text,
      lat double precision,
      lng double precision,
      geohash text
    );
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'social_t') THEN
    CREATE TYPE social_t AS (
      twitter text,
      facebook text,
      instagram text,
      linkedin text,
      youtube text,
      website text
    );
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS stores (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type store_type NOT NULL,
  company_name text NOT NULL,
  dba_name text NULL,
  description text NULL,
  phone text NULL,
  email text NULL,
  kyc_status kyc_status NOT NULL DEFAULT 'pending_verification',
  subscription_active boolean NOT NULL DEFAULT false,
  delivery_radius_meters integer NOT NULL DEFAULT 0,
  address address_t NOT NULL,
  geom geography(Point,4326) NOT NULL,
  social social_t NULL,
  owner uuid NOT NULL REFERENCES users (id),
  last_active_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((address).lat IS NOT NULL AND (address).lng IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS stores_geom_gist_idx ON stores USING GIST (geom);
CREATE INDEX IF NOT EXISTS stores_type_kyc_status_idx ON stores (type, kyc_status);
CREATE INDEX IF NOT EXISTS stores_subscription_idx ON stores (subscription_active);

-- +goose StatementEnd
