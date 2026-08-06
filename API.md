# API Documentation

**Base URL:** `http://localhost:3000`

**Version:** v1

**Content-Type:** `application/json`

---

## Response Envelope

Semua endpoint mengembalikan JSON dengan envelope standar:

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
| `status` | string | `"success"` atau `"error"` |
| `data` | object/array | Payload (hanya pada success) |
| `message` | string | Deskripsi hasil |
| `trace_id` | string | Request ID unik (hanya pada error) |
| `errors` | object | Detail tambahan (opsional) |

---

## Endpoints

### 1. Health Check

Cek status kesehatan server.

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
| `data.service` | string | Nama aplikasi dari `APP_NAME` env |
| `data.version` | string | Environment dari `APP_ENV` env |

**Contoh curl:**

```bash
curl http://localhost:3000/api/v1/health
```

---

### 2. LMS Login

Login otomatis ke LMS Universitas Mandiri via headless browser.

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
| `npm` | string | ✅ | NPM (Nomor Pokok Mahasiswa) |
| `password` | string | ✅ | Password LMS |

**Response 200 OK — Login Berhasil:**

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

**Response 200 OK — Login Gagal (Credentials Salah):**

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

> **Catatan:** Login gagal tetap mengembalikan HTTP 200 dengan `data.success: false`.
> Error message berasal dari halaman LMS (sudah di-sanitize).

**Response 400 Bad Request — NPM Kosong:**

```json
{
  "status": "error",
  "message": "npm is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Password Kosong:**

```json
{
  "status": "error",
  "message": "password is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Body Tidak Valid:**

```json
{
  "status": "error",
  "message": "invalid request body: ...",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error — Browser Gagal:**

```json
{
  "status": "error",
  "message": "login operation failed",
  "trace_id": "abc123..."
}
```

> **Catatan:** Detail error internal dicatat di server log, tidak diekspos ke client.
> Kemungkinan penyebab: Chrome tidak terinstall, browser timeout, atau LMS down.

**Contoh curl:**

```bash
# Login berhasil
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"izzan027"}'

# Login gagal (credentials salah)
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"wrong","password":"wrong"}'

# NPM kosong
curl -X POST http://localhost:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"password":"test"}'
```

---

### 3. Download KRS

Download file PDF KRS (Kartu Rencana Studi) untuk mahasiswa tertentu.

```
GET /api/v1/lms/krs?npm=xxx
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `npm` | string | ✅ | NPM (hanya digit, regex: `^[0-9]+$`) |

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
| `data.success` | bool | Status download |
| `data.message` | string | Deskripsi hasil |
| `data.npm` | string | NPM mahasiswa |
| `data.file_path` | string | Path file tersimpan (canonical) |
| `data.size` | int | Ukuran file dalam bytes |
| `data.timestamp` | string | Waktu download (ISO 8601) |

**Response 400 Bad Request — NPM Kosong:**

```json
{
  "status": "error",
  "message": "npm query parameter is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Tidak Valid:**

```json
{
  "status": "error",
  "message": "npm must contain only digits",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error — Download Gagal:**

```json
{
  "status": "error",
  "message": "KRS download failed",
  "trace_id": "abc123..."
}
```

**Contoh curl:**

```bash
curl "http://localhost:3000/api/v1/lms/krs?npm=2211700006"
```

**Lokasi File Tersimpan:**

```
downloads/2211700006/krs/semester_8.pdf
```

> **Catatan:** File KRS selalu disimpan sebagai `krs.pdf` dan di-overwrite jika sudah ada.

---

### 4. Get KHS Semesters

Mendapatkan daftar semester yang tersedia untuk KHS (Kartu Hasil Studi) mahasiswa tertentu.

```
GET /api/v1/lms/khs/semesters?npm=xxx
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `npm` | string | ✅ | NPM (hanya digit, regex: `^[0-9]+$`) |

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
| `data.npm` | string | NPM mahasiswa |
| `data.semesters` | array | Daftar semester tersedia |
| `data.semesters[].tahun_ajaran` | string | Tahun ajaran (format: `YYYY/YYYY`) |
| `data.semesters[].semester` | string | Semester (`GANJIL` atau `GENAP`) |
| `data.semesters[].sks` | int | Total SKS untuk semester tersebut |
| `data.timestamp` | string | Waktu pengambilan data (ISO 8601) |

**Response 400 Bad Request — NPM Kosong:**

```json
{
  "status": "error",
  "message": "npm query parameter is required",
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

**Contoh curl:**

```bash
curl "http://localhost:3000/api/v1/lms/khs/semesters?npm=2211700006"
```

---

### 5. Download KHS

Download file PDF KHS (Kartu Hasil Studi) untuk semester tertentu.

```
GET /api/v1/lms/khs?npm=xxx&tahun_ajaran=xxx&semester=xxx
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `npm` | string | ✅ | NPM (hanya digit, regex: `^[0-9]+$`) |
| `tahun_ajaran` | string | ✅ | Tahun ajaran (format: `YYYY/YYYY`, contoh: `2022/2023`) |
| `semester` | string | ✅ | Semester (`GANJIL` atau `GENAP`, case-sensitive) |

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
| `data.success` | bool | Status download |
| `data.message` | string | Deskripsi hasil |
| `data.npm` | string | NPM mahasiswa |
| `data.tahun_ajaran` | string | Tahun ajaran |
| `data.semester` | string | Semester |
| `data.file_path` | string | Path file tersimpan (canonical) |
| `data.size` | int | Ukuran file dalam bytes |
| `data.timestamp` | string | Waktu download (ISO 8601) |

**Response 400 Bad Request — Parameter Kosong:**

```json
{
  "status": "error",
  "message": "npm query parameter is required",
  "trace_id": "abc123..."
}
```

```json
{
  "status": "error",
  "message": "tahun_ajaran query parameter is required",
  "trace_id": "abc123..."
}
```

```json
{
  "status": "error",
  "message": "semester query parameter is required",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — NPM Tidak Valid:**

```json
{
  "status": "error",
  "message": "npm must contain only digits",
  "trace_id": "abc123..."
}
```

**Response 400 Bad Request — Semester Tidak Valid:**

```json
{
  "status": "error",
  "message": "semester must be GANJIL or GENAP",
  "trace_id": "abc123..."
}
```

**Response 500 Internal Server Error — Download Gagal:**

```json
{
  "status": "error",
  "message": "KHS download failed",
  "trace_id": "abc123..."
}
```

**Contoh curl:**

```bash
curl "http://localhost:3000/api/v1/lms/khs?npm=2211700006&tahun_ajaran=2022/2023&semester=GANJIL"
```

**Lokasi File Tersimpan:**

```
downloads/2211700006/khs/2022_2023_GANJIL.pdf
```

> **Catatan:** File KHS disimpan dengan nama canonical `{tahun_ajaran}_{semester}.pdf`.
> `tahun_ajaran` dengan `/` akan diganti `_` (contoh: `2022/2023` → `2022_2023`).
> File di-overwrite jika sudah ada (latest download wins).

---

## Download Folder Structure

Semua file PDF disimpan di bawah `{DOWNLOAD_DIR}/{NPM}/` dengan nama canonical:

```
downloads/
├── {NPM}/
│   ├── krs/
│   │   └── semester_{N}.pdf           # KHS per semester (N = nomor semester)
│   └── khs/
│       ├── 2022_2023_GANJIL.pdf        # KHS per semester
│       ├── 2022_2023_GENAP.pdf
│       └── 2023_2024_GANJIL.pdf
```

**Aturan Penamaan:**
- KRS: `{downloadDir}/{npm}/krs/semester_{N}.pdf` (N = nomor semester mahasiswa)
- KHS: `{downloadDir}/{npm}/khs/{tahun_ajaran}_{semester}.pdf`
- `tahun_ajaran`: `/` diganti `_` (contoh: `2022/2023` → `2022_2023`)

---

## Login Flow (Internal)

```
Client
  │
  ▼
POST /api/v1/lms/login
  │
  ▼
Handler: validate npm & password
  │
  ▼
Service: launch headless Chrome
  │
  ▼
Navigate to LMS login page (/)
  │
  ▼
Fill #username + input[name=password]
  │
  ▼
Click input[type=submit]
  │
  ▼
Race: .wrapper (success) vs .alert-danger (failure)
  │
  ├── .wrapper matched ──▶ Verify URL contains /admin/ ──▶ Return success
  │
  └── .alert-danger matched ──▶ Return failure + error text
```

---

## Error Handling

| HTTP Status | Constructor | Kapan Terjadi |
|-------------|-------------|---------------|
| 400 | `apperror.BadRequest(msg)` | Request tidak valid (body parse error, field kosong) |
| 401 | `apperror Unauthorized(msg)` | Belum diimplementasi |
| 403 | `apperror Forbidden(msg)` | Belum diimplementasi |
| 404 | `apperror NotFound(msg)` | Endpoint tidak ditemukan |
| 500 | `apperror Internal(msg, err)` | Infrastruktur gagal (browser crash, page load error) |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_NAME` | `lonceng_unman_be` | Nama aplikasi |
| `APP_ENV` | `development` | Environment: development, staging, production |
| `APP_PORT` | `3000` | Port server (1-65535) |
| `APP_HOST` | `0.0.0.0` | Bind address |
| `LMS_BASE_URL` | `https://elearning.universitasmandiri.ac.id` | Base URL LMS |
| `LMS_DASHBOARD_URL` | `https://elearning.universitasmandiri.ac.id/admin/` | URL dashboard setelah login |
| `BROWSER_HEADLESS` | `true` | Jalankan Chrome headless |
| `BROWSER_TIMEOUT` | `30s` | Timeout keseluruhan operasi browser |
| `ACTION_TIMEOUT` | `10s` | Timeout per aksi (click, fill, dll) |
| `DOWNLOAD_DIR` | `./downloads` | Direktori penyimpanan file download |

---

## Prerequisites

- Go 1.26.4+
- Chrome/Chromium terinstall (untuk LMS login)
- go-rod v0.116.2 (otomatis di-install via `go mod tidy`)
