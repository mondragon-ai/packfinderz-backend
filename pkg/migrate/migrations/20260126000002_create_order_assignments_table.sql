-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS order_assignments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id uuid NOT NULL,
  agent_user_id uuid NOT NULL,
  assigned_by_user_id uuid NULL,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  unassigned_at timestamptz NULL,
  active boolean NOT NULL DEFAULT true,
  CONSTRAINT order_assignments_order_fk FOREIGN KEY (order_id) REFERENCES vendor_orders(id) ON DELETE CASCADE,
  CONSTRAINT order_assignments_agent_fk FOREIGN KEY (agent_user_id) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT order_assignments_assigned_by_fk FOREIGN KEY (assigned_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_order_assignments_agent_active ON order_assignments (agent_user_id, active);
CREATE INDEX IF NOT EXISTS idx_order_assignments_order ON order_assignments (order_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_order_assignments_order_active ON order_assignments (order_id) WHERE active = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_order_assignments_order_active;
DROP INDEX IF EXISTS idx_order_assignments_order;
DROP INDEX IF EXISTS idx_order_assignments_agent_active;
DROP TABLE IF EXISTS order_assignments;

-- +goose StatementEnd
