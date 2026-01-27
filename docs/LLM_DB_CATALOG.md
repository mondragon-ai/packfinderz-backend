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
- `notification_type`: `system_announcement|market_update|security_alert|order_alert|compliance` used by the `notifications` table (pkg/migrate/migrations/20260124000000_create_notifications.sql:1-41; pkg/enums/notification.go:5-41).
- `vendor_order_fulfillment_status`: `pending|partial|fulfilled` describes `vendor_orders.fulfillment_status` so buyers can filter ready/partial/fulfilled states (pkg/migrate/migrations/20260126000001_add_vendor_order_fields.sql:4-11; pkg/enums/vendor_order_fulfillment_status.go:5-42).
- `vendor_order_shipping_status`: `pending|dispatched|in_transit|delivered` tracks the logistics stage on `vendor_orders.shipping_status`, enabling the buyer list to show shipment progress (pkg/migrate/migrations/20260126000001_add_vendor_order_fields.sql:15-23; pkg/enums/vendor_order_shipping_status.go:5-45).
- `geography(Point,4326)`: stored in `stores.geom` and materialized via `types.GeographyPoint` `Value/Scan` (pkg/migrate/migrations/20260120003412_create_stores_table.sql:18-36; pkg/types/geography_point.go:12-117).
- `ratings` JSONB uses `types.Ratings` for flexible score maps on stores (pkg/migrate/migrations/20260120003414_add_store_profile_fields.sql:1-8; pkg/types/ratings.go:9-47).

## Tables
### users
- Primary key `id uuid DEFAULT gen_random_uuid()`, unique `email`, password hash, names, optional `phone`, `is_active` default true, `last_login_at`, `system_role`, `store_ids uuid[] DEFAULT ARRAY[]::uuid[]`, timestamps (pkg/migrate/migrations/20260120003411_create_users_table.sql:1-24; pkg/db/models/user.go:9-23).

### stores
- `id`, `type store_type`, `company_name`, optional `dba_name/description/phone/email`, `kyc_status` default `pending_verification`, `subscription_active` bool, `delivery_radius_meters`, `address address_t`, `geom geography(Point,4326)`, optional `social social_t`, `banner_url`, `logo_url`, `ratings jsonb`, `categories text[]`, `owner` FK to `users`, `last_active_at`, timestamps, GIST index on `geom`, indexes on `(type,kyc_status)` and `subscription_active` (pkg/migrate/migrations/20260120003412_create_stores_table.sql:1-42; pkg/migrate/migrations/20260120003414_add_store_profile_fields.sql:1-8; pkg/db/models/store.go:13-35).
- `kyc_status`, `subscription_active`, and `address.state` serve as the canonical visibility flags: buyer product/list/detail queries call `pkg/visibility.EnsureVendorVisible` which requires `kyc_status=verified`, `subscription_active=true`, and matching `state` before returning any vendor data, yielding `422` or `404` when violated (pkg/visibility/visibility.go:11-46).

### store_memberships
- FK to `stores`/`users`, `role member_role`, `status membership_status`, optional `invited_by_user_id`, `UNIQUE (store_id,user_id)`, indexes on `user_id`, `(store_id,role)`, `(store_id,status)` (pkg/migrate/migrations/20260120003413_create_store_memberships.sql:1-33; pkg/db/models/store_membership.go:11-21).

### media
- `id`, optional `store_id`/`user_id` (FKs), `kind media_kind`, `status media_status DEFAULT 'pending'`, `gcs_key` unique (corrected via `20260122143235`), `file_name`, `mime_type`, `ocr`, `size_bytes`, `is_compressed` bool, timestamps plus `uploaded_at`, `verified_at`, `processing_started_at`, `ready_at`, `failed_at`, `deleted_at`; indexes on `(store_id,created_at DESC)`, `kind`, `user_id` (pkg/migrate/migrations/20260120003415_create_media.sql:1-41; pkg/migrate/migrations/20260122143235_fix_media_gcs_key.sql:1-52; pkg/db/models/media.go:11-32).

### media_attachments
- `id`, `media_id FK`, `owner_type`, `owner_id`, timestamp, unique constraint `(media_id,owner_type,owner_id)` for attachments (pkg/migrate/migrations/20260120003415_create_media.sql:7-24).

### products
- `id uuid`, `store_id store_id FK`, `sku`, `title`, optional `subtitle/body_html`, `category category`, `feelings feelings[]`, `flavors flavors[]`, `usage usage[]`, `strain`, `classification classification`, `unit unit`, `moq`, `price_cents`, optional `compare_at_price_cents`, `is_active bool`, `is_featured bool`, optional `thc_percent`, optional `cbd_percent`, timestamps (DESIGN_DOC.md:2710-2757; pkg/db/models/product.go:9-45; pkg/enums/product.go:5-148).
- Arrays for `feelings`, `flavors`, and `usage` use `text[]` columns to capture multi-select metadata; `category` and `unit` are backed by canonical enums (`pkg/enums/product.go`), ensuring product lookups can rely on consistent values.
- FK: `store_id -> stores(id)` enforces vendor ownership, and GORM relations define `Inventory`, `VolumeDiscounts`, and `Media` preloads for the primary product repo (pkg/db/models/product.go:30-45).

### inventory_items
- `product_id uuid PRIMARY KEY REFERENCES products(id)` stores the 1:1 inventory row, along with `available_qty`, `reserved_qty`, and `updated_at` (DESIGN_DOC.md:2813-2824; pkg/db/models/inventory_item.go:9-24).
- The repository ensures `product_id` is the PK so `UpsertInventory`/`GetInventoryByProductID` always target the single row per product.

### product_volume_discounts
- `id uuid`, `product_id uuid REFERENCES products(id)`, `min_qty`, `unit_price_cents`, `created_at` plus `unique(product_id,min_qty)` and `order by (product_id,min_qty desc)` for tiered pricing lookups (DESIGN_DOC.md:2780-2804; pkg/db/models/product_volume_discount.go:9-24).
- The discount repo keeps the `(product_id,min_qty)` uniqueness and orders results descending by `min_qty` for efficient greatest-eligible-tier retrieval.

### cart_records
- `id`, `buyer_store_id uuid REFERENCES stores(id) ON DELETE CASCADE`, optional `session_id`, `status cart_status NOT NULL DEFAULT 'active'`, optional `shipping_address address_t`, `total_discount`, `fees`, `subtotal_cents`, `total_cents`, optional `cart_level_discount cart_level_discount[]`, `created_at`, `updated_at`, plus indexes on `(buyer_store_id,status)` and `session_id` (pkg/migrate/migrations/20260124000003_create_cart_records.sql:1-41; pkg/db/models/cart_record.go:12-41; pkg/enums/cart_status.go:1-26).
- `cart_status` enum (`active|converted`) governs the buyer-scoped lifecycle and is enforced by `internal/cart.Repository.UpdateStatus` before the record is consumed by checkout.

### cart_items
- `id`, `cart_id uuid REFERENCES cart_records(id) ON DELETE CASCADE`, `product_id uuid REFERENCES products(id) ON DELETE RESTRICT`, `vendor_store_id uuid REFERENCES stores(id) ON DELETE RESTRICT`, `qty`, `product_sku`, `unit unit`, `unit_price_cents`, optional compare-at/tier/discount/subtotal fields, optional `featured_image`, `moq`, `thc_percent numeric(5,2)`, `cbd_percent numeric(5,2)`, timestamps, and indexes on `cart_id` plus `vendor_store_id` for buyer/vendor lookups (pkg/migrate/migrations/20260124000003_create_cart_records.sql:42-79; pkg/db/models/cart_item.go:11-37).
- These rows persist the product/vendor snapshot that checkout uses when the buyer converts the cart, preventing recomputation of pricing/MOQ data at execution time.

### checkout_groups
- Placeholder for the `checkout_groups` table introduced in PF-077; it will reference `cart_records`, mirror buyer context, store aggregated totals, and own linkages to `vendor_orders` once the migrations are in place (implementation pending, see PF-077).

### order_line_items
- Placeholder for the `order_line_items` table introduced in PF-077; it will reference `vendor_orders`, capture product snapshots, quantities, pricing tiers, and inventory references, mirroring the `cart_items` payload (implementation pending, see PF-077).

### payment_intents
- Placeholder for the `payment_intents` table introduced in PF-077; it will track payment status (`cash` default), totals, and vendor split info when checkout executes, aligning with Doc 4’s master enums (implementation pending, see PF-077).

### product_media
- `id uuid`, `product_id uuid REFERENCES products(id)`, optional `url`, `gcs_key`, `position`, and timestamps; `unique(product_id, position)` plus ordered `position ASC` is required for canonical media presentation to buyers (DESIGN_DOC.md:2831-2852; pkg/db/models/product_media.go:11-29).
- Repository preloads `Media` ordered by `position` so services can expose `media[0]` as the primary thumbnail and iteratively display the rest.

### licenses
- `id`, `store_id`, `user_id`, `status license_status DEFAULT 'pending'`, `media_id`, `gcs_key UNIQUE` added later, `issuing_state`, optional `issue_date`/`expiration_date`, `type license_type`, unique `number`, timestamps, indexes on `(store_id,status)` and `expiration_date` (pkg/migrate/migrations/20260122192426_create_license_table.sql:1-34; pkg/migrate/migrations/20260122193650_add_gcs_key_license.sql:1-7; pkg/db/models/license.go:11-26).
- Scheduler logic relies on the `expiration_date` index to find licenses expiring in 14 days and those expiring today; it emits `license_status_changed` events for warnings/expirations and flips `stores.kyc_status` when no valid licenses remain (`internal/schedulers/licenses/service.go`:1-220).

### outbox_events
- Append-only stream with `id`, `event_type event_type_enum`, `aggregate_type aggregate_type_enum`, `aggregate_id`, `payload jsonb`, `created_at` default now, nullable `published_at`, `attempt_count` default 0, `last_error` text; indexes on `published_at`, `event_type`, `(aggregate_type,aggregate_id)` (pkg/migrate/migrations/20260123000001_create_outbox_events.sql:1-39; pkg/db/models/outbox_event.go:12-23).

### notifications
- `id`, `store_id`, `type notification_type`, `title`, `message`, optional `link`, `read_at`, `created_at` default `now()` (pkg/migrate/migrations/20260124000000_create_notifications.sql:1-41; pkg/db/models/notification.go:10-24).
- Indexes on `(store_id,created_at desc)`, `(store_id,read_at)`, and `(created_at)` plus `store_id -> stores(id)` cascade FK (pkg/migrate/migrations/20260124000000_create_notifications.sql:1-41).
- Compliance workflows insert `notification_type=compliance` rows for pending uploads (admin notices) and verified/rejected licences (store notices) when `license_status_changed` events are consumed, keeping a `store_id` anchor and `link` for UI navigation (internal/notifications/consumer.go:128-186).

### vendor_orders
- Per-vendor order snapshot produced after checkout converts a `cart_record` into `checkout_groups`/`vendor_orders`/`order_line_items`/`payment_intents` (pkg/migrate/migrations/20260124000004_create_checkout_order_tables.sql:84-205).
- Fields include `checkout_group_id`, `buyer_store_id`, `vendor_store_id`, `status`, `refund_status`, money totals, `notes`/`internal_notes`, timestamps, and the new `fulfillment_status`, `shipping_status`, and sequential `order_number` populated from `vendor_order_number_seq` so buyers can search by incremental order IDs (pkg/migrate/migrations/20260126000001_add_vendor_order_fields.sql:4-35).
- `delivered_at` captures the moment an assigned agent marked the order delivered (via `internal/orders.Service.AgentDeliver`), and the service enforces an `in_transit` precondition while updating `status`/`shipping_status` to `delivered` so downstream reporting can surface exact handoff times (internal/orders/service.go:724-778).
- Indexes:
  - `(buyer_store_id, created_at DESC)` (idx_vendor_orders_buyer_created, buyer order list), `(vendor_store_id, created_at DESC)` (idx_vendor_orders_vendor_created, vendor order list), and `(status)` (idx_vendor_orders_status, action-state lookups) are defined in the checkout tables migration (pkg/migrate/migrations/20260124000004_create_checkout_order_tables.sql:138-150).
  - `unique(order_number)` (ux_vendor_orders_order_number, sequential buyer reference) is created by the vendor order fields migration (pkg/migrate/migrations/20260126000001_add_vendor_order_fields.sql:29-35).
  - `unique(checkout_group_id, vendor_store_id)` (ux_vendor_orders_group_vendor, one order per vendor per checkout) preserves the original checkout constraint (pkg/migrate/migrations/20260124000004_create_checkout_order_tables.sql:146-150).
- Foreign keys: `checkout_group_id -> checkout_groups(id)`, `buyer_store_id -> stores(id)`, `vendor_store_id -> stores(id)` (all in the same migration block).
- Constraint: `CHECK (buyer_store_id <> vendor_store_id)` to enforce opposing roles on the same order.

### ledger_events
- Append-only ledger rows capturing cash collection, vendor payouts, adjustments, and future refunds; defined by `pkg/migrate/migrations/20260130000000_create_ledger_events_table.sql`, which creates `ledger_event_type_enum`, `ledger_events`, and the `(order_id, created_at)` and `(type, created_at)` indexes while the Goose down block drops the table+enum.
- Fields: `id uuid pk`; `order_id uuid not null`; `type ledger_event_type_enum not null`; `amount_cents int not null`; `metadata jsonb null`; `created_at timestamptz not null default now()` (pkg/db/models/ledger_event.go:9-33; pkg/enums/ledger_event_type.go:7-33; pkg/migrate/migrations/20260130000000_create_ledger_events_table.sql:1-27).
- Indexes: `(order_id, created_at)` (ledger_events_order_created_idx) and `(type, created_at)` (ledger_events_type_created_idx) (pkg/migrate/migrations/20260130000000_create_ledger_events_table.sql:19-27).
- Foreign keys: `order_id -> vendor_orders(id) ON DELETE RESTRICT` (pkg/migrate/migrations/20260130000000_create_ledger_events_table.sql:13-23).
- Append-only enforcement: `internal/ledger.Repository` only exposes `Create` and `ListByOrderID`, and `internal/ledger.Service.RecordEvent` validates the enum before persisting so no application path issues `UPDATE`/`DELETE` against ledger rows (internal/ledger/service.go:22-64; internal/ledger/repo.go:12-38).

### order_assignments
- Tracks agent assignments per vendor order so there is always at most one `active = true` row that `internal/orders.Repository.FindOrderDetail` can read for dashboards (pkg/migrate/migrations/20260128000000_create_order_assignments_table.sql:1-24; internal/orders/repo.go:322-347).
- Fields: `id uuid pk`; `order_id uuid not null`; `agent_user_id uuid not null`; `assigned_by_user_id uuid null`; `assigned_at timestamptz not null default now()`; `unassigned_at timestamptz null`; `active boolean not null default true`.
- Indexes: `(agent_user_id, active)` (idx_order_assignments_agent_active), `(order_id)` (idx_order_assignments_order), `unique(order_id) WHERE active = true` (ux_order_assignments_order_active) (pkg/migrate/migrations/20260128000000_create_order_assignments_table.sql:7-20).
- Foreign keys: `order_id -> vendor_orders(id) ON DELETE CASCADE`; `agent_user_id -> users(id) ON DELETE RESTRICT`; `assigned_by_user_id -> users(id) ON DELETE SET NULL`.
- Meta columns: migration `20260129000000_add_order_assignment_meta.sql` adds `pickup_time`, `delivery_time`, `cash_pickup_time`, `pickup_signature_gcs_key`, and `delivery_signature_gcs_key` so assignment records can log pickup/delivery timestamps and optional signature artifacts before future proofing payment capture traces; the down script drops these columns when rolling back (pkg/migrate/migrations/20260129000000_add_order_assignment_meta.sql).
- Reversibility: the Goose down section drops the indexes and table so rolling back removes `order_assignments` cleanly (pkg/migrate/migrations/20260128000000_create_order_assignments_table.sql:26-29).
