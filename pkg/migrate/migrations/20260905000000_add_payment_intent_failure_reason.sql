-- +goose Up
ALTER TABLE payment_intents
    ADD COLUMN failure_reason TEXT;

-- +goose Down
ALTER TABLE payment_intents
    DROP COLUMN failure_reason;
