-- +goose Up

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_type
    WHERE typname = 'review_type'
  ) THEN
    CREATE TYPE review_type AS ENUM ('store', 'product');
  END IF;
END;
$$;
-- +goose StatementEnd

CREATE TABLE reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  review_type review_type NOT NULL,

  buyer_store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  buyer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

  vendor_store_id UUID REFERENCES stores(id) ON DELETE CASCADE,
  product_id UUID REFERENCES products(id) ON DELETE CASCADE,
  order_id UUID REFERENCES vendor_orders(id) ON DELETE SET NULL,

  rating SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
  title VARCHAR(150),
  body TEXT,

  is_verified_purchase BOOLEAN NOT NULL DEFAULT false,
  is_visible BOOLEAN NOT NULL DEFAULT true,

  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX reviews_vendor_store_id_idx ON reviews(vendor_store_id);
CREATE INDEX reviews_created_at_idx ON reviews(created_at);
CREATE INDEX reviews_vendor_store_created_at_idx ON reviews(vendor_store_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS reviews;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_type t
    WHERE t.typname = 'review_type'
      AND NOT EXISTS (
        SELECT 1
        FROM pg_depend d
        WHERE d.refclassid = 'pg_type'::regclass
          AND d.refobjid = t.oid
      )
  ) THEN
    DROP TYPE review_type;
  END IF;
END;
$$;
-- +goose StatementEnd