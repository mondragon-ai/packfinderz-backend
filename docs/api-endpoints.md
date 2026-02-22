# API Endpoints

## Notifications

All `/api/v1/notifications` routes require a valid `Authorization: Bearer <access_token>` header because the `/api` group is guarded by `middleware.Auth`. The active store `store_id` is inferred from the token, so requests without a store-scoped access token will be rejected with HTTP 403. Every `POST` route under this prefix is also protected by `middleware.Idempotency`, so include a unique `Idempotency-Key` header even if the request has no body.

### `GET /api/v1/notifications`

Returns a paginated `ListResult` of notifications scoped to the active store. Supported query parameters:

- `limit` – optional positive integer; defaults to 25 and caps at 100 (`pagination.NormalizeLimit`).
- `cursor` – optional encoded value used for cursor-based pagination ordered by `(created_at, id) DESC`.
- `unreadOnly` – optional boolean to filter only unread notifications (`true`) or drop the filter (`false`/absent).

```bash
curl -G "{{API_BASE_URL}}/api/v1/notifications" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  --data-urlencode "limit=25" \
  --data-urlencode "cursor={{NEXT_CURSOR}}" \
  --data-urlencode "unreadOnly=true"
```

### `POST /api/v1/notifications/{notificationId}/read`

Marks a single notification as read for the active store. `notificationId` must be a valid UUID; cross-store values return HTTP 403/404. A successful call responds with `{"read": true}`.

```bash
curl -X POST "{{API_BASE_URL}}/api/v1/notifications/{{notification_id}}/read" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  -H "Idempotency-Key: {{UNIQUE_KEY}}"
```

### `POST /api/v1/notifications/read-all`

Flags every unread notification for the active store as read and responds with `{"updated": <number>}` indicating how many rows were touched.

```bash
curl -X POST "{{API_BASE_URL}}/api/v1/notifications/read-all" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  -H "Idempotency-Key: {{UNIQUE_KEY}}"
```

## Wishlist

All `/api/v1/wishlist` routes require the `Authorization: Bearer {{ACCESS_TOKEN}}` header and run inside the `/api` group guarded by `middleware.StoreContext`. The active store ID is inferred from the token, so missing or invalid store claims return HTTP 403/401 before your controller runs.

### `GET /api/v1/wishlist`

Reads the buyer store’s wishlist and returns a cursor page of `WishlistItemDTO` rows (`product` + `created_at`). Supports cursor pagination with the same parameters as the product browse response:

- `limit` – optional positive integer; defaults to 25 and caps at 100 via `pagination.NormalizeLimit`.
- `cursor` – optional base64 cursor representing `(created_at, id)` for bookmark-style paging.

```bash
curl -G "{{API_BASE_URL}}/api/v1/wishlist" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  --data-urlencode "limit=25" \
  --data-urlencode "cursor={{NEXT_CURSOR}}"
```

Response DTO example:

```json
{
  "data": {
    "items": [
      {
        "product": {
          "id": "uuid",
          "sku": "...",
          "title": "...",
          "subtitle": "...",
          "category": "...",
          "classification": "...",
          "unit": "...",
          "price_cents": 1000,
          "compare_at_price_cents": null,
          "thc_percent": null,
          "cbd_percent": null,
          "has_promo": false,
          "vendor_store_id": "uuid",
          "created_at": "2025-01-01T00:00:00Z",
          "updated_at": "...",
          "max_qty": 5,
          "thumbnail_url": null
        },
        "created_at": "2025-01-01T01:23:45Z"
      }
    ],
    "pagination": {
      "page": 1,
      "total": 42,
      "current": "{{REQUEST_CURSOR}}",
      "first": "{{FIRST_CURSOR}}",
      "last": "{{LAST_CURSOR}}",
      "prev": "{{REQUEST_CURSOR}}",
      "next": "{{NEXT_CURSOR}}"
    }
  }
}
```

### `GET /api/v1/wishlist/ids`

Returns the product UUIDs a buyer store likes. The endpoint now supports the same cursor pagination inputs as the browse page (`limit` + `cursor`), and the response includes identical `pagination` metadata.

```bash
curl -G "{{API_BASE_URL}}/api/v1/wishlist/ids" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  --data-urlencode "limit=25" \
  --data-urlencode "cursor={{NEXT_CURSOR}}"
```

Response body:

```json
{
  "data": {
    "product_ids": [
      "uuid-1",
      "uuid-2"
    ],
    "pagination": {
      "page": 1,
      "total": 42,
      "current": "{{REQUEST_CURSOR}}",
      "first": "{{FIRST_CURSOR}}",
      "last": "{{LAST_CURSOR}}",
      "prev": "{{REQUEST_CURSOR}}",
      "next": "{{NEXT_CURSOR}}"
    }
  }
}
```

## Stores

Routes under `/api/v1/stores` require the standard `/api` auth + store context guards. `middleware.StoreContext` extracts the buyer or vendor store ID from the token, so only requests backed by an `activeStoreId` can reach these handlers.

### `GET /api/v1/stores/me`

Returns the active store’s profile. The response matches `stores.StoreDTO`, exposing company info, contact channels, curated badge status, address, ratings, and timestamps such as `last_logged_in_at`.

```bash
curl "{{API_BASE_URL}}/api/v1/stores/me" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}"
```

Response example:

```json
{
  "data": {
    "id": "store-uuid",
    "type": "buyer",
    "company_name": "Acme Groceries",
    "description": "Locally owned",
    "phone": "+15555551234",
    "email": "care@acme.local",
    "kyc_status": "verified",
    "subscription_active": true,
    "delivery_radius_meters": 10000,
    "address": {
      "line1": "123 Market Street",
      "city": "Testville",
      "state": "CA",
      "postal_code": "94103",
      "country": "US"
    },
    "social": null,
    "banner_url": null,
    "logo_url": null,
    "ratings": {
      "service": 5
    },
    "categories": [
      "groceries",
      "cannabis"
    ],
    "owner": "owner-uuid",
    "badge": "quality_verified",
    "last_logged_in_at": "2024-04-01T12:34:56Z",
    "created_at": "...",
    "updated_at": "..."
  }
}
```

### `PUT /api/v1/stores/me`

Allows updating writable store fields. All attributes are optional—send only the values that should change. Valid fields match `storeUpdateRequest` structures and include:

- `company_name` (min 1 char, display/company name)
- `description`, `phone`, `email` (contact info; email must be valid)
- `social` object per `pkg/types.Social` (keys: `twitter`, `facebook`, `instagram`, `linkedin`, `youtube`, `website`; nullable strings to clear)
- `banner_url`, `logo_url` (public URLs that override the persisted attachments)
- `banner_media_id`, `logo_media_id` (nullable UUID for pre-uploaded assets; send `null` to remove)
- `ratings` map (string keys → ints; send `{}` to clear ratings)
- `categories` array (string tags; send `[]` to clear)
- `badge` (`store_badge` enum; provide a known value to apply a curated badge)

Example request showing every writable field:

```json
{
  "company_name": "Acme Goods",
  "description": "Farm-to-table grocer & delivery hub in the Mission.",
  "phone": "+15551234567",
  "email": "hello@acme.gro",
  "social": {
    "twitter": "https://twitter.com/acmegoods",
    "facebook": "https://facebook.com/acmegoods",
    "instagram": "https://instagram.com/acmegoods",
    "linkedin": "https://linkedin.com/company/acme",
    "youtube": "https://youtube.com/@acmegoods",
    "website": "https://acme.goods"
  },
  "banner_url": "https://cdn.packfinderz.com/banners/acme-store.jpg",
  "logo_url": "https://cdn.packfinderz.com/logos/acme.png",
  "badge": "quality_verified",
  "banner_media_id": "2e3f5a00-7cb3-4b41-8f14-1f2abc3d4e5f",
  "logo_media_id": null,
  "ratings": {
    "service": 4,
    "selection": 5,
    "delivery": 3
  },
  "categories": [
    "groceries",
    "delivery",
    "produce"
  ]
}
```

```bash
curl -X PUT "{{API_BASE_URL}}/api/v1/stores/me" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  -H "Content-Type: application/json" \
  -d '{
    "company_name": "Acme Goods",
    "description": "Farm-to-table grocer & delivery hub in the Mission.",
    "phone": "+15551234567",
    "email": "hello@acme.gro",
    "social": {
      "twitter": "https://twitter.com/acmegoods",
      "facebook": "https://facebook.com/acmegoods",
      "instagram": "https://instagram.com/acmegoods",
      "linkedin": "https://linkedin.com/company/acme",
      "youtube": "https://youtube.com/@acmegoods",
      "website": "https://acme.goods"
    },
    "badge": "quality_verified",
    "banner_media_id": "2e3f5a00-7cb3-4b41-8f14-1f2abc3d4e5f",
    "logo_media_id": null,
    "categories": [
      "groceries",
      "delivery",
      "produce"
    ]
  }'
```

Response uses the same `StoreDTO` as `GET /stores/me`.

### `GET /api/v1/stores/me/users`

Returns the active store’s membership roster (`memberships.StoreUserDTO`). Owners/managers may filter (server-side) by role/status; the handler simply returns whatever the service provides.

```bash
curl "{{API_BASE_URL}}/api/v1/stores/me/users" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}"
```

Response example:

```json
{
  "data": [
    {
      "membership_id": "membership-uuid",
      "store_id": "store-uuid",
      "user_id": "user-uuid",
      "email": "owner@example.com",
      "first_name": "Owner",
      "last_name": "One",
      "role": "owner",
      "membership_status": "active",
      "created_at": "...",
      "last_login_at": "..."
    }
  ]
}
```

### `POST /api/v1/stores/me/users/invite`

Invites a new user with a store role. Owners/managers must include an `Idempotency-Key` header to avoid duplicate invites. The body follows `storeInviteRequest` and requires:

- `email` (required, normalized to lowercase)
- `first_name` (required)
- `last_name` (required)
- `role` (required, one of `owner`, `admin`, `manager`, `viewer`, `agent`, `staff`, `ops`)

Example request with every required field:

```json
{
  "email": "invitee@example.com",
  "first_name": "New",
  "last_name": "User",
  "role": "manager"
}
```

```bash
curl -X POST "{{API_BASE_URL}}/api/v1/stores/me/users/invite" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: {{IDEMPOTENCY_KEY}}" \
  -d '{
    "email": "invitee@example.com",
    "first_name": "New",
    "last_name": "User",
    "role": "manager"
  }'
```

Response includes the created `memberships.StoreUserDTO` and the temporary password when generated:

```json
{
  "data": {
    "user": {
      "membership_id": "...",
      "store_id": "...",
      "user_id": "...",
      "email": "invitee@example.com",
      "first_name": "New",
      "last_name": "User",
      "role": "manager",
      "membership_status": "invited",
      "created_at": "...",
      "last_login_at": null
    },
    "temporary_password": "temp1234"
  }
}
```

### `DELETE /api/v1/stores/me/users/{userId}`

Removes a membership by UUID. The path parameter must be a valid UUID; missing or invalid IDs return HTTP 422/400.

```bash
curl -X DELETE "{{API_BASE_URL}}/api/v1/stores/me/users/{{USER_ID}}" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}"
```

Response body for success: `{"data":null}`.

### `POST /api/v1/wishlist/items`

Adds a product to the wishlist. Idempotency is handled at the DB level (`ON CONFLICT DO NOTHING`), so repeat calls return success even when the row already exists.

Request DTO:

```json
{
  "product_id": "uuid"
}
```

```bash
curl -X POST "{{API_BASE_URL}}/api/v1/wishlist/items" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  -H "Content-Type: application/json" \
  -d '{"product_id":"{{PRODUCT_ID}}"}'
```

Response body:

```json
{
  "data": {
    "added": true
  }
}
```

### `DELETE /api/v1/wishlist/items/{productId}`

Removes the liked product for the store. Missing rows still return success (`{"removed": true}`).

```bash
curl -X DELETE "{{API_BASE_URL}}/api/v1/wishlist/items/{{PRODUCT_ID}}" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}"
```

Response body:

```json
{
  "data": {
    "removed": true
  }
}
```

### `GET /api/v1/products`

Browses products that match the requesting store context. Requires `/api` auth + store context (`middleware.StoreContext`). Buyer stores must supply `state` (matching their address) while vendor stores omit it. Supported query parameters:

- `limit` (default `20`)
- `page` (default `1`)
- `cursor` (opaque string from prior responses for pagination)
- `state` (`CA`, `OR`, etc.; required for buyers)
- `category`, `classification` (`enums.ProductCategory`, `enums.ProductClassification`)
- `price_min_cents`, `price_max_cents`
- `thc_min`, `thc_max`, `cbd_min`, `cbd_max`
- `has_promo` (`true`/`false`)
- `q` for a title/SKU search term

```bash
curl "{{API_BASE_URL}}/api/v1/products?state=CA&limit=25&page=1&category=flower&price_min_cents=1000&has_promo=true&q=indica" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}"
```

Response mirrors `product.ProductListResult`:

```json
{
  "data": {
    "products": [
      {
        "id": "product-uuid-1",
        "sku": "FLOWER-001",
        "title": "Blue Dream Flower",
        "subtitle": "Sativa-dominant classic",
        "category": "flower",
        "classification": null,
        "unit": "gram",
        "moq": 1,
        "price_cents": 1800,
        "compare_at_price_cents": 2200,
        "thc_percent": 18,
        "cbd_percent": 0.4,
        "has_promo": true,
        "coa_added": true,
        "vendor_store_id": "vendor-store-uuid",
        "created_at": "...",
        "updated_at": "...",
        "max_qty": 10,
        "thumbnail_url": "https://cdn.packfinderz.com/products/flower-001-thumb.jpg"
      },
      {
        "id": "product-uuid-2",
        "sku": "EDIBLE-001",
        "title": "Sour Gummies",
        "category": "edibles",
        "unit": "pack",
        "moq": 1,
        "price_cents": 1800,
        "has_promo": false,
        "coa_added": false,
        "vendor_store_id": "vendor-store-uuid",
        "created_at": "...",
        "updated_at": "...",
        "max_qty": 5,
        "thumbnail_url": null
      }
    ],
    "pagination": {
      "page": 1,
      "total": 2,
      "current": "{{CURSOR_THIS_PAGE}}",
      "next": "{{NEXT_CURSOR}}"
    }
  }
}
```

### `GET /api/v1/products/{productId}`

Returns the full `product.ProductDTO` for the requested product, including inventory, media gallery, volume discounts, COA info, and vendor summary. Requires `/api` auth + store context that owns or has access to the product.

```bash
curl "{{API_BASE_URL}}/api/v1/products/{{PRODUCT_ID}}" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}"
```

Response example:

```json
{
  "data": {
    "id": "product-uuid-1",
    "sku": "FLOWER-001",
    "title": "Blue Dream Flower",
    "subtitle": "Sativa-dominant classic",
    "body_html": "<p>Premium batch from the coast.</p>",
    "category": "flower",
    "feelings": ["Relaxed","Creative"],
    "flavors": ["Citrus","Berry"],
    "usage": ["Day","Creative"],
    "classification": "Sativa",
    "unit": "gram",
    "moq": 1,
    "price_cents": 1800,
    "compare_at_price_cents": 2200,
    "is_active": true,
    "is_featured": false,
    "thc_percent": 18.2,
    "cbd_percent": 0.4,
    "inventory": {
      "available_qty": 24,
      "reserved_qty": 2,
      "low_stock_threshold": 5,
      "updated_at": "..."
    },
    "volume_discounts": [
      {"id": "vd-1", "min_qty": 3, "discount_percent": 5, "created_at": "..."}
    ],
    "media": [
      {"id": "media-1", "position": 0, "url": "https://cdn...", "gcs_key": "product/flower-1.jpg", "created_at": "..."}
    ],
    "coa_media_id": "coa-uuid",
    "coa_read_url": "https://signed-url",
    "vendor": {
      "store_id": "vendor-store-uuid",
      "company_name": "Coastal Cultivars",
      "logo_media_id": "logo-uuid",
      "logo_gcs_key": "logos/coastal-logo.png"
    },
    "max_qty": 10,
    "created_at": "...",
    "updated_at": "..."
  }
}
```

# Auth (store switching)

Switching stores relies on the scoped JWT sent with the request (`Authorization: Bearer {{ACCESS_TOKEN}}`). The handler extracts `store_id` from the body, reuses the access token’s `jti`/refresh mapping, and returns a fresh access token via the `X-PF-Token` header plus a `refresh_token` value in the JSON payload. The body only requires the new store ID, there is no refresh token input because the JWT already identifies the session.

### `POST /api/v1/auth/switch-store`

```bash
curl -X POST "{{API_BASE_URL}}/api/v1/auth/switch-store" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  -H "Content-Type: application/json" \
  -d '{"store_id":"{{NEW_STORE_ID}}"}'
```

Successful calls update the `X-PF-Token` response header with the new access token and return JSON like `{"data":{"store_id":"...","store_name":"...","store_type":"vendor","refresh_token":"..."}}`.



---                            
