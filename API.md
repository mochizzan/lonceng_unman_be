# API Documentation

**Base URL:** `http://localhost:3000`

**Version:** v1

**Content-Type:** `application/json`

---

## Table of Contents

1. [Response Envelope](#response-envelope)
2. [Session Management](#session-management)
3. [Endpoints](#endpoints)
   - [1. Health Check](#1-health-check)
   - [2. LMS Login](#2-lms-login)
   - [3. Download KRS](#3-download-krs)
   - [4. Get KHS Semesters](#4-get-khs-semesters)
   - [5. Download KHS](#5-download-khs)
   - [6. Extract KRS](#6-extract-krs)
   - [7. Extract KHS](#7-extract-khs)
   - [8. Get KRS Data](#8-get-krs-data)
   - [9. Get KHS Data](#9-get-khs-data)
4. [Data Type Reference](#data-type-reference)
5. [Download Folder Structure](#download-folder-structure)
6. [Request Flow](#request-flow)
7. [Error Handling](#error-handling)
8. [Environment Variables](#environment-variables)
9. [Prerequisites](#prerequisites)

---

## Response Envelope

All endpoints return JSON with a standard envelope. The shape differs between success and error responses.

### Success Response

```json
{
  "status": "success",
  "data": {},
  "message": "..."
}
```

| Field | JSON Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Status | `status` | `string` | Always | Always `"success"` |
| Data | `data` | `object \| array \| null` | On success | Response payload. Omitted (`null`) when not applicable |
| Message | `message` | `string` | Always | Human-readable result description |

### Error Response

```json
{
  "status": "error",
  "message": "...",
  "trace_id": "...",
  "errors": {}
}
```

| Field | JSON Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Status | `status` | `string` | Always | Always `"error"` |
| Message | `message` | `string` | Always | Human-readable error description (sanitized, never exposes internals) |
| Trace ID | `trace_id` | `string` | Always | Unique request identifier from the `X-Request-ID` header (generated per request) |
| Errors | `errors` | `any` | Optional | Additional error details. Omitted when not applicable |

> **Note:** The `data` and `errors` fields are mutually exclusive. Success responses include `data`; error responses include `trace_id` and optionally `errors`.

---

## Session Management

All LMS endpoints (except health check) require `npm` + `password` in the JSON body. Sessions are cached in memory:

| Behavior | Detail |
|----------|--------|
| **First request** with a given NPM | Launches headless Chrome, logs in to LMS, caches the browser session |
| **Subsequent requests** with same NPM | Reuses cached session (no Chrome launch, no login) |
| **After 15 min idle** | Session evicted, next request creates a fresh session |
| **Max 10 sessions** | LRU eviction when limit is reached |
| **Concurrent requests** with same NPM | Safe via per-NPM mutex lock |
| **Cleanup interval** | Background goroutine runs every 1 minute to evict expired sessions |
| **Maximum session lifetime** | Up to 16 minutes (15 min TTL + 1 min cleanup interval) |

---

## Endpoints

---

### 1. Health Check

Check server health status.

```
GET /api/v1/health
```

**Request Body:** None

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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Status | `status` | `string` | `"ok"` | Always `"ok"` when healthy |
| Service | `data.service` | `string` | `"lonceng_unman_be"` | Application name from `APP_NAME` env var |
| Version | `data.version` | `string` | `"development"` | Environment from `APP_ENV` env var |

**Example curl:**

```bash
curl http://localhost:3000/api/v1/health
```

---

### 2. LMS Login

Validate LMS credentials and cache the session for subsequent requests. On success, the browser session is kept alive for reuse.

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password |

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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Success | `data.success` | `boolean` | `true` | Whether the login attempt succeeded |
| Message | `data.message` | `string` | `"Login successful"` | Login result description from the server |
| NPM | `data.npm` | `string` | `"2211700006"` | The NPM that was used for login |
| Timestamp | `data.timestamp` | `string` (ISO 8601) | `"2026-08-06T00:45:00+07:00"` | When the login attempt occurred (server timezone) |

> **Note:** Both success and login failure return HTTP 200. Differentiate by checking `data.success`.

**Response 200 OK — Login Failed (Invalid Credentials):**

```json
{
  "status": "success",
  "data": {
    "success": false,
    "message": "Username atau password salah",
    "npm": "2211700006",
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "Username atau password salah"
}
```

> **Note:** Login failure returns HTTP 200 with `data.success: false`. The message `"Username atau password salah"` is returned when LMS credentials are invalid.

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — NPM Non-Digits:**

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

**Response 500 Internal Server Error — Browser Failure:**

```json
{
  "status": "error",
  "message": "login operation failed",
  "trace_id": "abc123..."
}
```

> **Note:** Internal error details are logged server-side only, never exposed to the client.
> Possible causes: Chrome/Chromium not installed, browser timeout, LMS unreachable.

**Example curl:**

```bash
# Login success
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'

# Login failed (wrong credentials)
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"wrong"}'

# Missing NPM
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"password":"test"}'
```

---

### 3. Download KRS

Download KRS (Kartu Rencana Studi) PDF for a specific student. Navigates to the KRS page, extracts the current semester number, then downloads the PDF.

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password |

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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Success | `data.success` | `boolean` | `true` | Whether the download succeeded |
| Message | `data.message` | `string` | `"KRS downloaded successfully"` | Download result description |
| NPM | `data.npm` | `string` | `"2211700006"` | Student NPM |
| File Path | `data.file_path` | `string` | `"downloads/2211700006/krs/semester_8.pdf"` | Canonical saved file path (relative to project root) |
| Size | `data.size` | `integer` | `12345` | File size in bytes |
| Timestamp | `data.timestamp` | `string` (ISO 8601) | `"2026-08-06T00:45:00+07:00"` | Download time (server timezone) |

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — NPM Non-Digits:**

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

> **Note:** Possible causes: LMS page load error, PDF generation failed, or network timeout.

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

> **Note:** KRS filename uses `semester_{N}.pdf` where N is the student's current semester number extracted from the KRS page. File is overwritten if it already exists (latest download wins).

---

### 4. Get KHS Semesters

Get the list of available KHS (Kartu Hasil Studi) semesters for a specific student. Navigates to the KHS page and scrapes the semester list.

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KHS semesters retrieved",
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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Success | `data.success` | `boolean` | `true` | Whether the retrieval succeeded |
| Message | `data.message` | `string` | `"KHS semesters retrieved"` | Result description |
| NPM | `data.npm` | `string` | `"2211700006"` | Student NPM |
| Semesters | `data.semesters` | `array<KHSSemester>` | *(see below)* | Array of available KHS semesters |
| Semesters[i]. Tahun Ajaran | `data.semesters[i].tahun_ajaran` | `string` | `"2022/2023"` | Academic year range (format: `YYYY/YYYY`) |
| Semesters[i]. Semester | `data.semesters[i].semester` | `string` | `"GANJIL"` | Semester name: `"GANJIL"` (odd) or `"GENAP"` (even) |
| Semesters[i]. SKS | `data.semesters[i].sks` | `integer` | `20` | Total SKS (credits) taken in that semester |
| Timestamp | `data.timestamp` | `string` (ISO 8601) | `"2026-08-06T00:45:00+07:00"` | Retrieval time (server timezone) |

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — NPM Non-Digits:**

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

**Response 500 Internal Server Error — Fetch Failed:**

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

Download KHS (Kartu Hasil Studi) PDF for a specific student and semester.

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password |
| Tahun Ajaran | `tahun_ajaran` | `string` | Yes | `"2022/2023"` | Academic year (format: `YYYY/YYYY`, case-sensitive) |
| Semester | `semester` | `string` | Yes | `"GANJIL"` | Semester: `"GANJIL"` (odd) or `"GENAP"` (even). Case-sensitive |

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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Success | `data.success` | `boolean` | `true` | Whether the download succeeded |
| Message | `data.message` | `string` | `"KHS downloaded successfully"` | Download result description |
| NPM | `data.npm` | `string` | `"2211700006"` | Student NPM |
| Tahun Ajaran | `data.tahun_ajaran` | `string` | `"2022/2023"` | Academic year |
| Semester | `data.semester` | `string` | `"GANJIL"` | Semester |
| File Path | `data.file_path` | `string` | `"downloads/2211700006/khs/2022_2023_GANJIL.pdf"` | Canonical saved file path (relative to project root) |
| Size | `data.size` | `integer` | `15678` | File size in bytes |
| Timestamp | `data.timestamp` | `string` (ISO 8601) | `"2026-08-06T00:45:00+07:00"` | Download time (server timezone) |

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — NPM Non-Digits:**

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

> **Note:** KHS filename format: `{tahun_ajaran}_{semester}.pdf` where `/` in `tahun_ajaran` is replaced with `_` (e.g. `2022/2023` → `2022_2023`). File is overwritten if it already exists (latest download wins).

---

## PDF Extraction Endpoints

The extraction endpoints parse downloaded PDF files and convert them to structured JSON. The JSON is cached on disk and can be retrieved without re-parsing.

---

### 6. Extract KRS

Extract structured data from a downloaded KRS PDF. Always re-extracts and overwrites existing cache. Requires a valid LMS session (verifies credentials first).

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password (used to verify session before extraction) |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KRS extracted successfully",
    "npm": "2211700006",
    "timestamp": "2026-08-06T12:45:00+07:00"
  },
  "message": "KRS extracted successfully"
}
```

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Success | `data.success` | `boolean` | `true` | Whether extraction succeeded |
| Message | `data.message` | `string` | `"KRS extracted successfully"` | Extraction result description |
| NPM | `data.npm` | `string` | `"2211700006"` | Student NPM |
| Timestamp | `data.timestamp` | `string` (ISO 8601) | `"2026-08-06T12:45:00+07:00"` | Extraction time (server timezone) |

> **Note:** This endpoint always re-extracts and overwrites existing cache.
> Use POST `/api/v1/lms/krs/data` to retrieve cached extraction data without re-parsing.

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — NPM Non-Digits:**

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

**Response 401 Unauthorized — LMS Login Failed:**

```json
{
  "status": "error",
  "message": "LMS login failed: ...",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 401 when LMS credentials are invalid. The session is not cached.

**Response 404 Not Found — PDF Not Found:**

```json
{
  "status": "error",
  "message": "KRS PDF not found for npm: 2211700006",
  "trace_id": "abc123..."
}
```

> **Note:** Returns 404 when no KRS PDF file exists for the given NPM.
> Possible causes: PDF was never downloaded, or file was deleted.

**Response 500 Internal Server Error — Extraction Failed:**

```json
{
  "status": "error",
  "message": "KRS extraction failed",
  "trace_id": "abc123..."
}
```

> **Note:** Possible causes: PDF corrupted, PDF empty, PDF too large (>50MB), or parse error.

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/krs/extract \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'
```

---

### 7. Extract KHS

Extract structured data from a downloaded KHS PDF. Always re-extracts and overwrites existing cache. Requires a valid LMS session (verifies credentials first).

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Password | `password` | `string` | Yes | `"izzan027"` | LMS password (used to verify session before extraction) |
| Tahun Ajaran | `tahun_ajaran` | `string` | Yes | `"2022/2023"` | Academic year (format: `YYYY/YYYY`) |
| Semester | `semester` | `string` | Yes | `"GENAP"` | Semester: `"GANJIL"` or `"GENAP"` (case-insensitive, auto-uppercased) |

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KHS extracted successfully",
    "npm": "2211700006",
    "timestamp": "2026-08-06T12:45:00+07:00"
  },
  "message": "KHS extracted successfully"
}
```

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Success | `data.success` | `boolean` | `true` | Whether extraction succeeded |
| Message | `data.message` | `string` | `"KHS extracted successfully"` | Extraction result description |
| NPM | `data.npm` | `string` | `"2211700006"` | Student NPM |
| Timestamp | `data.timestamp` | `string` (ISO 8601) | `"2026-08-06T12:45:00+07:00"` | Extraction time (server timezone) |

> **Note:** This endpoint always re-extracts and overwrites existing cache.
> Use POST `/api/v1/lms/khs/data` to retrieve cached extraction data without re-parsing.

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 500 Internal Server Error — Extraction Failed:**

```json
{
  "status": "error",
  "message": "KHS extraction failed",
  "trace_id": "abc123..."
}
```

**Example curl:**

```bash
curl -X POST http://localhost:3000/api/v1/lms/khs/extract \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027","tahun_ajaran":"2022/2023","semester":"GENAP"}'
```

---

### 8. Get KRS Data

Retrieve cached KRS extraction data. Returns the full structured KRS data as previously extracted.

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |

**Response 200 OK — Full Response:**

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
        "tahun_ajaran": {
          "awal": "2025",
          "akhir": "2026"
        },
        "semester": "GENAP"
      },
      "mata_kuliah": [
        {
          "no": 1,
          "kode": "SI401",
          "nama": "Sistem Informasi Manajemen",
          "sks": 3,
          "kelas": "A",
          "dosen": "Dr. Budi Santoso, M.Kom.",
          "jadwal": {
            "hari": "Senin",
            "waktu_mulai": "08:00",
            "waktu_selesai": "10:30"
          }
        },
        {
          "no": 2,
          "kode": "SI402",
          "nama": "Keamanan Informasi",
          "sks": 3,
          "kelas": "A",
          "dosen": "Dr. Ani Wijayanti, M.T.",
          "jadwal": {
            "hari": "Selasa",
            "waktu_mulai": "10:30",
            "waktu_selesai": "13:00"
          }
        }
      ],
      "total_sks": 12,
      "penerbitan": {
        "tempat": "Yogyakarta",
        "tanggal": "2026-01-15"
      },
      "persetujuan": {
        "mahasiswa": {
          "nama": "MOCHAMAD IZZAN FIRASYANSYAH"
        },
        "ketua_program_studi": {
          "jabatan": "Ketua Program Studi",
          "nama": "Dr. Eko Prasetyo, M.Kom.",
          "nidn": "0425088901"
        }
      }
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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| **KRS** | `data.krs` | `KRSExtraction.KRS` | *(see below)* | The full KRS extraction data |
| Mahasiswa | `data.krs.mahasiswa` | `Mahasiswa` | *(see below)* | Student identity information |
| Mahasiswa. Nama | `data.krs.mahasiswa.nama` | `string` | `"MOCHAMAD IZZAN FIRASYANSYAH"` | Student full name (uppercase as on LMS) |
| Mahasiswa. NPM | `data.krs.mahasiswa.npm` | `string` | `"2211700006"` | Student NPM |
| Mahasiswa. Program Studi | `data.krs.mahasiswa.program_studi` | `string` | `"Sistem Informasi"` | Study program name |
| Periode | `data.krs.periode` | `Periode` | *(see below)* | Academic period information |
| Periode. Tahun Ajaran | `data.krs.periode.tahun_ajaran` | `TahunAjaran` | *(see below)* | Academic year range |
| Periode. Tahun Ajaran. Awal | `data.krs.periode.tahun_ajaran.awal` | `string` | `"2025"` | Start year of academic period |
| Periode. Tahun Ajaran. Akhir | `data.krs.periode.tahun_ajaran.akhir` | `string` | `"2026"` | End year of academic period |
| Periode. Semester | `data.krs.periode.semester` | `string` | `"GENAP"` | `"GANJIL"` (odd) or `"GENAP"` (even) |
| Mata Kuliah | `data.krs.mata_kuliah` | `array<KRSMataKuliah>` | *(see below)* | Array of course entries |
| Mata Kuliah[i]. No | `data.krs.mata_kuliah[i].no` | `integer` | `1` | Course sequence number |
| Mata Kuliah[i]. Kode | `data.krs.mata_kuliah[i].kode` | `string` | `"SI401"` | Course code |
| Mata Kuliah[i]. Nama | `data.krs.mata_kuliah[i].nama` | `string` | `"Sistem Informasi Manajemen"` | Course name |
| Mata Kuliah[i]. SKS | `data.krs.mata_kuliah[i].sks` | `integer` | `3` | Credit units (SKS) |
| Mata Kuliah[i]. Kelas | `data.krs.mata_kuliah[i].kelas` | `string` | `"A"` | Class section |
| Mata Kuliah[i]. Dosen | `data.krs.mata_kuliah[i].dosen` | `string` | `"Dr. Budi Santoso, M.Kom."` | Lecturer name with title |
| Mata Kuliah[i]. Jadwal | `data.krs.mata_kuliah[i].jadwal` | `KRSJadwal` | *(see below)* | Class schedule |
| Mata Kuliah[i]. Jadwal. Hari | `data.krs.mata_kuliah[i].jadwal.hari` | `string` | `"Senin"` | Day of week (Indonesian) |
| Mata Kuliah[i]. Jadwal. Waktu Mulai | `data.krs.mata_kuliah[i].jadwal.waktu_mulai` | `string` | `"08:00"` | Start time (HH:MM format) |
| Mata Kuliah[i]. Jadwal. Waktu Selesai | `data.krs.mata_kuliah[i].jadwal.waktu_selesai` | `string` | `"10:30"` | End time (HH:MM format) |
| Total SKS | `data.krs.total_sks` | `integer` | `12` | Total credit units across all courses |
| Penerbitan | `data.krs.penerbitan` | `Penerbitan` | *(see below)* | Document publication info |
| Penerbitan. Tempat | `data.krs.penerbitan.tempat` | `string` | `"Yogyakarta"` | Publication location |
| Penerbitan. Tanggal | `data.krs.penerbitan.tanggal` | `string` | `"2026-01-15"` | Publication date (YYYY-MM-DD format) |
| Persetujuan | `data.krs.persetujuan` | `KRSPersetujuan` | *(see below)* | Approval section |
| Persetujuan. Mahasiswa. Nama | `data.krs.persetujuan.mahasiswa.nama` | `string` | `"MOCHAMAD IZZAN FIRASYANSYAH"` | Student name in approval section |
| Persetujuan. Ketua Prodi. Jabatan | `data.krs.persetujuan.ketua_program_studi.jabatan` | `string` | `"Ketua Program Studi"` | Position title |
| Persetujuan. Ketua Prodi. Nama | `data.krs.persetujuan.ketua_program_studi.nama` | `string \| null` | `"Dr. Eko Prasetyo, M.Kom."` | Head of study program name. `null` if not available |
| Persetujuan. Ketua Prodi. NIDN | `data.krs.persetujuan.ketua_program_studi.nidn` | `string \| null` | `"0425088901"` | National lecturer ID. `null` if not available |
| **Metadata** | `data.metadata` | `ExtractionMetadata` | *(see below)* | Extraction process metadata |
| Metadata. Extracted At | `data.metadata.extracted_at` | `string` (ISO 8601) | `"2026-08-06T12:45:00+07:00"` | When extraction was performed |
| Metadata. Source File | `data.metadata.source_file` | `string` | `"downloads/2211700006/krs/semester_8.pdf"` | Path to the source PDF file |
| Metadata. File Size | `data.metadata.file_size` | `integer` | `183701` | Source PDF file size in bytes |

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — NPM Non-Digits:**

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

**Response 404 Not Found — Extraction Data Not Found:**

```json
{
  "status": "error",
  "message": "KRS extraction not found for npm: 2211700006",
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

> **Note:** Returns 403 when file system permission is denied.

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

Retrieve cached KHS extraction data. Returns the full structured KHS data as previously extracted.

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

| Field | JSON Key | Type | Required | Example Value | Description |
|-------|----------|------|----------|---------------|-------------|
| NPM | `npm` | `string` | Yes | `"2211700006"` | Student identification number. Digits only, 8-12 characters |
| Tahun Ajaran | `tahun_ajaran` | `string` | Yes | `"2022/2023"` | Academic year (format: `YYYY/YYYY`) |
| Semester | `semester` | `string` | Yes | `"GENAP"` | Semester: `"GANJIL"` or `"GENAP"` (case-insensitive, auto-uppercased) |

**Response 200 OK — Full Response:**

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
        "tahun_ajaran": {
          "awal": "2022",
          "akhir": "2023"
        },
        "semester": "GENAP"
      },
      "mata_kuliah": [
        {
          "no": 1,
          "kode": "SI301",
          "nama": "Basis Data Lanjut",
          "dosen": "Dr. Hendra Wijaya, M.T.",
          "sks": 3,
          "nilai": "A",
          "mutu": 4
        },
        {
          "no": 2,
          "kode": "SI302",
          "nama": "Rekayasa Perangkat Lunak",
          "dosen": "Dr. Siti Nurhaliza, M.Kom.",
          "sks": 3,
          "nilai": "B+",
          "mutu": 3
        },
        {
          "no": 3,
          "kode": "SI303",
          "nama": "Jaringan Komputer",
          "dosen": "Prof. Agus Setiawan, Ph.D.",
          "sks": 3,
          "nilai": "A",
          "mutu": 4
        }
      ],
      "rekapitulasi": {
        "total_sks": 23,
        "total_mutu": 84,
        "ipk": 3.65
      },
      "penerbitan": {
        "tempat": "Yogyakarta",
        "tanggal": "2023-07-20"
      },
      "persetujuan": {
        "dekan": {
          "jabatan": "Dekan Fakultas Ilmu Komputer",
          "nama": "Prof. Dr. Rina Hartati, M.Si.",
          "nidn": "0412066201"
        }
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

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| **KHS** | `data.khs` | `KHSExtraction.KHS` | *(see below)* | The full KHS extraction data |
| Mahasiswa | `data.khs.mahasiswa` | `Mahasiswa` | *(see below)* | Student identity information |
| Mahasiswa. Nama | `data.khs.mahasiswa.nama` | `string` | `"MOCHAMAD IZZAN FIRASYANSYAH"` | Student full name (uppercase as on LMS) |
| Mahasiswa. NPM | `data.khs.mahasiswa.npm` | `string` | `"2211700006"` | Student NPM |
| Mahasiswa. Program Studi | `data.khs.mahasiswa.program_studi` | `string` | `"Sistem Informasi"` | Study program name |
| Periode | `data.khs.periode` | `Periode` | *(see below)* | Academic period information |
| Periode. Tahun Ajaran | `data.khs.periode.tahun_ajaran` | `TahunAjaran` | *(see below)* | Academic year range |
| Periode. Tahun Ajaran. Awal | `data.khs.periode.tahun_ajaran.awal` | `string` | `"2022"` | Start year of academic period |
| Periode. Tahun Ajaran. Akhir | `data.khs.periode.tahun_ajaran.akhir` | `string` | `"2023"` | End year of academic period |
| Periode. Semester | `data.khs.periode.semester` | `string` | `"GENAP"` | `"GANJIL"` (odd) or `"GENAP"` (even) |
| Mata Kuliah | `data.khs.mata_kuliah` | `array<KHSMataKuliah>` | *(see below)* | Array of course entries |
| Mata Kuliah[i]. No | `data.khs.mata_kuliah[i].no` | `integer` | `1` | Course sequence number |
| Mata Kuliah[i]. Kode | `data.khs.mata_kuliah[i].kode` | `string` | `"SI301"` | Course code |
| Mata Kuliah[i]. Nama | `data.khs.mata_kuliah[i].nama` | `string` | `"Basis Data Lanjut"` | Course name |
| Mata Kuliah[i]. Dosen | `data.khs.mata_kuliah[i].dosen` | `string` | `"Dr. Hendra Wijaya, M.T."` | Lecturer name with title |
| Mata Kuliah[i]. SKS | `data.khs.mata_kuliah[i].sks` | `integer` | `3` | Credit units (SKS) |
| Mata Kuliah[i]. Nilai | `data.khs.mata_kuliah[i].nilai` | `string` | `"A"` | Letter grade (`"A"`, `"B+"`, `"B"`, `"C+"`, `"C"`, `"D"`, `"E"`) |
| Mata Kuliah[i]. Mutu | `data.khs.mata_kuliah[i].mutu` | `integer` | `4` | Grade point (0-4 scale) |
| Rekapitulasi | `data.khs.rekapitulasi` | `KHSRekapitulasi` | *(see below)* | Summary statistics |
| Rekapitulasi. Total SKS | `data.khs.rekapitulasi.total_sks` | `integer` | `23` | Total credit units for the semester |
| Rekapitulasi. Total Mutu | `data.khs.rekapitulasi.total_mutu` | `integer` | `84` | Total grade points (mutu) for the semester |
| Rekapitulasi. IPK | `data.khs.rekapitulasi.ipk` | `number` (float64) | `3.65` | Cumulative GPA (Indeks Prestasi Kumulatif) |
| Penerbitan | `data.khs.penerbitan` | `Penerbitan` | *(see below)* | Document publication info |
| Penerbitan. Tempat | `data.khs.penerbitan.tempat` | `string` | `"Yogyakarta"` | Publication location |
| Penerbitan. Tanggal | `data.khs.penerbitan.tanggal` | `string` | `"2023-07-20"` | Publication date (YYYY-MM-DD format) |
| Persetujuan | `data.khs.persetujuan` | `KHSPersetujuan` | *(see below)* | Approval section |
| Persetujuan. Dekan. Jabatan | `data.khs.persetujuan.dekan.jabatan` | `string` | `"Dekan Fakultas Ilmu Komputer"` | Dean position title |
| Persetujuan. Dekan. Nama | `data.khs.persetujuan.dekan.nama` | `string` | `"Prof. Dr. Rina Hartati, M.Si."` | Dean name |
| Persetujuan. Dekan. NIDN | `data.khs.persetujuan.dekan.nidn` | `string` | `"0412066201"` | National lecturer ID |
| **Metadata** | `data.metadata` | `ExtractionMetadata` | *(see below)* | Extraction process metadata |
| Metadata. Extracted At | `data.metadata.extracted_at` | `string` (ISO 8601) | `"2026-08-06T12:45:00+07:00"` | When extraction was performed |
| Metadata. Source File | `data.metadata.source_file` | `string` | `"downloads/2211700006/khs/2022_2023_GENAP.pdf"` | Path to the source PDF file |
| Metadata. File Size | `data.metadata.file_size` | `integer` | `193331` | Source PDF file size in bytes |

**Response 400 Bad Request — Invalid JSON Body:**

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

**Response 400 Bad Request — Missing NPM:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Non-Digits:**

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

**Response 404 Not Found — Extraction Data Not Found:**

```json
{
  "status": "error",
  "message": "KHS extraction not found",
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

## Data Type Reference

All Go structs from `internal/domain/entity/` used in API responses.

### Primitive Types

| Go Type | JSON Type | Example | Notes |
|---------|-----------|---------|-------|
| `string` | `string` | `"hello"` | UTF-8 text |
| `int` | `integer` | `42` | Signed integer |
| `float64` | `number` | `3.65` | IEEE 754 double-precision |
| `bool` | `boolean` | `true` | `true` or `false` |
| `*string` | `string \| null` | `null` | Nullable string. Pointer in Go; `null` in JSON when nil |
| `time.Time` | `string` (ISO 8601) | `"2026-08-06T12:45:00+07:00"` | RFC 3339 format with timezone |

### Mahasiswa

Student identity information, shared by both KRS and KHS.

```go
type Mahasiswa struct {
    Nama         string `json:"nama"`
    NPM          string `json:"npm"`
    ProgramStudi string `json:"program_studi"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| Nama | `nama` | `string` | `"MOCHAMAD IZZAN FIRASYANSYAH"` | Student full name (uppercase) |
| NPM | `npm` | `string` | `"2211700006"` | Student identification number |
| ProgramStudi | `program_studi` | `string` | `"Sistem Informasi"` | Study program name |

### TahunAjaran

Academic year range (e.g. 2025/2026).

```go
type TahunAjaran struct {
    Awal  string `json:"awal"`
    Akhir string `json:"akhir"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| Awal | `awal` | `string` | `"2025"` | Start year (YYYY) |
| Akhir | `akhir` | `string` | `"2026"` | End year (YYYY) |

### Periode

Academic period, used by both KRS and KHS.

```go
type Periode struct {
    TahunAjaran TahunAjaran `json:"tahun_ajaran"`
    Semester    string      `json:"semester"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| TahunAjaran | `tahun_ajaran` | `TahunAjaran` | `{"awal":"2025","akhir":"2026"}` | Academic year range |
| Semester | `semester` | `string` | `"GENAP"` | `"GANJIL"` (odd) or `"GENAP"` (even) |

### Penerbitan

Document publication info, used by both KRS and KHS.

```go
type Penerbitan struct {
    Tempat  string `json:"tempat"`
    Tanggal string `json:"tanggal"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| Tempat | `tempat` | `string` | `"Yogyakarta"` | Publication location |
| Tanggal | `tanggal` | `string` | `"2026-01-15"` | Publication date (YYYY-MM-DD) |

### ExtractionMetadata

Metadata about the extraction process.

```go
type ExtractionMetadata struct {
    ExtractedAt time.Time `json:"extracted_at"`
    SourceFile  string    `json:"source_file"`
    FileSize    int       `json:"file_size"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| ExtractedAt | `extracted_at` | `string` (ISO 8601) | `"2026-08-06T12:45:00+07:00"` | When extraction was performed |
| SourceFile | `source_file` | `string` | `"downloads/2211700006/krs/semester_8.pdf"` | Path to source PDF |
| FileSize | `file_size` | `integer` | `183701` | Source PDF size in bytes |

### KRS Types

#### KRSJadwal

Class schedule for a KRS course.

```go
type KRSJadwal struct {
    Hari         string `json:"hari"`
    WaktuMulai   string `json:"waktu_mulai"`
    WaktuSelesai string `json:"waktu_selesai"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| Hari | `hari` | `string` | `"Senin"` | Day of week (Indonesian) |
| WaktuMulai | `waktu_mulai` | `string` | `"08:00"` | Start time (HH:MM) |
| WaktuSelesai | `waktu_selesai` | `string` | `"10:30"` | End time (HH:MM) |

#### KRSMataKuliah

A course entry in KRS.

```go
type KRSMataKuliah struct {
    No     int       `json:"no"`
    Kode   string    `json:"kode"`
    Nama   string    `json:"nama"`
    SKS    int       `json:"sks"`
    Kelas  string    `json:"kelas"`
    Dosen  string    `json:"dosen"`
    Jadwal KRSJadwal `json:"jadwal"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| No | `no` | `integer` | `1` | Sequence number |
| Kode | `kode` | `string` | `"SI401"` | Course code |
| Nama | `nama` | `string` | `"Sistem Informasi Manajemen"` | Course name |
| SKS | `sks` | `integer` | `3` | Credit units |
| Kelas | `kelas` | `string` | `"A"` | Class section |
| Dosen | `dosen` | `string` | `"Dr. Budi Santoso, M.Kom."` | Lecturer name |
| Jadwal | `jadwal` | `KRSJadwal` | *(see above)* | Class schedule |

#### KRSPersetujuan

Approval section in KRS.

```go
type KRSPersetujuan struct {
    Mahasiswa struct {
        Nama string `json:"nama"`
    } `json:"mahasiswa"`
    KetuaProgramStudi struct {
        Jabatan string  `json:"jabatan"`
        Nama    *string `json:"nama"`
        NIDN    *string `json:"nidn"`
    } `json:"ketua_program_studi"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| Mahasiswa. Nama | `mahasiswa.nama` | `string` | `"MOCHAMAD IZZAN FIRASYANSYAH"` | Student name |
| Ketua Prodi. Jabatan | `ketua_program_studi.jabatan` | `string` | `"Ketua Program Studi"` | Position title |
| Ketua Prodi. Nama | `ketua_program_studi.nama` | `string \| null` | `"Dr. Eko Prasetyo, M.Kom."` | Head of study program. `null` if unavailable |
| Ketua Prodi. NIDN | `ketua_program_studi.nidn` | `string \| null` | `"0425088901"` | National lecturer ID. `null` if unavailable |

#### KRSExtraction

Full extracted KRS data (returned by `GET /api/v1/lms/krs/data`).

```go
type KRSExtraction struct {
    KRS struct {
        Mahasiswa   Mahasiswa       `json:"mahasiswa"`
        Periode     Periode         `json:"periode"`
        MataKuliah  []KRSMataKuliah `json:"mata_kuliah"`
        TotalSKS    int             `json:"total_sks"`
        Penerbitan  Penerbitan      `json:"penerbitan"`
        Persetujuan KRSPersetujuan  `json:"persetujuan"`
    } `json:"krs"`
    Metadata ExtractionMetadata `json:"metadata"`
}
```

### KHS Types

#### KHSMataKuliah

A course entry in KHS.

```go
type KHSMataKuliah struct {
    No    int    `json:"no"`
    Kode  string `json:"kode"`
    Nama  string `json:"nama"`
    Dosen string `json:"dosen"`
    SKS   int    `json:"sks"`
    Nilai string `json:"nilai"`
    Mutu  int    `json:"mutu"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| No | `no` | `integer` | `1` | Sequence number |
| Kode | `kode` | `string` | `"SI301"` | Course code |
| Nama | `nama` | `string` | `"Basis Data Lanjut"` | Course name |
| Dosen | `dosen` | `string` | `"Dr. Hendra Wijaya, M.T."` | Lecturer name |
| SKS | `sks` | `integer` | `3` | Credit units |
| Nilai | `nilai` | `string` | `"A"` | Letter grade: `"A"`, `"B+"`, `"B"`, `"C+"`, `"C"`, `"D"`, `"E"` |
| Mutu | `mutu` | `integer` | `4` | Grade point (0-4 scale) |

#### KHSRekapitulasi

Summary statistics in KHS.

```go
type KHSRekapitulasi struct {
    TotalSKS  int     `json:"total_sks"`
    TotalMutu int     `json:"total_mutu"`
    IPK       float64 `json:"ipk"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| TotalSKS | `total_sks` | `integer` | `23` | Total credit units for the semester |
| TotalMutu | `total_mutu` | `integer` | `84` | Total grade points (mutu) |
| IPK | `ipk` | `number` | `3.65` | Cumulative GPA (Indeks Prestasi Kumulatif) |

#### KHSPersetujuan

Approval section in KHS.

```go
type KHSPersetujuan struct {
    Dekan struct {
        Jabatan string `json:"jabatan"`
        Nama    string `json:"nama"`
        NIDN    string `json:"nidn"`
    } `json:"dekan"`
}
```

| Field | JSON Key | Type | Example | Description |
|-------|----------|------|---------|-------------|
| Dekan. Jabatan | `dekan.jabatan` | `string` | `"Dekan Fakultas Ilmu Komputer"` | Dean position title |
| Dekan. Nama | `dekan.nama` | `string` | `"Prof. Dr. Rina Hartati, M.Si."` | Dean name |
| Dekan. NIDN | `dekan.nidn` | `string` | `"0412066201"` | National lecturer ID |

#### KHSExtraction

Full extracted KHS data (returned by `GET /api/v1/lms/khs/data`).

```go
type KHSExtraction struct {
    KHS struct {
        Mahasiswa    Mahasiswa       `json:"mahasiswa"`
        Periode      Periode         `json:"periode"`
        MataKuliah   []KHSMataKuliah `json:"mata_kuliah"`
        Rekapitulasi KHSRekapitulasi `json:"rekapitulasi"`
        Penerbitan   Penerbitan      `json:"penerbitan"`
        Persetujuan  KHSPersetujuan  `json:"persetujuan"`
    } `json:"khs"`
    Metadata ExtractionMetadata `json:"metadata"`
}
```

### Request Types

#### LoginRequest

```go
type LoginRequest struct {
    NPM      string `json:"npm"`
    Password string `json:"password"`
}
```

#### KRSDownloadRequest

```go
type KRSDownloadRequest struct {
    NPM      string `json:"npm"`
    Password string `json:"password"`
}
```

#### KHSSemestersRequest

```go
type KHSSemestersRequest struct {
    NPM      string `json:"npm"`
    Password string `json:"password"`
}
```

#### KHSDownloadRequest

```go
type KHSDownloadRequest struct {
    NPM         string `json:"npm"`
    Password    string `json:"password"`
    TahunAjaran string `json:"tahun_ajaran"`
    Semester    string `json:"semester"`
}
```

### Response Types

#### LoginResult

```go
type LoginResult struct {
    Success   bool      `json:"success"`
    Message   string    `json:"message"`
    NPM       string    `json:"npm"`
    Timestamp time.Time `json:"timestamp"`
}
```

#### KRSDownloadResult

```go
type KRSDownloadResult struct {
    Success   bool      `json:"success"`
    Message   string    `json:"message"`
    NPM       string    `json:"npm"`
    FilePath  string    `json:"file_path"`
    Size      int       `json:"size"`
    Timestamp time.Time `json:"timestamp"`
}
```

#### KHSDownloadResult

```go
type KHSDownloadResult struct {
    Success     bool      `json:"success"`
    Message     string    `json:"message"`
    NPM         string    `json:"npm"`
    TahunAjaran string    `json:"tahun_ajaran"`
    Semester    string    `json:"semester"`
    FilePath    string    `json:"file_path"`
    Size        int       `json:"size"`
    Timestamp   time.Time `json:"timestamp"`
}
```

#### KHSSemestersResult

```go
type KHSSemestersResult struct {
    Success   bool          `json:"success"`
    Message   string        `json:"message"`
    NPM       string        `json:"npm"`
    Semesters []KHSSemester `json:"semesters"`
    Timestamp time.Time     `json:"timestamp"`
}
```

#### KHSSemester

```go
type KHSSemester struct {
    TahunAjaran string `json:"tahun_ajaran"`
    Semester    string `json:"semester"`
    SKS         int    `json:"sks"`
}
```

#### ExtractionResult

```go
type ExtractionResult struct {
    Success   bool      `json:"success"`
    Message   string    `json:"message"`
    NPM       string    `json:"npm"`
    FilePath  string    `json:"file_path"`
    Timestamp time.Time `json:"timestamp"`
}
```

### Constants

| Constant | Value | Used In |
|----------|-------|---------|
| `SemesterGanjil` | `"GANJIL"` | Semester validation |
| `SemesterGenap` | `"GENAP"` | Semester validation |
| `DocTypeKRS` | `"krs"` | Extraction cache directory |
| `DocTypeKHS` | `"khs"` | Extraction cache directory |
| `ExtPDF` | `".pdf"` | File extension |
| `ExtJSON` | `".json"` | File extension |
| `KRSFilePrefix` | `"semester_"` | KRS filename prefix |

### Validation Rules

| Field | Rules |
|-------|-------|
| `npm` | Required. Digits only (`^[0-9]+$`). 8-12 characters. |
| `password` | Required. Any string. |
| `tahun_ajaran` | Required (where applicable). Format: `YYYY/YYYY`. |
| `semester` | Required (where applicable). Must be `"GANJIL"` or `"GENAP"` (case-insensitive on input, auto-uppercased). |

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
- KRS: `{downloadDir}/{npm}/krs/semester_{N}.pdf` (N = semester number extracted from KRS page)
- KHS: `{downloadDir}/{npm}/khs/{tahun_ajaran}_{semester}.pdf`
- `tahun_ajaran`: `/` replaced with `_` (e.g. `2022/2023` → `2022_2023`)
- Files are overwritten if they already exist (latest download wins)

### Extraction Cache Structure

Extracted JSON files are stored under `{EXTRACT_DIR}/{NPM}/`:

```
extracted/
├── {NPM}/
│   ├── krs/
│   │   └── semester_{N}.json          # KRS extraction cache
│   └── khs/
│       ├── 2022_2023_GANJIL.json      # KHS extraction cache
│       ├── 2022_2023_GENAP.json
│       └── 2023_2024_GANJIL.json
```

**Naming Rules:**
- KRS: `{extractDir}/{npm}/krs/semester_{N}.json`
- KHS: `{extractDir}/{npm}/khs/{tahun_ajaran}_{semester}.json`
- `/` in `tahun_ajaran` replaced with `_`

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

## Error Handling

### HTTP Status Codes

| HTTP Status | Constructor | When |
|-------------|-------------|------|
| 400 | `apperror.BadRequest(msg)` | Invalid request (body parse error, missing fields, invalid NPM format) |
| 401 | `apperror.Unauthorized(msg)` | Authentication failure (LMS login failed) |
| 403 | `apperror.Forbidden(msg)` | Permission denied (file system permission denied) |
| 404 | `apperror.NotFound(msg, err)` | Resource not found (PDF missing, extraction data not found) |
| 500 | `apperror.Internal(msg, err)` | Infrastructure failure (browser crash, page load error, PDF corrupted) |

### Extraction-Specific Errors

| HTTP Status | Scenario | Message Pattern |
|-------------|----------|-----------------|
| 404 | KRS PDF not found | `KRS PDF not found for npm: {npm}` |
| 404 | KHS PDF not found | `KHS PDF not found` |
| 404 | KRS extraction not found | `KRS extraction not found for npm: {npm}` |
| 404 | KHS extraction not found | `KHS extraction not found` |
| 500 | PDF corrupted | `pdf file may be corrupted` |
| 500 | PDF empty | `pdf contains no extractable text` |
| 500 | PDF too large | `pdf too large: {size} bytes` |
| 500 | Parse error | `pdf parsing failed: {detail}` |

---

## Environment Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `APP_NAME` | `string` | `lonceng_unman_be` | Application name |
| `APP_ENV` | `string` | `development` | Environment: `development`, `staging`, `production` |
| `APP_PORT` | `string` | `3000` | Server port (validated: 1-65535) |
| `APP_HOST` | `string` | `0.0.0.0` | Bind address |
| `LMS_BASE_URL` | `string` | `https://elearning.universitasmandiri.ac.id` | LMS base URL |
| `LMS_DASHBOARD_URL` | `string` | `https://elearning.universitasmandiri.ac.id/admin/` | Dashboard URL (used for login success detection) |
| `BROWSER_HEADLESS` | `string` | `true` | Run Chrome headless (`true`/`false`) |
| `BROWSER_TIMEOUT` | `string` | `60s` | Overall browser operation timeout (Go duration format) |
| `DNS_TIMEOUT` | `string` | `5s` | DNS lookup timeout before browser connection (Go duration format) |
| `DOWNLOAD_DIR` | `string` | `./downloads` | PDF download directory |
| `EXTRACT_DIR` | `string` | `./extracted` | Directory for extracted JSON cache files |
| `SESSION_TTL` | `string` | `15m` | Session cache duration before expiry (Go duration format) |
| `MAX_SESSIONS` | `string` | `10` | Maximum cached browser sessions in memory |
| `PROFILE_BASE_DIR` | `string` | `./profiles` | Base directory for persistent Chrome profiles |
| `MAX_BODY_SIZE` | `string` | `1MB` | Max HTTP request body size (Fiber format) |
| `MAX_PDF_SIZE` | `string` | `50MB` | Max PDF file size for extraction |
| `CORS_ALLOW_ORIGINS` | `string` | `*` | CORS allowed origins |
| `CORS_ALLOW_METHODS` | `string` | `GET,POST,OPTIONS` | CORS allowed HTTP methods |
| `CORS_ALLOW_HEADERS` | `string` | `Content-Type` | CORS allowed request headers |

---

## Prerequisites

- Go 1.26.4+
- Chrome/Chromium installed (required for LMS login via go-rod)
- go-rod v0.116.2 (browser automation, auto-installed via `go mod tidy`)
- razvandimescu/gopdf v0.9.5 (PDF generation, auto-installed via `go mod tidy`)
