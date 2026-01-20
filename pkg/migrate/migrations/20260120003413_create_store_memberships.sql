-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
	CREATE TYPE member_role AS ENUM (
		'owner',
		'admin',
		'manager',
		'viewer',
		'agent',
		'staff',
		'ops'
	);
EXCEPTION
	WHEN duplicate_object THEN NULL;
END$$;

DO $$
BEGIN
	CREATE TYPE membership_status AS ENUM (
		'invited',
		'active',
		'removed',
		'pending'
	);
EXCEPTION
	WHEN duplicate_object THEN NULL;
END$$;

CREATE TABLE IF NOT EXISTS store_memberships (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
	user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role member_role NOT NULL,
	status membership_status NOT NULL,
	invited_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (store_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_store_memberships_user_id ON store_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_store_memberships_store_role ON store_memberships(store_id, role);
CREATE INDEX IF NOT EXISTS idx_store_memberships_store_status ON store_memberships(store_id, status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS store_memberships;
DROP TYPE IF EXISTS membership_status;
DROP TYPE IF EXISTS member_role;
-- +goose StatementEnd
