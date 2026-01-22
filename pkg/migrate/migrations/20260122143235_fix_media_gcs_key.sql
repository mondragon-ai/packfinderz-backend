-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
  -- Only run if the table exists.
  IF EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'media'
  ) THEN

    -- If old typo column exists and correct column does not, rename it.
    IF EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public' AND table_name = 'media' AND column_name = 'gsc_key'
    )
    AND NOT EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public' AND table_name = 'media' AND column_name = 'gcs_key'
    ) THEN
      ALTER TABLE public.media RENAME COLUMN gsc_key TO gcs_key;
    END IF;

    -- Rename unique constraint if it exists (optional cleanup).
    IF EXISTS (
      SELECT 1
      FROM pg_constraint
      WHERE conname = 'media_gsc_key_key'
        AND conrelid = 'public.media'::regclass
    )
    AND NOT EXISTS (
      SELECT 1
      FROM pg_constraint
      WHERE conname = 'media_gcs_key_key'
        AND conrelid = 'public.media'::regclass
    ) THEN
      ALTER TABLE public.media RENAME CONSTRAINT media_gsc_key_key TO media_gcs_key_key;
    END IF;

  END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DO $$
BEGIN
  -- Only run if the table exists.
  IF EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'media'
  ) THEN

    -- If correct column exists and typo column does not, rename it back.
    IF EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public' AND table_name = 'media' AND column_name = 'gcs_key'
    )
    AND NOT EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public' AND table_name = 'media' AND column_name = 'gsc_key'
    ) THEN
      ALTER TABLE public.media RENAME COLUMN gcs_key TO gsc_key;
    END IF;

    -- Rename constraint back if present.
    IF EXISTS (
      SELECT 1
      FROM pg_constraint
      WHERE conname = 'media_gcs_key_key'
        AND conrelid = 'public.media'::regclass
    )
    AND NOT EXISTS (
      SELECT 1
      FROM pg_constraint
      WHERE conname = 'media_gsc_key_key'
        AND conrelid = 'public.media'::regclass
    ) THEN
      ALTER TABLE public.media RENAME CONSTRAINT media_gcs_key_key TO media_gsc_key_key;
    END IF;

  END IF;
END $$;

-- +goose StatementEnd

