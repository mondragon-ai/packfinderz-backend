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
