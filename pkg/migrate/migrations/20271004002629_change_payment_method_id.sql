-- +goose Up
-- Remove Stripe payment method column and replace with Square payment method column

ALTER TABLE payment_methods
DROP CONSTRAINT IF EXISTS payment_methods_stripe_payment_method_id_key;

ALTER TABLE payment_methods
DROP COLUMN IF EXISTS stripe_payment_method_id;

ALTER TABLE payment_methods
ADD COLUMN square_payment_method_id TEXT NOT NULL;

ALTER TABLE payment_methods
ADD CONSTRAINT payment_methods_square_payment_method_id_key
UNIQUE (square_payment_method_id);


-- +goose Down
-- Revert back to Stripe column

ALTER TABLE payment_methods
DROP CONSTRAINT IF EXISTS payment_methods_square_payment_method_id_key;

ALTER TABLE payment_methods
DROP COLUMN IF EXISTS square_payment_method_id;

ALTER TABLE payment_methods
ADD COLUMN stripe_payment_method_id TEXT NOT NULL;

ALTER TABLE payment_methods
ADD CONSTRAINT payment_methods_stripe_payment_method_id_key
UNIQUE (stripe_payment_method_id);