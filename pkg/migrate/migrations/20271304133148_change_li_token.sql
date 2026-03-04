-- +goose Up
-- Convert order_line_items.ad_token from text[] → text

ALTER TABLE order_line_items
ALTER COLUMN ad_token DROP NOT NULL;

ALTER TABLE order_line_items
ALTER COLUMN ad_token TYPE text
USING (
    CASE
        WHEN ad_token IS NULL OR array_length(ad_token, 1) = 0 THEN NULL
        ELSE ad_token[1]
    END
);

-- +goose Down
-- Revert order_line_items.ad_token from text → text[]

ALTER TABLE order_line_items
ALTER COLUMN ad_token TYPE text[]
USING (
    CASE
        WHEN ad_token IS NULL THEN ARRAY[]::text[]
        ELSE ARRAY[ad_token]
    END
);