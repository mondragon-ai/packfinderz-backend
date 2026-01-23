-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  CREATE TYPE category AS ENUM (
    'flower',
    'cart',
    'pre_roll',
    'edible',
    'concentrate',
    'beverage',
    'vape',
    'topical',
    'tincture',
    'seed',
    'seedling',
    'accessory'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE classification AS ENUM (
    'sativa',
    'hybrid',
    'indica',
    'cbd',
    'hemp',
    'balanced'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE unit AS ENUM (
    'unit',
    'gram',
    'ounce',
    'pound',
    'eighth',
    'sixteenth'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE flavors AS ENUM (
    'earthy',
    'citrus',
    'fruity',
    'floral',
    'cheese',
    'diesel',
    'spicy',
    'sweet',
    'pine',
    'herbal'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE feelings AS ENUM (
    'relaxed',
    'happy',
    'euphoric',
    'focused',
    'hungry',
    'talkative',
    'creative',
    'sleepy',
    'uplifted',
    'calm'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  CREATE TYPE usage AS ENUM (
    'stress_relief',
    'pain_relief',
    'sleep',
    'depression',
    'muscle_relaxant',
    'nausea',
    'anxiety',
    'appetite_stimulation'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL,
  sku text NOT NULL,
  title text NOT NULL,
  subtitle text NULL,
  body_html text NULL,
  category category NOT NULL,
  feelings feelings[] NOT NULL DEFAULT ARRAY[]::feelings[],
  flavors flavors[] NOT NULL DEFAULT ARRAY[]::flavors[],
  usage usage[] NOT NULL DEFAULT ARRAY[]::usage[],
  strain text NULL,
  classification classification NULL,
  unit unit NOT NULL,
  moq int NOT NULL DEFAULT 1,
  price_cents int NOT NULL,
  compare_at_price_cents int NULL,
  is_active boolean NOT NULL DEFAULT true,
  is_featured boolean NOT NULL DEFAULT false,
  thc_percent numeric(5,2) NULL,
  cbd_percent numeric(5,2) NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT products_store_fk FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
  CONSTRAINT products_moq_positive CHECK (moq >= 1),
  CONSTRAINT products_price_non_negative CHECK (price_cents >= 0),
  CONSTRAINT products_thc_percent_range CHECK (
    thc_percent IS NULL OR (thc_percent >= 0 AND thc_percent <= 100)
  ),
  CONSTRAINT products_cbd_percent_range CHECK (
    cbd_percent IS NULL OR (cbd_percent >= 0 AND cbd_percent <= 100)
  )
);

CREATE INDEX IF NOT EXISTS idx_products_store_is_active ON products (store_id, is_active);
CREATE INDEX IF NOT EXISTS idx_products_category ON products (category);
CREATE INDEX IF NOT EXISTS idx_products_price ON products (price_cents);
CREATE INDEX IF NOT EXISTS idx_products_title ON products (title);

CREATE TABLE IF NOT EXISTS product_media (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id uuid NOT NULL,
  url text NULL,
  gcs_key text NOT NULL,
  position int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT product_media_product_fk FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
  CONSTRAINT product_media_position_non_negative CHECK (position >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_media_product_position
  ON product_media (product_id, position);

-- +goose StatementEnd
