## Extensions
- `pgcrypto` + `postgis` enable `gen_random_uuid()` and geography columns before any tables are created (pkg/migrate/migrations/20260120003410_enable_extensions.sql:3-9).

## Enums & composites
- `store_type`: `buyer|vendor` for the `stores.type` enum (pkg/migrate/migrations/20260120003412_create_stores_table.sql:1-42; pkg/enums/store.go:5-41).
- `kyc_status`: `pending_verification|verified|rejected|expired|suspended` for store lifecycle (pkg/migrate/migrations/20260120003412_create_stores_table.sql:1-42; pkg/enums/store.go:43-85).
- `address_t`: composite `(line1,line2,city,state,postal_code,country,lat,lng,geohash)` backed by `types.Address` `Value/Scan` helpers (pkg/migrate/migrations/20260120003412_create_stores_table.sql:12-32; pkg/types/address.go:10-109).
- `social_t`: composite `(twitter,facebook,instagram,linkedin,youtube,website)` reflected in `types.Social` (pkg/migrate/migrations/20260120003412_create_stores_table.sql:25-36; pkg/types/social.go:9-58).
- `member_role`: owner|admin|manager|viewer|agent|staff|ops for memberships (pkg/migrate/migrations/20260120003413_create_store_memberships.sql:1-33; pkg/enums/member_role.go:5-50).
- `membership_status`: invited|active|removed|pending for memberships (pkg/migrate/migrations/20260120003413_create_store_memberships.sql:34-56; pkg/enums/membership_status.go:5-44).
- `media_kind`: product|ads|pdf|license_doc|coa|manifest|user|other for `media.kind` (pkg/migrate/migrations/20260120003415_create_media.sql:1-34; pkg/enums/media_kind.go:5-52).
- `media_status`: states `pending`→`uploaded|processing|ready|failed|delete_requested|deleted|delete_failed` stored in `Media` rows (pkg/db/models/media.go:11-32; pkg/enums/media_status.go:5-52).
- `license_status`: pending|verified|rejected|expired (pkg/migrate/migrations/20260122192426_create_license_table.sql:1-34; pkg/enums/license.go:5-85).
- `license_type`: producer|grower|dispensary|merchant (pkg/migrate/migrations/20260122192426_create_license_table.sql:1-34; pkg/enums/license.go:47-87).
- `event_type_enum`: enumerates domain events (order/line item/license/media/payment/cash/vendor notification/reservation/ad) used by `outbox_events` (pkg/migrate/migrations/20260123000001_create_outbox_events.sql:1-38; pkg/enums/outbox.go:16-69).
- `aggregate_type_enum`: vendor_order|checkout_group|license|store|media|ledger_event|notification|ad for the aggregate_id context (pkg/migrate/migrations/20260123000001_create_outbox_events.sql:39-77; pkg/enums/outbox.go:5-41).
- `geography(Point,4326)`: stored in `stores.geom` and materialized via `types.GeographyPoint` `Value/Scan` (pkg/migrate/migrations/20260120003412_create_stores_table.sql:18-36; pkg/types/geography_point.go:12-117).
- `ratings` JSONB uses `types.Ratings` for flexible score maps on stores (pkg/migrate/migrations/20260120003414_add_store_profile_fields.sql:1-8; pkg/types/ratings.go:9-47).

## Tables
### users
- Primary key `id uuid DEFAULT gen_random_uuid()`, unique `email`, password hash, names, optional `phone`, `is_active` default true, `last_login_at`, `system_role`, `store_ids uuid[] DEFAULT ARRAY[]::uuid[]`, timestamps (pkg/migrate/migrations/20260120003411_create_users_table.sql:1-24; pkg/db/models/user.go:9-23).

### stores
- `id`, `type store_type`, `company_name`, optional `dba_name/description/phone/email`, `kyc_status` default `pending_verification`, `subscription_active` bool, `delivery_radius_meters`, `address address_t`, `geom geography(Point,4326)`, optional `social social_t`, `banner_url`, `logo_url`, `ratings jsonb`, `categories text[]`, `owner` FK to `users`, `last_active_at`, timestamps, GIST index on `geom`, indexes on `(type,kyc_status)` and `subscription_active` (pkg/migrate/migrations/20260120003412_create_stores_table.sql:1-42; pkg/migrate/migrations/20260120003414_add_store_profile_fields.sql:1-8; pkg/db/models/store.go:13-35).

### store_memberships
- FK to `stores`/`users`, `role member_role`, `status membership_status`, optional `invited_by_user_id`, `UNIQUE (store_id,user_id)`, indexes on `user_id`, `(store_id,role)`, `(store_id,status)` (pkg/migrate/migrations/20260120003413_create_store_memberships.sql:1-33; pkg/db/models/store_membership.go:11-21).

### media
- `id`, optional `store_id`/`user_id` (FKs), `kind media_kind`, `status media_status DEFAULT 'pending'`, `gcs_key` unique (corrected via `20260122143235`), `file_name`, `mime_type`, `ocr`, `size_bytes`, `is_compressed` bool, timestamps plus `uploaded_at`, `verified_at`, `processing_started_at`, `ready_at`, `failed_at`, `deleted_at`; indexes on `(store_id,created_at DESC)`, `kind`, `user_id` (pkg/migrate/migrations/20260120003415_create_media.sql:1-41; pkg/migrate/migrations/20260122143235_fix_media_gcs_key.sql:1-52; pkg/db/models/media.go:11-32).

### media_attachments
- `id`, `media_id FK`, `owner_type`, `owner_id`, timestamp, unique constraint `(media_id,owner_type,owner_id)` for attachments (pkg/migrate/migrations/20260120003415_create_media.sql:7-24).

### licenses
- `id`, `store_id`, `user_id`, `status license_status DEFAULT 'pending'`, `media_id`, `gcs_key UNIQUE` added later, `issuing_state`, optional `issue_date`/`expiration_date`, `type license_type`, unique `number`, timestamps, indexes on `(store_id,status)` and `expiration_date` (pkg/migrate/migrations/20260122192426_create_license_table.sql:1-34; pkg/migrate/migrations/20260122193650_add_gcs_key_license.sql:1-7; pkg/db/models/license.go:11-26).

### outbox_events
- Append-only stream with `id`, `event_type event_type_enum`, `aggregate_type aggregate_type_enum`, `aggregate_id`, `payload jsonb`, `created_at` default now, nullable `published_at`, `attempt_count` default 0, `last_error` text; indexes on `published_at`, `event_type`, `(aggregate_type,aggregate_id)` (pkg/migrate/migrations/20260123000001_create_outbox_events.sql:1-39; pkg/db/models/outbox_event.go:12-23).
