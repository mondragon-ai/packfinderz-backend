-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    password_hash text NOT NULL,
    first_name text NOT NULL,
    last_name text NOT NULL,
    phone text NULL,
    is_active boolean NOT NULL DEFAULT true,
    last_login_at timestamptz NULL,
    system_role text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE users
ADD CONSTRAINT users_email_key UNIQUE (email);

CREATE INDEX IF NOT EXISTS users_system_role_not_null_idx
    ON users (system_role)
    WHERE system_role IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
