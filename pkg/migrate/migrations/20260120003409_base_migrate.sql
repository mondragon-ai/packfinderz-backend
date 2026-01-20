-- +goose Up
-- +goose StatementBegin
-- base_migrate
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- rollback base_migrate
-- +goose StatementEnd
