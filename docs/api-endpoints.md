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
