-- +goose Up
DROP INDEX IF EXISTS stores_geom_gist_idx;
ALTER TABLE IF EXISTS stores DROP COLUMN IF EXISTS geom;

DO $$
BEGIN
  CREATE TYPE store_badge AS ENUM ('top_brand', 'quality_verified');
EXCEPTION WHEN duplicate_object THEN NULL;
END$$;

ALTER TABLE stores
  ADD COLUMN IF NOT EXISTS last_logged_in_at timestamptz NULL,
  ADD COLUMN IF NOT EXISTS badge store_badge NULL;

-- +goose Down
ALTER TABLE stores DROP COLUMN IF EXISTS badge;
ALTER TABLE stores DROP COLUMN IF EXISTS last_logged_in_at;
DROP TYPE IF EXISTS store_badge;
ALTER TABLE IF EXISTS stores ADD COLUMN IF NOT EXISTS geom geography(Point,4326);
CREATE INDEX IF NOT EXISTS stores_geom_gist_idx ON stores USING GIST (geom);
