# API Documentation

**Base URL:** `http://localhost:3000`
**Version:** v1
**Content-Type:** `application/json`

## Endpoints

| # | Method | Path | Description | Doc |
|---|--------|------|-------------|-----|
| 1 | GET | /api/v1/health | Health check | [01_HEALTH.md](01_HEALTH.md) |
| 2 | POST | /api/v1/lms/login | LMS login | [02_LMS_LOGIN.md](02_LMS_LOGIN.md) |
| 3 | POST | /api/v1/lms/krs | Download KRS PDF | [03_KRS_DOWNLOAD.md](03_KRS_DOWNLOAD.md) |
| 4 | POST | /api/v1/lms/khs/semesters | Get KHS semesters | [04_KHS_SEMESTERS.md](04_KHS_SEMESTERS.md) |
| 5 | POST | /api/v1/lms/khs | Download KHS PDF | [05_KHS_DOWNLOAD.md](05_KHS_DOWNLOAD.md) |
| 6 | POST | /api/v1/extraction/krs | Extract KRS | [06_EXTRACT_KRS.md](06_EXTRACT_KRS.md) |
| 7 | POST | /api/v1/extraction/khs | Extract KHS | [07_EXTRACT_KHS.md](07_EXTRACT_KHS.md) |
| 8 | POST | /api/v1/data/krs | Get KRS data | [08_GET_KRS_DATA.md](08_GET_KRS_DATA.md) |
| 9 | POST | /api/v1/data/khs | Get KHS data | [09_GET_KHS_DATA.md](09_GET_KHS_DATA.md) |
| 10 | POST | /api/v1/lms/student-profile | Scrape student profile | [10_STUDENT_PROFILE_SCRAPE.md](10_STUDENT_PROFILE_SCRAPE.md) |
| 11 | POST | /api/v1/lms/student-profile/data | Get student profile | [11_STUDENT_PROFILE_GET.md](11_STUDENT_PROFILE_GET.md) |
| 12 | POST | /api/v1/lms/student-profile/photo | Get student profile photo | [12_STUDENT_PROFILE_PHOTO.md](12_STUDENT_PROFILE_PHOTO.md) |

## Quick Links

- [Response Envelope](#response-envelope)
- [Session Management](#session-management)
- [Data Types](DATA_TYPES.md)
- [Folder Structure](FOLDER_STRUCTURE.md)
- [Error Handling](ERROR_HANDLING.md)
- [Environment Variables](ENVIRONMENT.md)

## Response Envelope

### Success

```json
{
  "status": "success",
  "data": {},
  "message": "..."
}
```

### Error

```json
{
  "status": "error",
  "message": "...",
  "trace_id": "..."
}
```

## Session Management

All LMS endpoints require `npm` + `password`. Sessions are cached in memory:

| Behavior | Detail |
|----------|--------|
| **First request** with a given NPM | Launches headless Chrome, logs in to LMS, caches the browser session |
| **Subsequent requests** with same NPM | Reuses cached session |
| **After 15 min idle** | Session evicted |
| **Max 10 sessions** | LRU eviction when limit is reached |
| **Concurrent requests** with same NPM | Safe via per-NPM mutex lock |
| **Cleanup interval** | Background goroutine runs every 1 minute |

---

**See also:** [Data Types](DATA_TYPES.md) | [Folder Structure](FOLDER_STRUCTURE.md) | [Error Handling](ERROR_HANDLING.md) | [Environment Variables](ENVIRONMENT.md)
