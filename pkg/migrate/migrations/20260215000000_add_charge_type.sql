-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
    CREATE TYPE charge_type AS ENUM (
        'subscription',
        'ad_spend',
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE charges
    ADD COLUMN IF NOT EXISTS type charge_type NOT NULL DEFAULT 'subscription';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE charges
    DROP COLUMN IF EXISTS type;

DROP TYPE IF EXISTS charge_type;

-- +goose StatementEnd
