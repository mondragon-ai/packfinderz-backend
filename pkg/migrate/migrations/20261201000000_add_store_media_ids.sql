-- +goose Up
ALTER TABLE stores
  ADD COLUMN logo_media_id uuid NULL,
  ADD COLUMN banner_media_id uuid NULL;

ALTER TABLE stores
  ADD CONSTRAINT stores_logo_media_fk FOREIGN KEY (logo_media_id) REFERENCES media(id) ON DELETE RESTRICT,
  ADD CONSTRAINT stores_banner_media_fk FOREIGN KEY (banner_media_id) REFERENCES media(id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE stores DROP CONSTRAINT IF EXISTS stores_banner_media_fk;
ALTER TABLE stores DROP CONSTRAINT IF EXISTS stores_logo_media_fk;
ALTER TABLE stores DROP COLUMN IF EXISTS banner_media_id;
ALTER TABLE stores DROP COLUMN IF EXISTS logo_media_id;
