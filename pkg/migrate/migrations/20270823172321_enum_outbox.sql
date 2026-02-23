-- +goose Up
-- +goose NO TRANSACTION
ALTER TYPE event_type_enum
ADD VALUE IF NOT EXISTS 'checkout_converted';

-- +goose Down
-- (Down migrations for enum values are non-trivial; usually omit or document)