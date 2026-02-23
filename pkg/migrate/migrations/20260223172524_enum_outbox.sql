-- +goose Up
-- +goose StatementBegin
-- enum_outbox
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- rollback enum_outbox
-- +goose StatementEnd
