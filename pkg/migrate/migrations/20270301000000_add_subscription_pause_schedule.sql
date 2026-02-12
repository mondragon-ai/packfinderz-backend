-- +goose Up
ALTER TABLE subscriptions ADD COLUMN pause_effective_at timestamptz;
ALTER TABLE subscriptions ADD COLUMN resume_effective_at timestamptz;

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN IF EXISTS resume_effective_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS pause_effective_at;
