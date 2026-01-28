-- +goose Up
-- +goose StatementBegin

-- Enums (idempotent across PG versions)
DO $$
BEGIN
    CREATE TYPE subscription_status AS ENUM (
        'trialing',
        'active',
        'past_due',
        'canceled',
        'incomplete',
        'incomplete_expired',
        'unpaid'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE charge_status AS ENUM (
        'pending',
        'succeeded',
        'failed',
        'refunded'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE payment_method_type AS ENUM (
        'card',
        'us_bank_account',
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Tables
CREATE TABLE IF NOT EXISTS subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    stripe_subscription_id text NOT NULL UNIQUE,
    status subscription_status NOT NULL DEFAULT 'active',
    price_id text,
    current_period_start timestamptz,
    current_period_end timestamptz NOT NULL,
    cancel_at_period_end boolean NOT NULL DEFAULT false,
    canceled_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS subscriptions_store_idx ON subscriptions (store_id);

CREATE TABLE IF NOT EXISTS payment_methods (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    stripe_payment_method_id text NOT NULL UNIQUE,
    type payment_method_type NOT NULL DEFAULT 'card',
    fingerprint text,
    card_brand text,
    card_last4 text,
    card_exp_month integer,
    card_exp_year integer,
    billing_details jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS payment_methods_store_idx ON payment_methods (store_id);

CREATE TABLE IF NOT EXISTS charges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    subscription_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL,
    payment_method_id uuid REFERENCES payment_methods(id) ON DELETE SET NULL,
    stripe_charge_id text NOT NULL UNIQUE,
    amount_cents bigint NOT NULL,
    currency text NOT NULL DEFAULT 'usd',
    status charge_status NOT NULL DEFAULT 'pending',
    description text,
    billed_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS charges_store_idx ON charges (store_id);

CREATE TABLE IF NOT EXISTS usage_charges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    subscription_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL,
    charge_id uuid REFERENCES charges(id) ON DELETE SET NULL,
    stripe_usage_charge_id text NOT NULL UNIQUE,
    quantity bigint NOT NULL,
    amount_cents bigint NOT NULL,
    currency text NOT NULL DEFAULT 'usd',
    description text,
    billing_period_start timestamptz,
    billing_period_end timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS usage_charges_store_idx ON usage_charges (store_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS usage_charges;
DROP TABLE IF EXISTS charges;
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS subscriptions;

-- Types (only after dependent tables are gone)
DROP TYPE IF EXISTS payment_method_type;
DROP TYPE IF EXISTS charge_status;
DROP TYPE IF EXISTS subscription_status;

-- +goose StatementEnd
