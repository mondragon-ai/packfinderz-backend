-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'notification_type') THEN
    CREATE TYPE notification_type AS ENUM (
      'system_announcement',
      'market_update',
      'security_alert',
      'order_alert',
      'compliance'
    );
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL,
  type notification_type NOT NULL,
  title text NOT NULL,
  message text NOT NULL,
  link text NULL,
  read_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notifications_store_created_idx ON notifications (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS notifications_store_read_idx ON notifications (store_id, read_at);
CREATE INDEX IF NOT EXISTS notifications_created_idx ON notifications (created_at);

ALTER TABLE notifications
  ADD CONSTRAINT notifications_store_fk FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS notifications;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'notification_type') THEN
    DROP TYPE notification_type;
  END IF;
END$$;

-- +goose StatementEnd
