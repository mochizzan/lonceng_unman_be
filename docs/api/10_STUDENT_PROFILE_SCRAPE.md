# Scrape Student Profile

Scrape student profile data from LMS HTML form. Navigates to profile page, reads all form fields via bulk JavaScript evaluation, and caches the result as JSON.

**Endpoint:** `POST /api/v1/lms/student-profile`

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

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "npm": "2211700006",
    "message": "Profile scraped successfully",
    "cached_at": "2026-08-09T16:30:39+07:00"
  },
  "message": "Student profile downloaded"
}
```

| JSON Path | Type | Description |
|-----------|------|-------------|
| `data.npm` | `string` | Student NPM |
| `data.message` | `string` | Scrape result description |
| `data.cached_at` | `string` (ISO 8601) | When the profile was scraped and cached |

**Response 400 Bad Request:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error:**

```json
{
  "status": "error",
  "message": "failed to scrape student profile",
  "trace_id": "abc123..."
}
```

> **Note:** Possible causes: Chrome not installed, LMS unreachable, login failed, page load timeout.

**Cached File:** `extracted/{npm}/profile/student_profile.json`

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/student-profile \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'
```
