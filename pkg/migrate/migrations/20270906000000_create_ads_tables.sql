

-- +goose Up
-- +goose StatementBegin

-- Enums (Postgres does NOT support: CREATE TYPE IF NOT EXISTS ... AS ENUM)
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ad_status') THEN
    CREATE TYPE ad_status AS ENUM (
      'draft',
      'active',
      'paused',
      'exhausted',
      'expired',
      'archived'
    );
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ad_target_type') THEN
    CREATE TYPE ad_target_type AS ENUM ('store', 'product');
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS ads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  status ad_status NOT NULL DEFAULT 'draft',
  placement text NOT NULL,
  target_type ad_target_type NOT NULL,
  target_id uuid NOT NULL,
  bid_cents bigint NOT NULL DEFAULT 0,
  daily_budget_cents bigint NOT NULL DEFAULT 0,
  starts_at timestamptz,
  ends_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ads_status_placement_window_idx
  ON ads (status, placement, starts_at, ends_at, store_id);

CREATE INDEX IF NOT EXISTS ads_target_idx
  ON ads (target_type, target_id);

CREATE TABLE IF NOT EXISTS ad_creatives (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_id uuid NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
  media_id uuid REFERENCES media(id) ON DELETE SET NULL,
  destination_url text NOT NULL,
  headline text,
  body text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ad_creatives_ad_id_idx
  ON ad_creatives (ad_id);

CREATE TABLE IF NOT EXISTS ad_daily_rollups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_id uuid NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
  day date NOT NULL,
  impressions bigint NOT NULL DEFAULT 0,
  clicks bigint NOT NULL DEFAULT 0,
  spend_cents bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ad_daily_rollups_ad_day_key UNIQUE (ad_id, day)
);

CREATE INDEX IF NOT EXISTS ad_daily_rollups_ad_id_idx
  ON ad_daily_rollups (ad_id);

ALTER TABLE vendor_orders
  ADD COLUMN IF NOT EXISTS ad_token text;

ALTER TABLE order_line_items
  ADD COLUMN IF NOT EXISTS ad_token text[] NOT NULL DEFAULT '{}'::text[];

ALTER TABLE usage_charges
  ADD COLUMN IF NOT EXISTS usage_type text NOT NULL DEFAULT 'ad_spend';

ALTER TABLE usage_charges
  ADD COLUMN IF NOT EXISTS for_date date NOT NULL DEFAULT CURRENT_DATE;

-- Postgres does NOT support: ADD CONSTRAINT IF NOT EXISTS
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'usage_charges_store_type_for_date_key'
  ) THEN
    ALTER TABLE usage_charges
      ADD CONSTRAINT usage_charges_store_type_for_date_key
      UNIQUE (store_id, usage_type, for_date);
  END IF;
END$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE usage_charges
  DROP CONSTRAINT IF EXISTS usage_charges_store_type_for_date_key;

ALTER TABLE usage_charges
  DROP COLUMN IF EXISTS for_date;

ALTER TABLE usage_charges
  DROP COLUMN IF EXISTS usage_type;

ALTER TABLE order_line_items
  DROP COLUMN IF EXISTS ad_token;

ALTER TABLE vendor_orders
  DROP COLUMN IF EXISTS ad_token;

DROP TABLE IF EXISTS ad_daily_rollups;
DROP TABLE IF EXISTS ad_creatives;
DROP TABLE IF EXISTS ads;

DROP TYPE IF EXISTS ad_target_type;
DROP TYPE IF EXISTS ad_status;

-- +goose StatementEnd