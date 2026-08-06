# API Documentation

**Base URL:** `http://localhost:3000`

**Version:** v1

**Content-Type:** `application/json`

---

## Response Envelope

All endpoints return JSON with a standard envelope:

### Success Response

```json
{
  "status": "success",
  "data": {},
  "message": "..."
}
```

### Error Response

```json
{
  "status": "error",
  "message": "...",
  "trace_id": "...",
  "errors": {}
}
```

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `"success"` or `"error"` |
| `data` | object/array | Payload (only on success) |
| `message` | string | Result description |
| `trace_id` | string | Unique request ID (only on error) |
| `errors` | object | Additional details (optional) |

---

## Session Management

All LMS endpoints require `npm` + `password` in the JSON body. Sessions are cached in memory:

- **First request** with a given NPM → launches Chrome, logs in, caches session
- **Subsequent requests** with same NPM → reuses cached session (no Chrome launch, no login)
- **After 15 min idle** → session evicted, next request creates fresh session
- **Max 10 sessions** → LRU eviction when limit reached
- **Concurrent requests** with same NPM → safe via per-NPM mutex

Sessions are cleaned up every 1 minute by a background goroutine. A session may remain cached up to 16 minutes (15 min TTL + 1 min cleanup interval).

---

## Endpoints

### 1. Health Check

Check server health status.

```
GET /api/v1/health
```

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "status": "ok",
    "service": "lonceng_unman_be",
    "version": "development"
  },
  "message": "Service is healthy"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `data.status` | string | `"ok"` |
| `data.service` | string | Application name from `APP_NAME` env |
| `data.version` | string | Environment from `APP_ENV` env |

**Example curl:**

```bash
curl http://localhost:3000/api/v1/health
```

---

### 2. LMS Login

Validate LMS credentials and cache the session for subsequent requests.

```
POST /api/v1/lms/login
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `password` | string | Yes | LMS password |

**Response 200 OK — Login Success:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "Login successful",
    "npm": "2211700006",
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "Login successful"
}
```

**Response 200 OK — Login Failed (Invalid Credentials):**

```json
{
  "status": "success",
  "data": {
    "success": false,
    "message": "login failed: Maaf, username/password yang anda masukkan tidak valid.",
    "npm": "2211700006",
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "login failed: Maaf, username/password yang anda masukkan tidak valid."
}
```

> **Note:** Login failure returns HTTP 200 with `data.success: false`.
> Error message comes from the LMS page (already sanitized).

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Password:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Invalid Body:**

```json
{
  "status": "error",
  "message": "invalid request body: ...",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error — Browser Failure:**

```json
{
  "status": "error",
  "message": "login operation failed",
  "trace_id": "abc123..."
}
```

> **Note:** Internal error details are logged server-side, not exposed to client.
> Possible causes: Chrome not installed, browser timeout, or LMS down.

**Example curl:**

```bash
# Login success
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'

# Login failed (wrong credentials)
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"wrong","password":"wrong"}'

# Missing NPM
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"password":"test"}'
```

---

### 3. Download KRS

Download KRS (Kartu Rencana Studi) PDF for a specific student.
Navigates to the KRS page, extracts semester number, then downloads the PDF.

```
POST /api/v1/lms/krs
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `password` | string | Yes | LMS password |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KRS downloaded successfully",
    "npm": "2211700006",
    "file_path": "downloads/2211700006/krs/semester_8.pdf",
    "size": 12345,
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "KRS downloaded successfully"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `data.success` | bool | Download status |
| `data.message` | string | Result description |
| `data.npm` | string | Student NPM |
| `data.file_path` | string | Saved file path (canonical) |
| `data.size` | int | File size in bytes |
| `data.timestamp` | string | Download time (ISO 8601) |

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Invalid NPM:**

```json
{
  "status": "error",
  "message": "npm must contain only digits",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Password:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error — Download Failed:**

```json
{
  "status": "error",
  "message": "KRS download failed",
  "trace_id": "abc123..."
}
```

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/krs \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'
```

**Saved File Location:**

```
downloads/2211700006/krs/semester_8.pdf
```

> **Note:** KRS filename uses `semester_{N}.pdf` where N is the student's current semester number.
> The semester number is extracted from the KRS page.
> File is overwritten if it already exists (latest download wins).

---

### 4. Get KHS Semesters

Get list of available KHS (Kartu Hasil Studi) semesters for a specific student.

```
POST /api/v1/lms/khs/semesters
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `password` | string | Yes | LMS password |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "npm": "2211700006",
    "semesters": [
      {
        "tahun_ajaran": "2022/2023",
        "semester": "GANJIL",
        "sks": 20
      },
      {
        "tahun_ajaran": "2022/2023",
        "semester": "GENAP",
        "sks": 22
      },
      {
        "tahun_ajaran": "2023/2024",
        "semester": "GANJIL",
        "sks": 21
      }
    ],
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "KHS semesters retrieved"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `data.npm` | string | Student NPM |
| `data.semesters` | array | Available semesters |
| `data.semesters[].tahun_ajaran` | string | Academic year (format: `YYYY/YYYY`) |
| `data.semesters[].semester` | string | Semester (`GANJIL` or `GENAP`) |
| `data.semesters[].sks` | int | Total SKS for that semester |
| `data.timestamp` | string | Retrieval time (ISO 8601) |

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Password:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error:**

```json
{
  "status": "error",
  "message": "fetch KHS semesters failed",
  "trace_id": "abc123..."
}
```

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/khs/semesters \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'
```

---

### 5. Download KHS

Download KHS (Kartu Hasil Studi) PDF for a specific semester.

```
POST /api/v1/lms/khs
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027",
  "tahun_ajaran": "2022/2023",
  "semester": "GANJIL"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `password` | string | Yes | LMS password |
| `tahun_ajaran` | string | Yes | Academic year (format: `YYYY/YYYY`, e.g. `2022/2023`) |
| `semester` | string | Yes | Semester (`GANJIL` or `GENAP`, case-sensitive) |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KHS downloaded successfully",
    "npm": "2211700006",
    "tahun_ajaran": "2022/2023",
    "semester": "GANJIL",
    "file_path": "downloads/2211700006/khs/2022_2023_GANJIL.pdf",
    "size": 15678,
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "KHS downloaded successfully"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `data.success` | bool | Download status |
| `data.message` | string | Result description |
| `data.npm` | string | Student NPM |
| `data.tahun_ajaran` | string | Academic year |
| `data.semester` | string | Semester |
| `data.file_path` | string | Saved file path (canonical) |
| `data.size` | int | File size in bytes |
| `data.timestamp` | string | Download time (ISO 8601) |

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Password:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Tahun Ajaran:**

```json
{
  "status": "error",
  "message": "tahun_ajaran is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Semester:**

```json
{
  "status": "error",
  "message": "semester is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Invalid NPM:**

```json
{
  "status": "error",
  "message": "npm must contain only digits",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Invalid Semester:**

```json
{
  "status": "error",
  "message": "semester must be GANJIL or GENAP",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error — Download Failed:**

```json
{
  "status": "error",
  "message": "KHS download failed",
  "trace_id": "abc123..."
}
```

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/khs \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027","tahun_ajaran":"2022/2023","semester":"GANJIL"}'
```

**Saved File Location:**

```
downloads/2211700006/khs/2022_2023_GANJIL.pdf
```

> **Note:** KHS filename uses `{tahun_ajaran}_{semester}.pdf`.
> `tahun_ajaran` with `/` is replaced with `_` (e.g. `2022/2023` → `2022_2023`).
> File is overwritten if it already exists (latest download wins).

---

## Download Folder Structure

All PDFs are stored under `{DOWNLOAD_DIR}/{NPM}/` with canonical filenames:

```
downloads/
├── {NPM}/
│   ├── krs/
│   │   └── semester_{N}.pdf           # KRS (N = student's current semester)
│   └── khs/
│       ├── 2022_2023_GANJIL.pdf        # KHS per semester
│       ├── 2022_2023_GENAP.pdf
│       └── 2023_2024_GANJIL.pdf
```

**Naming Rules:**
- KRS: `{downloadDir}/{npm}/krs/semester_{N}.pdf` (N = semester number from KRS page)
- KHS: `{downloadDir}/{npm}/khs/{tahun_ajaran}_{semester}.pdf`
- `tahun_ajaran`: `/` replaced with `_` (e.g. `2022/2023` → `2022_2023`)
- Files are overwritten if they already exist (latest download wins)

---

## Request Flow

```
Client Request (npm + password)
  │
  ▼
Handler: validate JSON body (npm, password, ...)
  │
  ▼
Service: SessionManager.GetOrCreate(npm, password)
  │
  ├── Session exists & valid? ──▶ Reuse cached browser session
  │
  └── No session? ──▶ Launch Chrome → Login → Cache session → Return BrowserSession
  │
  ▼
Service: use BrowserSession (navigate, eval, download)
  │
  ▼
Response envelope → JSON output
```

---

## PDF Extraction Endpoints

### 6. Extract KRS

Extract structured data from downloaded KRS PDF. Always re-extracts and overwrites existing cache.

```
POST /api/v1/lms/krs/extract
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `password` | string | Yes | LMS password |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KRS extracted successfully",
    "npm": "2211700006",
    "file_path": "extracted/2211700006/krs/semester_8.json",
    "timestamp": "2026-08-06T12:45:00+07:00"
  },
  "message": "KRS extracted successfully"
}
```

> **Note:** This endpoint always re-extracts and overwrites existing cache.
> Use POST `/api/v1/lms/krs/data` with `{"npm":"..."}` to retrieve cached data.

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Password:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 401 Unauthorized — LMS Login Failed:**

```json
{
  "status": "error",
  "message": "LMS login failed: ...",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 401 when LMS credentials are invalid.

**Response 404 Not Found — PDF Not Found:**

```json
{
  "status": "error",
  "message": "KRS PDF not found for npm: 2211700006",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 404 when no KRS PDF file exists for the given NPM.
> Possible causes: PDF was never downloaded or file was deleted.

**Response 500 Internal Server Error — Extraction Failed:**

```json
{
  "status": "error",
  "message": "KRS extraction failed",
  "trace_id": "abc123..."
}
```

> **Note:** Internal error details are logged server-side, not exposed to client.
> Possible causes: PDF corrupted, PDF empty, PDF too large (>50MB), or parse error.

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/krs/extract \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'
```

---

### 7. Extract KHS

Extract structured data from downloaded KHS PDF. Always re-extracts and overwrites existing cache.

```
POST /api/v1/lms/khs/extract
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "password": "izzan027",
  "tahun_ajaran": "2022/2023",
  "semester": "GENAP"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `password` | string | Yes | LMS password |
| `tahun_ajaran` | string | Yes | Academic year (e.g. `2022/2023`) |
| `semester` | string | Yes | Semester (`GANJIL` or `GENAP`) |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KHS extracted successfully",
    "npm": "2211700006",
    "file_path": "extracted/2211700006/khs/2022_2023_GENAP.json",
    "timestamp": "2026-08-06T12:45:00+07:00"
  },
  "message": "KHS extracted successfully"
}
```

> **Note:** This endpoint always re-extracts and overwrites existing cache.
> Use POST `/api/v1/lms/khs/data` with `{"npm":"...","tahun_ajaran":"...","semester":"..."}` to retrieve cached data.

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Password:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Tahun Ajaran:**

```json
{
  "status": "error",
  "message": "tahun_ajaran is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Semester:**

```json
{
  "status": "error",
  "message": "semester is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 401 Unauthorized — LMS Login Failed:**

```json
{
  "status": "error",
  "message": "LMS login failed: ...",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 401 when LMS credentials are invalid.

**Response 404 Not Found — PDF Not Found:**

```json
{
  "status": "error",
  "message": "KHS PDF not found",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 404 when no KHS PDF file exists for the given NPM/tahun_ajaran/semester.
> Possible causes: PDF was never downloaded or file was deleted.

**Response 500 Internal Server Error — Extraction Failed:**

```json
{
  "status": "error",
  "message": "KHS extraction failed",
  "trace_id": "abc123..."
}
```

> **Note:** Internal error details are logged server-side, not exposed to client.
> Possible causes: PDF corrupted, PDF empty, PDF too large (>50MB), or parse error.

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/khs/extract \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027","tahun_ajaran":"2022/2023","semester":"GENAP"}'
```

---

### 8. Get KRS Data

Get cached KRS extraction data.

```
POST /api/v1/lms/krs/data
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "krs": {
      "mahasiswa": {
        "nama": "MOCHAMAD IZZAN FIRASYANSYAH",
        "npm": "2211700006",
        "program_studi": "Sistem Informasi"
      },
      "periode": {
        "tahun_ajaran": "2025/2026",
        "semester": "GENAP"
      },
      "mata_kuliah": [...],
      "total_sks": 12
    },
    "metadata": {
      "extracted_at": "2026-08-06T12:45:00+07:00",
      "source_file": "downloads/2211700006/krs/semester_8.pdf",
      "file_size": 183701
    }
  },
  "message": "KRS data retrieved"
}
```

**Response 400 Bad Request — Invalid Body:**

```json
{
  "status": "error",
  "message": "invalid request body",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 404 Not Found — Extraction Data Not Found:**

```json
{
  "status": "error",
  "message": "extraction data not found",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 404 when no cached extraction exists for the given NPM.
> Call POST `/api/v1/lms/krs/extract` first to create extraction data.

**Response 403 Forbidden — Permission Denied:**

```json
{
  "status": "error",
  "message": "permission denied accessing KRS extraction",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 403 when file permission is denied.

**Response 500 Internal Server Error — Read Failed:**

```json
{
  "status": "error",
  "message": "failed to retrieve KRS extraction",
  "trace_id": "abc123..."
}
```

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/krs/data \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006"}'
```

---

### 9. Get KHS Data

Get cached KHS extraction data.

```
POST /api/v1/lms/khs/data
Content-Type: application/json
```

**Request Body:**

```json
{
  "npm": "2211700006",
  "tahun_ajaran": "2022/2023",
  "semester": "GENAP"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npm` | string | Yes | NPM (digits only, 8-12 characters) |
| `tahun_ajaran` | string | Yes | Academic year (e.g. `2022/2023`) |
| `semester` | string | Yes | Semester (`GANJIL` or `GENAP`) |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "khs": {
      "mahasiswa": {
        "nama": "MOCHAMAD IZZAN FIRASYANSYAH",
        "npm": "2211700006",
        "program_studi": "Sistem Informasi"
      },
      "periode": {
        "tahun_ajaran": "2022/2023",
        "semester": "GENAP"
      },
      "mata_kuliah": [...],
      "rekapitulasi": {
        "total_sks": 23,
        "total_mutu": 84,
        "ipk": 3.65
      }
    },
    "metadata": {
      "extracted_at": "2026-08-06T12:45:00+07:00",
      "source_file": "downloads/2211700006/khs/2022_2023_GENAP.pdf",
      "file_size": 193331
    }
  },
  "message": "KHS data retrieved"
}
```

**Response 400 Bad Request — Invalid Body:**

```json
{
  "status": "error",
  "message": "invalid request body",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Missing Parameters:**

```json
{
  "status": "error",
  "message": "tahun_ajaran and semester are required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Length:**

```json
{
  "status": "error",
  "message": "npm must be 8-12 characters",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Invalid Semester:**

```json
{
  "status": "error",
  "message": "semester must be GANJIL or GENAP",
  "trace_id": "abc123..."
}
```

**Response 404 Not Found — Extraction Data Not Found:**

```json
{
  "status": "error",
  "message": "extraction data not found",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 404 when no cached extraction exists for the given NPM/tahun_ajaran/semester.
> Call POST `/api/v1/lms/khs/extract` first to create extraction data.

**Response 403 Forbidden — Permission Denied:**

```json
{
  "status": "error",
  "message": "permission denied accessing KHS extraction",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 403 when file permission is denied.

**Response 500 Internal Server Error — Read Failed:**

```json
{
  "status": "error",
  "message": "failed to retrieve KHS extraction",
  "trace_id": "abc123..."
}
```

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/khs/data \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","tahun_ajaran":"2022/2023","semester":"GENAP"}'
```

---

## Error Handling

|HTTP Status|Constructor|When|
|---|---|---|
|400|`apperror.BadRequest(msg)`|Invalid request (body parse error, missing fields)|
|401|`apperror.Unauthorized(msg)`|Authentication failure (LMS login failed)|
|403|`apperror.Forbidden(msg)`|Permission denied (extraction access denied)|
|404|`apperror.NotFound(msg, err)`|Resource not found (PDF missing, extraction data not found)|
|500|`apperror.Internal(msg, err)`|Infrastructure failure (browser crash, page load error, PDF corrupted)|

### Extraction-Specific Errors

| HTTP Status | Scenario | Message Pattern |
|-------------|----------|------------------|
| 404 | KRS PDF not found | `KRS PDF not found for npm: {npm}` |
| 404 | KHS PDF not found | `KHS PDF not found` |
| 404 | Extraction data not found | `extraction data not found` |
| 500 | PDF corrupted | `pdf file may be corrupted` |
| 500 | PDF empty | `pdf contains no extractable text` |
| 500 | PDF too large | `pdf too large: {size} bytes` |
| 500 | Parse error | `pdf parsing failed: {detail}` |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_NAME` | `lonceng_unman_be` | Application name |
| `APP_ENV` | `development` | Environment: development, staging, production |
| `APP_PORT` | `3000` | Server port (1-65535) |
| `APP_HOST` | `0.0.0.0` | Bind address |
| `LMS_BASE_URL` | `https://elearning.universitasmandiri.ac.id` | LMS base URL |
| `LMS_DASHBOARD_URL` | `https://elearning.universitasmandiri.ac.id/admin/` | Dashboard URL after login |
| `BROWSER_HEADLESS` | `true` | Run Chrome headless |
| `BROWSER_TIMEOUT` | `30s` | Overall browser operation timeout |
| `ACTION_TIMEOUT` | `10s` | Per-action timeout (click, fill, etc.) |
| `DOWNLOAD_DIR` | `./downloads` | Download directory |
| `SESSION_TTL` | `15m` | Session cache duration before expiry |
| `MAX_SESSIONS` | `10` | Maximum cached sessions in memory |
| `EXTRACT_DIR` | `./extracted` | Directory for extracted JSON files |
| `MAX_BODY_SIZE` | `1MB` | Max HTTP request body size |
| `CORS_ALLOW_ORIGINS` | `*` | CORS allowed origins |
| `CORS_ALLOW_METHODS` | `GET,POST,PUT,PATCH,DELETE,OPTIONS` | CORS allowed methods |
| `CORS_ALLOW_HEADERS` | `Content-Type,Authorization` | CORS allowed headers |

---

## Prerequisites

- Go 1.26.4+
- Chrome/Chromium installed (for LMS login)
- go-rod v0.116.2 (auto-installed via `go mod tidy`)
- razvandimescu/gopdf (auto-installed via `go mod tidy`)
