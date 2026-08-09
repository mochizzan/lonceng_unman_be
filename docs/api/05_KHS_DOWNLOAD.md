# Download KHS

Download KHS (Kartu Hasil Studi) PDF for a specific student and semester.

**Endpoint:** `POST /api/v1/lms/khs`

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

**Response 500 Internal Server Error — Download Failed:**

```json
{
  "status": "error",
  "message": "KHS download failed",
  "trace_id": "abc123..."
}
```

> **Note:** Possible causes: invalid semester value (not `GANJIL`/`GENAP`), LMS page load error, PDF generation failed, or network timeout. Invalid semester values are validated in the service layer and returned as a 500 error (not 400) because the check happens after the handler.

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
