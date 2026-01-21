-- +goose Up
-- +goose StatementBegin

CREATE TYPE IF NOT EXISTS media_kind AS ENUM (
  'product',
  'ads',
  'pdf',
  'license_doc',
  'coa',
  'manifest',
  'user',
  'other'
);

CREATE TABLE IF NOT EXISTS media (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NULL,
  user_id uuid NULL,
  kind media_kind NOT NULL,
  url text NULL,
  gsc_key text NOT NULL UNIQUE,
  file_name text NOT NULL,
  mime_type text NOT NULL,
  ocr text NULL,
  size_bytes bigint NOT NULL,
  is_compressed boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT media_store_fk FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE SET NULL,
  CONSTRAINT media_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS media_attachments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  media_id uuid NOT NULL,
  owner_type text NOT NULL,
  owner_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT media_attachments_media_fk FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
  CONSTRAINT media_attachments_unique UNIQUE (media_id, owner_type, owner_id)
);

CREATE INDEX IF NOT EXISTS idx_media_store_created_at ON media (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_kind ON media (kind);
CREATE INDEX IF NOT EXISTS idx_media_user ON media (user_id);
CREATE INDEX IF NOT EXISTS idx_media_attachments_owner ON media_attachments (owner_type, owner_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS media_attachments;
DROP TABLE IF EXISTS media;
DROP TYPE IF EXISTS media_kind;

-- +goose StatementEnd
