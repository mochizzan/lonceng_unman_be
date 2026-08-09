# Get Student Profile Photo

Download the student's profile photo from the LMS dashboard. The photo is scraped from the `<a href="ktm_take_foto.php">` widget, cached on disk for 15 minutes, and returned as binary JPEG.

**Endpoint:** `POST /api/v1/lms/student-profile/photo`

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027"
}
```

| Field | JSON Key | Type | Required | Example | Description |
|-------|----------|------|----------|---------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student ID. Digits only, 8-12 chars |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password |

**Response 200 OK (photo found):**

Returns raw binary image data (not JSON). Headers indicate content type and cache duration.

```
HTTP/1.1 200 OK
Content-Type: image/jpeg
Content-Length: 2578256
Cache-Control: max-age=900

<binary JPEG data>
```

| Header | Value | Description |
|--------|-------|-------------|
| `Content-Type` | `image/jpeg` | Photo MIME type |
| `Content-Length` | `2578256` | Photo size in bytes (e.g. ~2.5 MB) |
| `Cache-Control` | `max-age=900` | Client-side cache hint (15 minutes) |

**Behavior:**

1. First request: logs into LMS, navigates to dashboard, scrapes photo URL, downloads image. Response time ~8-14s.
2. Subsequent requests (within 15 min): serves from disk cache. Response time ~0.2s.
3. After TTL expires: re-scrapes from LMS on next request.

**Response 204 No Content (photo not found):**

Returned when the LMS dashboard does not contain a profile photo element (e.g. `<a href="ktm_take_foto.php">` not found or no `<img>` child).

```
HTTP/1.1 204 No Content
```

**Response 400 Bad Request:**

```json
{
  "status": "error",
  "message": "npm is required"
}
```

```json
{
  "status": "error",
  "message": "password is required"
}
```

**Response 500 Internal Server Error:**

Returned when LMS login fails or browser automation encounters an error.

```json
{
  "status": "error",
  "message": "failed to get student photo"
}
```

**Cache Structure:**

Photos are cached in the per-NPM download directory:

```
downloads/
└── {NPM}/
    └── photo/
        ├── {NPM}.jpg      # Photo binary (JPEG)
        └── {NPM}.json     # Cache metadata (TTL, timestamps)
```

Cache metadata example (`2211700006.json`):

```json
{
  "npm": "2211700006",
  "cached_at": "2026-08-10T14:30:00+07:00",
  "expires_at": "2026-08-10T14:45:00+07:00",
  "original_filename": "20240326204129_2211700006_PXL_20221007_054228932 (1).jpg"
}
```

**Example curl:**

```bash
# Download photo and save to file
curl -X POST http://localhost:3000/api/v1/lms/student-profile/photo \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}' \
  -o photo.jpg

# Check response headers only
curl -X POST http://localhost:3000/api/v1/lms/student-profile/photo \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}' \
  -I
```

**Notes:**

- The photo is scraped from the LMS dashboard (`/admin/`), not the student profile form page.
- The `<img>` element is located inside `<div class="small-box bg-yellow">` wrapped by `<a href="ktm_take_foto.php">`.
- The original filename from the LMS includes a timestamp prefix: `{timestamp}_{NPM}_{original_photo_name}`.
- Response times: ~8-14s (first scrape), ~0.2s (cache hit).
- The `Cache-Control` header (15 min) allows clients to skip redundant requests.
