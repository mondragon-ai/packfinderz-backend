-- +goose Up
-- +goose StatementBegin
ALTER TABLE media ADD COLUMN public_url text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE media DROP COLUMN public_url;
-- +goose StatementEnd
