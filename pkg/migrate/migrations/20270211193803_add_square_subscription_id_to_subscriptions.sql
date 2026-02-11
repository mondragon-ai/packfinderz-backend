-- +goose Up
-- +goose StatementBegin

-- 1) Drop NOT NULL so Square-only subscriptions can be inserted
ALTER TABLE subscriptions
ALTER COLUMN stripe_subscription_id DROP NOT NULL;

-- 2) Replace the existing UNIQUE CONSTRAINT (which enforces uniqueness across ALL rows)
-- with a partial unique index that enforces uniqueness only when the value is present.
--
-- Your table currently has:
--   "subscriptions_stripe_subscription_id_key" UNIQUE CONSTRAINT, btree (stripe_subscription_id)
--
-- We must drop it first.
ALTER TABLE subscriptions
DROP CONSTRAINT IF EXISTS subscriptions_stripe_subscription_id_key;

-- 3) Re-create uniqueness only for non-null stripe ids
CREATE UNIQUE INDEX IF NOT EXISTS subscriptions_stripe_subscription_id_uq
ON subscriptions(stripe_subscription_id)
WHERE stripe_subscription_id IS NOT NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Revert to the old behavior (not recommended long-term, but correct as a Down)

DROP INDEX IF EXISTS subscriptions_stripe_subscription_id_uq;

ALTER TABLE subscriptions
ADD CONSTRAINT subscriptions_stripe_subscription_id_key UNIQUE (stripe_subscription_id);

ALTER TABLE subscriptions
ALTER COLUMN stripe_subscription_id SET NOT NULL;

-- +goose StatementEnd
