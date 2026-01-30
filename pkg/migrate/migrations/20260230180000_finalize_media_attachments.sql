-- +goose Up

ALTER TABLE media_attachments
  DROP CONSTRAINT IF EXISTS media_attachments_unique,
  DROP CONSTRAINT IF EXISTS media_attachments_media_fk;

ALTER TABLE media_attachments RENAME COLUMN owner_type TO entity_type;
ALTER TABLE media_attachments RENAME COLUMN owner_id   TO entity_id;

ALTER TABLE media_attachments
  ADD COLUMN IF NOT EXISTS store_id uuid,
  ADD COLUMN IF NOT EXISTS gcs_key  text;

UPDATE media_attachments ma
SET store_id = COALESCE(
      m.store_id,
      CASE WHEN ma.entity_type = 'store' THEN ma.entity_id END
    ),
    gcs_key = m.gcs_key
FROM media m
WHERE ma.media_id = m.id;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM media_attachments WHERE store_id IS NULL) THEN
    RAISE EXCEPTION 'media_attachments store_id could not be populated';
  END IF;

  IF EXISTS (SELECT 1 FROM media_attachments WHERE gcs_key IS NULL) THEN
    RAISE EXCEPTION 'media_attachments gcs_key could not be populated';
  END IF;
END$$;
-- +goose StatementEnd

ALTER TABLE media_attachments
  ALTER COLUMN store_id SET NOT NULL,
  ALTER COLUMN gcs_key  SET NOT NULL;

ALTER TABLE media_attachments
  ADD CONSTRAINT media_attachments_media_fk FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE RESTRICT,
  ADD CONSTRAINT media_attachments_store_fk FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE RESTRICT;

DROP INDEX IF EXISTS idx_media_attachments_owner;

CREATE INDEX IF NOT EXISTS idx_media_attachments_entity ON media_attachments (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_media_attachments_media  ON media_attachments (media_id);

-- +goose Down

ALTER TABLE media_attachments
  DROP CONSTRAINT IF EXISTS media_attachments_store_fk,
  DROP CONSTRAINT IF EXISTS media_attachments_media_fk;

DROP INDEX IF EXISTS idx_media_attachments_media;
DROP INDEX IF EXISTS idx_media_attachments_entity;

ALTER TABLE media_attachments
  DROP COLUMN IF EXISTS store_id,
  DROP COLUMN IF EXISTS gcs_key;

ALTER TABLE media_attachments RENAME COLUMN entity_type TO owner_type;
ALTER TABLE media_attachments RENAME COLUMN entity_id   TO owner_id;

ALTER TABLE media_attachments
  ADD CONSTRAINT media_attachments_media_fk FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
  ADD CONSTRAINT media_attachments_unique UNIQUE (media_id, owner_type, owner_id);

CREATE INDEX IF NOT EXISTS idx_media_attachments_owner ON media_attachments (owner_type, owner_id);
