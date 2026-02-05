-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
    CREATE TYPE billing_interval AS ENUM (
        'EVERY_30_DAYS',
        'ANNUAL'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE plan_status AS ENUM (
        'active',
        'deprecated',
        'hidden'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE ui_badge AS ENUM (
        'popular',
        'best_value',
        'new'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS billing_plans (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status plan_status NOT NULL,
    square_billing_plan_id TEXT NOT NULL,

    test BOOLEAN NOT NULL DEFAULT FALSE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,

    trial_days INTEGER NOT NULL DEFAULT 0,
    trial_require_payment_method BOOLEAN NOT NULL DEFAULT FALSE,
    trial_start_on_activation BOOLEAN NOT NULL DEFAULT TRUE,

    interval billing_interval NOT NULL,

    price_amount NUMERIC(12,2) NOT NULL,
    currency_code TEXT NOT NULL,

    features TEXT[] DEFAULT '{}'::text[],

    ui JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_plans_status ON billing_plans(status);
CREATE INDEX IF NOT EXISTS idx_billing_plans_default ON billing_plans(is_default);
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_plans_square_id ON billing_plans(square_billing_plan_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS billing_plans;

DROP TYPE IF EXISTS ui_badge;
DROP TYPE IF EXISTS plan_status;
DROP TYPE IF EXISTS billing_interval;

-- +goose StatementEnd
