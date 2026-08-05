<div align="center">

# 🔔 Lonceng Unman Backend

**RESTful API Backend** — Go Fiber v3 + Clean Architecture

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Fiber](https://img.shields.io/badge/Fiber-v3.4-000000?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxNiIgaGVpZ2h0PSIxNiIgdmlld0JveD0iMCAwIDE2IDE2Ij48ZyBmaWxsPSJub25lIiBmaWxsLXJ1bGU9ImV2ZW5vZGQiPjxwYXRoIGZpbGw9IiMwMEFERDgiIGZpbGwtb3BhY2l0eT0iLjkiIGQ9Ik04IDFhNyA3IDAgMSAxLTcgN0E3IDcgMCAwIDEgOCAxem0wIDJhNSA1IDAgMSAwIDAgMTAgNSA1IDAgMCAwIDAtMTB6Ii8+PC9nPjwvc3ZnPg==)](https://gofiber.io/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

</div>

---

## 📋 Ringkasan

Backend API untuk sistem **Lonceng Unman** — dibangun dengan arsitektur bersih (Clean Architecture) menggunakan Go dan Fiber v3. Dirancang untuk scalability, maintainability, dan performa tinggi.

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
| `CORS_ALLOW_ORIGins` | `*` | Origin yang diizinkan CORS |
| `CORS_ALLOW_METHODS` | `GET,POST,PUT,PATCH,DELETE,OPTIONS` | Method HTTP yang diizinkan |
| `CORS_ALLOW_HEADERS` | `Content-Type,Authorization` | Header yang diizinkan |

## 📁 Struktur Project

```
lonceng_unman_be/
├── cmd/server/main.go                 # Entry point
├── internal/
│   ├── config/config.go               # Env-based configuration
│   ├── domain/entity/health.go        # Health entity
│   ├── application/service/           # Business logic
│   ├── interfaces/http/               # HTTP layer
│   │   ├── handler/                   # Request handlers
│   │   ├── router/                    # Route definitions
│   │   └── response/                  # JSON response helpers
│   └── infrastructure/middleware/      # Middleware stack
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
| **Config** | [godotenv](https://github.com/joho/godotenv) |
| **Validation** | [go-playground/validator](https://github.com/go-playground/validator) |
| **ID Generation** | [google/uuid](https://github.com/google/uuid) |

## 📝 API Endpoints

| Method | Endpoint | Deskripsi |
|--------|----------|-----------|
| `GET` | `/api/v1/health` | Health check |

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
