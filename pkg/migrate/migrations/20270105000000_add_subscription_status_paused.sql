-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
    ALTER TYPE subscription_status ADD VALUE 'paused';
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Downgrading enum values is not supported in Postgres; no-op.
SELECT 1;

-- +goose StatementEnd
