# API Documentation

All responses use the JSON envelope:

```json
{
  "data":  <payload>,
  "error": "<message or null>",
  "meta":  { "page": 1, "total": 100 }
}
```

---

## Media

### `GET /api/media`
List media items.

| Query param | Type    | Description                               |
|-------------|---------|-------------------------------------------|
| `page`      | int     | Page number (default: 1)                  |
| `limit`     | int     | Items per page (default: 50, max: 200)    |
| `type`      | string  | Filter by type (video/image/manga/comic/doujinshi) |
| `q`         | string  | Title search                              |
| `favorite`  | boolean | Filter favorites                          |

### `GET /api/media/:id`
Get a single media item including its tags.

### `PATCH /api/media/:id`
Update mutable fields.

```json
{
  "title":      "New Title",
  "rating":     4,
  "favorite":   true,
  "language":   "japanese",
  "source_url": "https://example.com"
}
```

### `DELETE /api/media/:id`
Soft-delete a media item.

---

## Tags

### `GET /api/tags`
List all tags. Optional `?category=artist`.

### `GET /api/tags/search?q=blon`
Autocomplete – returns up to 20 matching tags.

### `POST /api/tags`
```json
{ "name": "blonde hair", "category": "general" }
```

### `PATCH /api/tags/:id`
```json
{ "name": "blond hair", "category": "general" }
```

### `DELETE /api/tags/:id`
Remove tag and its associations.

---

## Downloads

### `POST /api/downloads`
Enqueue a new download.

```json
{
  "url":              "https://example.com/gallery/123",
  "target_directory": "/downloads/example",
  "auto_tags":        ["artist:foo"]
}
```

### `POST /api/downloads/batch`
Enqueue multiple URLs at once.

```json
{
  "urls": ["https://…", "https://…"],
  "target_directory": "/downloads/batch"
}
```

### `GET /api/downloads`
List all download jobs. Optional `?status=queued`.

### `PATCH /api/downloads/:id`
Control or update a job.

```json
{ "action": "pause" }    // pause | resume | cancel | retry
```

### `DELETE /api/downloads/:id`
Remove a download job.

---

## Schedules

### `GET /api/schedules`
List all scheduled downloads.

### `POST /api/schedules`
```json
{
  "name":             "Daily gallery update",
  "url_pattern":      "https://example.com/artist/xyz",
  "cron_expression":  "0 3 * * *",
  "target_directory": "/downloads/scheduled"
}
```

### `PATCH /api/schedules/:id`
```json
{ "enabled": false }
```

### `DELETE /api/schedules/:id`

---

## Library

### `POST /api/library/scan`
Trigger an immediate filesystem scan. Returns `202 Accepted`.

---

## Health

### `GET /healthz`
Returns `{"status":"ok"}`.
