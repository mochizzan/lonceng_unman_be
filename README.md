<div align="center">

# 🔔 Lonceng Unman Backend

**RESTful API Backend** — Go Fiber v3 + Clean Architecture

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Fiber](https://img.shields.io/badge/Fiber-v3.4-000000?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxNiIgaGVpZ2h0PSIxNiIgdmlld0JveD0iMCAwIDE2IDE2Ij48ZyBmaWxsPSJub25lIiBmaWxsLXJ1bGU9ImV2ZW5vZGQiPjxwYXRoIGZpbGw9IiMwMEFERDgiIGZpbGwtb3BhY2l0eT0iLjkiIGQ9Ik04IDFhNyA3IDAgMSAxLTcgN0E3IDcgMCAwIDEgOCAxem0wIDJhNSA1IDAgMSAwIDAgMTAgNSA1IDAgMCAwIDAtMTB6Ii8+PC9nPjwvc3ZnPg==)](https://gofiber.io/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

</div>

---

## 📋 Ringkasan

Backend API untuk sistem **Lonceng Unman** — dibangun dengan arsitektur bersih (Clean Architecture) menggunakan Go dan Fiber v3. Mendukung login LMS, download KRS/KHS PDF, dan browser automation untuk integrasi dengan sistem e-learning.

## 🏗️ Arsitektur

```
cmd/server/          → Entry point, wiring dependency
internal/
├── config/          → Konfigurasi dari environment variables
├── domain/entity/   → Entitas bisnis (pure data models)
├── application/     → Use case / service layer
├── interfaces/      → HTTP handlers, router, response helpers
└── infrastructure/  → Middleware (CORS, Logger, Recover)
```

**Prinsip:**
- **Dependency Inversion** — dependency selalu menuju inner layer
- **Separation of Concerns** — setiap file punya satu tanggung jawab
- **Zero Hardcoding** — semua konfigurasi dari environment variables

## ⚡ Quick Start

### Prasyarat

- [Go 1.26+](https://go.dev/dl/)

### Instalasi

```bash
# Clone repository
git clone https://github.com/mochizzan/lonceng_unman_be.git
cd lonceng_unman_be

# Copy environment config
cp .env.example .env

# Install dependencies
go mod tidy

# Jalankan server
go run ./cmd/server/
```

Server akan berjalan di `http://localhost:3000`

### Testing Endpoint

```bash
# Health check
curl http://localhost:3000/api/v1/health
```

Response:

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

## 🔧 Konfigurasi

Semua konfigurasi diambil dari environment variables. Copy `.env.example` ke `.env` dan sesuaikan:

| Variable | Default | Deskripsi |
|----------|---------|-----------|
| `APP_NAME` | `lonceng_unman_be` | Nama aplikasi |
| `APP_ENV` | `development` | Environment (`development` / `production`) |
| `APP_PORT` | `3000` | Port server |
| `APP_HOST` | `0.0.0.0` | Host bind address |
| `CORS_ALLOW_ORIGINS` | `*` | Origin yang diizinkan CORS |
| `CORS_ALLOW_METHODS` | `GET,POST,PUT,PATCH,DELETE,OPTIONS` | Method HTTP yang diizinkan |
| `CORS_ALLOW_HEADERS` | `Content-Type,Authorization` | Header yang diizinkan |
| `LMS_BASE_URL` | `https://elearning.universitasmandiri.ac.id` | Base URL LMS |
| `LMS_DASHBOARD_URL` | `https://elearning.universitasmandiri.ac.id/admin/` | URL dashboard LMS |
| `BROWSER_HEADLESS` | `true` | Jalankan browser tanpa GUI |
| `BROWSER_TIMEOUT` | `30s` | Timeout untuk operasi browser |
| `ACTION_TIMEOUT` | `10s` | Timeout untuk aksi spesifik |
| `DOWNLOAD_DIR` | `./downloads` | Direktori penyimpanan file download |

## 📁 Struktur Project

```
lonceng_unman_be/
├── cmd/server/main.go                 # Entry point & DI wiring
├── internal/
│   ├── config/config.go               # Env-based configuration
│   ├── apperror/apperror.go           # Shared error types
│   ├── domain/entity/                 # Business entities
│   │   ├── health.go                  # Health entity
│   │   ├── lms.go                     # LMS login entity
│   │   └── document.go                # Document download entities
│   ├── application/service/           # Business logic
│   │   ├── health_service.go          # Health check service
│   │   ├── lms_service.go             # LMS login & document service
│   │   └── document_service.go        # Document download service
│   ├── interfaces/http/               # HTTP layer
│   │   ├── handler/                   # Request handlers
│   │   │   ├── health_handler.go
│   │   │   ├── lms_handler.go
│   │   │   └── document_handler.go
│   │   ├── router/router.go           # Route definitions
│   │   └── response/response.go       # JSON response helpers
│   └── infrastructure/                # External integrations
│       ├── browser/                   # Headless browser automation
│       │   ├── browser.go             # Browser connection
│       │   ├── selectors.go           # CSS selectors
│       │   └── download.go            # PDF download & save
│       ├── middleware/                 # HTTP middleware
│       ├── logger/logger.go           # Structured logging
│       └── fibererror/handler.go      # Global error handler
├── tests/                             # External test packages
├── .env.example                       # Environment template
├── .gitignore
├── go.mod
└── go.sum
```

## 🛠️ Tech Stack

| Komponen | Teknologi |
|----------|-----------|
| **Language** | Go 1.26+ |
| **Framework** | [Fiber v3](https://gofiber.io/) |
| **Browser Automation** | [go-rod](https://github.com/go-rod/rod) |
| **Config** | [godotenv](https://github.com/joho/godotenv) |
| **Validation** | [go-playground/validator](https://github.com/go-playground/validator) |
| **ID Generation** | [google/uuid](https://github.com/google/uuid) |

## 📝 API Endpoints

| Method | Endpoint | Deskripsi |
|--------|----------|-----------|
| `GET` | `/api/v1/health` | Health check |
| `POST` | `/api/v1/lms/login` | Login ke LMS |
| `GET` | `/api/v1/lms/krs?npm=xxx` | Download KRS PDF |
| `GET` | `/api/v1/lms/khs/semesters?npm=xxx` | Daftar semester KHS |
| `GET` | `/api/v1/lms/khs?npm=xxx&tahun_ajaran=xxx&semester=xxx` | Download KHS PDF |

## 📄 Response Format

Semua endpoint mengembalikan JSON envelope:

```json
{
  "status": "success | error",
  "data": {},
  "message": "Deskripsi response",
  "errors": {}
}
```

### Contoh Response

**Health Check:**
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

**KRS Download:**
```json
{
  "status": "success",
  "data": {
    "success": true,
    "message": "KRS downloaded successfully",
    "npm": "2211700006",
    "file_path": "downloads/2211700006/krs/krs.pdf",
    "size": 12345,
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "KRS downloaded successfully"
}
```

**KHS Semesters:**
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
      }
    ],
    "timestamp": "2026-08-06T00:45:00+07:00"
  },
  "message": "KHS semesters retrieved"
}
```

**KHS Download:**
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

**Error Response:**
```json
{
  "status": "error",
  "message": "npm query parameter is required",
  "trace_id": "abc123"
}
```

## 📂 Struktur Download

File PDF yang di-download disimpan dengan nama canonical untuk mencegah duplikasi:

```
downloads/
├── {NPM}/
│   ├── krs/
│   │   └── krs.pdf                    # KRS (selalu overwritten)
│   └── khs/
│       ├── 2022_2023_GANJIL.pdf        # KHS per semester
│       ├── 2022_2023_GENAP.pdf
│       └── 2023_2024_GANJIL.pdf
```

**Aturan Penamaan:**
- KRS: `{downloadDir}/{npm}/krs/krs.pdf` (overwritten setiap download)
- KHS: `{downloadDir}/{npm}/khs/{tahun_ajaran}_{semester}.pdf`
- `tahun_ajaran`: `/` diganti `_` (contoh: `2022/2023` → `2022_2023`)

## 🤝 Kontribusi

1. Fork repository ini
2. Buat branch feature (`git checkout -b feat/nama-fitur`)
3. Commit perubahan (`git commit -m 'feat: tambah deskripsi'`)
4. Push ke branch (`git push origin feat/nama-fitur`)
5. Buka Pull Request

## 📄 Lisensi

MIT License — lihat [LICENSE](LICENSE) untuk detail.

---

<div align="center">

**Built with ❤️ using Go + Fiber**

</div>
