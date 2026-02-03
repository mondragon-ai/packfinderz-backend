-- +goose Up
ALTER TABLE product_media
  ADD COLUMN media_id uuid NULL;

ALTER TABLE product_media
  ADD CONSTRAINT fk_product_media_media
  FOREIGN KEY (media_id)
  REFERENCES media(id)
  ON DELETE SET NULL;

-- +goose Down
ALTER TABLE product_media
  DROP CONSTRAINT IF EXISTS fk_product_media_media;

ALTER TABLE product_media
  DROP COLUMN media_id;
