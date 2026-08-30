# Repository Guidelines

## Project Overview

**lonceng_unman_be** is a Go/Fiber v3 Clean Architecture backend for LMS (Learning Management System) automation at Universitas Mandiri. It provides automated browser-based scraping of student profiles, KRS/KHS PDF extraction, and structured data caching via a REST API.

- **Stack**: Go 1.26.4, Fiber v3.4.0, go-rod (browser automation), gopdf (PDF parsing), godotenv
- **Production**: Windows / WSL2 / Docker / Cloudflared tunnel
- **Container Image**: `ghcr.io/mochizzan/lonceng_unman_be:latest`

---

## Architecture & Data Flow

### Clean Architecture Layers

The project follows a layered Clean Architecture with dependency inversion via port interfaces:

```
cmd/server/                          ← Composition root, DI wiring, entry point
  └── internal/interfaces/http/      ← HTTP handlers, router, middleware, response envelopes
      └── internal/application/service/  ← Use cases, orchestration, DTOs
          └── internal/domain/       ← Entities, value objects, port interfaces (business contracts)
              └── internal/infrastructure/  ← Concrete implementations (browser, session, PDF, cache)
```

### Layer Responsibilities

| Layer | Path | Responsibility |
|-------|------|----------------|
| **Entry** | `cmd/server/main.go` | Loads config, initializes logger, wires dependencies via constructor injection, registers routes, starts server |
| **Interfaces** | `internal/interfaces/http/` | Fiber handlers, middleware chain, response envelopes |
| **Application** | `internal/application/service/` | Use cases: LMS login, document extraction, student profile scraping, eval comparison |
| **Domain** | `internal/domain/entity/` + `internal/domain/port/` | Business entities and interface contracts (ports) that infrastructure implements |
| **Infrastructure** | `internal/infrastructure/` | go-rod browser, session manager, PDF extractor, photo cache, eval store, logger, auth middleware |

### Key Domain Ports (Interfaces)

Infrastructure implements these interfaces defined in the domain layer:

- `port.BrowserSession` — Navigate, Eval, ElementAttribute, ElementExists, DownloadPDF, DownloadImage, Close
- `port.SessionManager` — GetOrCreate, Close, CloseAll (per-NPM authenticated browser sessions)
- `port.PDFParser` — ParseKRS, ParseKHS, MarshalToJSON
- `port.ExtractionCache` — Get, Set, Exists, GetModTime, Invalidate
- `port.EvalStore` / `port.EvalService` — Ground-truth storage and evaluation logic

### Request Data Flow

```
HTTP Request
  → Fiber middleware (recover → requestid → logger → compress → CORS)
  → Handler (validates NPM, extracts params)
    → Application Service (orchestrates use case)
      → Domain Port interfaces (abstract contracts)
        → Infrastructure implementations (browser automation, PDF parsing, caching)
    → Response envelope (Success/Error)
  → HTTP Response
```

---

## Key Directories

| Directory | Purpose |
|-----------|---------|
| `cmd/server/` | Main server entry point and DI composition root |
| `cmd/extractor/` | CLI tool for raw PDF text extraction (debugging utility) |
| `internal/config/` | Configuration loading from env vars with validation |
| `internal/domain/entity/` | Core business entities (KRS, KHSMataKuliah, Mahasiswa, StudentProfile, etc.) |
| `internal/domain/port/` | Interface contracts that infrastructure must implement |
| `internal/application/service/` | Use case implementations (health, lms, extraction, student_profile, eval) |
| `internal/infrastructure/browser/` | go-rod browser automation, session scraper |
| `internal/infrastructure/session/` | In-memory session manager with TTL eviction |
| `internal/infrastructure/extractor/` | KRS/KHS PDF parsing with gopdf |
| `internal/infrastructure/photocache/` | TTL-based student photo cache with compression |
| `internal/infrastructure/evalstore/` | JSON file-based ground-truth storage |
| `internal/interfaces/http/handler/` | HTTP handlers (health, lms, document, extraction, student_profile, eval, eval_gt, auth) |
| `internal/interfaces/http/response/` | Standard API response envelopes |
| `internal/interfaces/http/router/` | Route registration |
| `internal/infrastructure/middleware/` | Auth, CORS, logging middleware |
| `internal/infrastructure/logger/` | slog-based structured logger |
| `internal/infrastructure/fibererror/` | Global Fiber error handler |
| `internal/apperror/` | AppError constructors with PublicMsg/Internal separation |
| `tests/` | Top-level test directory organized by feature |
| `docs/api/` | API documentation (16 endpoint docs) |
| `docs/architecture/` | Architecture diagrams and flow docs |
| `tmp/` | Utility scripts (smoke tests, Excel generation, Mermaid rendering) |

---

## Development Commands

### Build & Run

```bash
# Build server binary
go build -o bin/server.exe cmd/server/main.go

# Build and run (Windows)
./run.cmd

# Run directly
go run cmd/server/main.go

# Build extractor CLI
go build -o bin/extractor.exe cmd/extractor/main.go
```

### Docker

```bash
# Build image
docker build -t lonceng_unman_be .

# Run with compose
docker compose up -d

# Production image is multi-stage: golang:1.26.4-alpine (build+UPX) → alpine:3.20 (runtime+Chromium)
# Final image: ~6MB compressed, non-root appuser, healthcheck on /api/v1/health
```

### Environment Configuration

```bash
# Copy and edit environment
cp .env.example .env

# Required variables (see .env.example for full list):
# APP_NAME, APP_ENV (development/production), APP_PORT, APP_HOST
# LMS_BASE_URL, LMS_DASHBOARD_URL
# BROWSER_HEADLESS, BROWSER_TIMEOUT (min 60s for cold starts)
# DOWNLOAD_DIR, EXTRACT_DIR, PROFILE_BASE_DIR, EVAL_DIR
# SESSION_TTL, MAX_SESSIONS, MAX_BODY_SIZE, MAX_PDF_SIZE
# SECRET_KEY, AUTO_LOGIN, TZ
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./tests/student_profile/...
go test ./tests/service/...
go test ./tests/extractor/...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run smoke tests (requires .env.test or running server)
node tmp/smoke-test.mjs
go run tmp/extraction-smoke-test.go
```

### Linting & Formatting

```bash
# Format code
gofmt -w .

# Lint (requires golangci-lint installation)
golangci-lint run ./...
```

---

## Code Conventions & Common Patterns

### Dependency Injection

Manual constructor injection in `cmd/server/main.go`. Dependencies flow top-down:

```go
// Example: Session manager creation
sessionMgr := session.NewManager(logger, cfg.Browser, cfg.Session, cfg.DNS)

// Example: Service wiring with interfaces
studentProfileSvc := student_profile_service.NewService(
    sessionMgr,           // port.SessionManager
    scraper,              // port.StudentProfileScraper
    photoCache,           // port.ExtractionCache
    logger,
    cfg.PhotoCache,
)
```

### Interface Segregation

Small, focused interfaces in `internal/domain/port/`:

```go
type BrowserSession interface {
    Navigate(url string) error
    Eval(js string) (json.RawMessage, error)
    ElementAttribute(selector, attr string) (string, error)
    ElementExists(selector string) (bool, error)
    DownloadPDF(url string) ([]byte, error)
    DownloadImage(url string) ([]byte, error)
    Close() error
}
```

### Error Handling

Custom `AppError` type with public/internal message separation:

```go
// Constructors for common errors
apperr := apperror.BadRequest("NPM must be 8-12 digits")
apperr := apperror.NotFound("Session not found")
apperr := apperror.Internal("browser launch failed").Wrap(err)

// Sentinel errors for flow control
var ErrPDFNotFound = errors.New("PDF not found")
var ErrExtractionNotFound = errors.New("extraction not found")
```

### Response Envelopes

Standard API response structure (NOTE: envelopes are inconsistent — see Important Notes):

```go
// Success envelope
response.Success(c, data, "optional message")
// → { "status": "success", "data": {...}, "message": "..." }

// Error envelope
response.Error(c, http.StatusBadRequest, "error message", err)
// → { "status": "error", "message": "...", "errors": [...] }
```

### Logging

slog-based structured logger with environment-aware output:

```go
// Development: text handler, DEBUG level
// Production: JSON handler, INFO level
logger.Info("session created", "npm", npm, "ttl", ttl)
logger.Error("browser launch failed", "error", err)
```

### NPM Validation

Student ID (NPM) validation rules:
- Digits only (`^[0-9]+$`)
- 8–12 characters long
- Implemented in `internal/interfaces/http/handler/validateNPM`

### go-rod Browser Automation

```go
// IMPORTANT: go-rod Eval requires arrow functions, NOT function declarations
// Correct:
page.Eval(`() => document.querySelector('input').value`)

// Wrong:
page.Eval(`function() { return document.querySelector('input').value }`)

// Session management pattern:
defer session.Close()        // Always close browser sessions
defer sessionMgr.Stop()      // Always stop session manager on shutdown
```

### Session Management Pattern

```go
// Session restoration with profile directory
session, err := sessionMgr.GetOrCreate(npm, cfg.Browser.Headless)
if err != nil {
    return err
}
defer session.Close()

// DNS pre-flight check before browser operations
// Background cleanup loop evicts expired sessions
// Per-NPM locks prevent duplicate session creation
```

### PDF Extraction Flow

```go
// 1. Download PDF via browser automation
pdfBytes, err := session.DownloadPDF(pdfURL)

// 2. Parse via PDFParser interface
result, err := parser.ParseKRS(pdfBytes)
// or
result, err := parser.ParseKHS(pdfBytes)

// 3. Cache extracted JSON
cache.Set(npm, jsonBytes)
```

---

## Important Files

### Entry Points

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | Main server entry point, DI wiring, route registration |
| `cmd/extractor/main.go` | CLI for raw PDF text extraction (debugging) |

### Configuration

| File | Purpose |
|------|---------|
| `.env.example` | Complete environment variable template (24 vars) |
| `.env` | Development environment configuration |
| `.env.test` | Test environment for smoke tests |
| `internal/config/config.go` | Config loading, validation, path resolution |

### Core Domain

| File | Purpose |
|------|---------|
| `internal/domain/entity/extraction.go` | KRS/KHS extraction entities |
| `internal/domain/entity/student_profile.go` | Student profile entity (55+ fields) |
| `internal/domain/port/browser.go` | BrowserSession interface |
| `internal/domain/port/session.go` | SessionManager interface |
| `internal/domain/port/extraction.go` | PDFParser and ExtractionCache interfaces |

### Infrastructure

| File | Purpose |
|------|---------|
| `internal/infrastructure/browser/browser.go` | go-rod browser wrapper with Connect/ConnectWithProfile |
| `internal/infrastructure/browser/scraper.go` | Student profile scraper with bulk JS eval |
| `internal/infrastructure/session/manager.go` | Session manager with TTL eviction and per-NPM locks |
| `internal/infrastructure/extractor/krs.go` | KRS PDF parser |
| `internal/infrastructure/extractor/khs.go` | KHS PDF parser |
| `internal/infrastructure/photocache/cache.go` | TTL-based photo cache with compression |

### Application Services

| File | Purpose |
|------|---------|
| `internal/application/service/lms_service.go` | LMS login validation |
| `internal/application/service/extraction_service.go` | PDF extraction orchestration |
| `internal/application/service/student_profile_service.go` | Student profile scraping with photo cache |
| `internal/application/service/eval_service.go` | Evaluation metrics (TP/FN/FP/TN, Precision, Recall, F1) |

### HTTP Layer

| File | Purpose |
|------|---------|
| `internal/interfaces/http/handler/` | 8 HTTP handlers |
| `internal/interfaces/http/response/response.go` | Success/Error response envelopes |
| `internal/infrastructure/middleware/middleware.go` | Auth, CORS, logging middleware |
| `internal/infrastructure/fibererror/handler.go` | Global error handler |

### Tests

| File | Purpose |
|------|---------|
| `tests/student_profile/handler_test.go` | Student profile handler tests |
| `tests/student_profile/service_test.go` | Student profile service tests |
| `tests/extractor/krs_parser_test.go` | KRS parser tests |
| `tests/extractor/khs_parser_test.go` | KHS parser tests |
| `tests/service/eval_service_test.go` | Eval service tests |
| `tests/infrastructure/auth/middleware_test.go` | Auth middleware tests |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | Project overview, quick start, API endpoints |
| `docs/api/` | 16 API endpoint documentation files |
| `docs/architecture/` | Architecture diagrams and flow documentation |

---

## Testing & QA

### Test Framework

- **Standard library**: `testing` package exclusively (no testify, no mockery)
- **HTTP testing**: Fiber's built-in `app.Test()` method
- **Mocking**: Manual mocks via function-field structs

### Test Organization

Tests live in a top-level `tests/` directory organized by feature:

```
tests/
  student_profile/
    handler_test.go      ← HTTP handler tests
    service_test.go      ← Service layer tests
    entity_test.go       ← Entity serialization tests
  handler/
    auth_handler_test.go ← Auth handler tests
  service/
    eval_service_test.go ← Eval service tests
    health_service_test.go
  extractor/
    krs_parser_test.go   ← KRS parser tests
    khs_parser_test.go   ← KHS parser tests
    split_tahun_test.go  ← Year splitting tests
    parse_header_test.go ← Header normalization tests
  config/
    config_test.go       ← Config loading tests
  infrastructure/
    auth/
      middleware_test.go ← Auth middleware tests
  eval/
    handler_test.go      ← Eval handler tests
    preview_test.go      ← PDF preview tests
  fibererror/
    handler_test.go      ← Error handler tests
  logger/
    logger_test.go       ← Logger tests
  apperror/
    apperror_test.go     ← AppError constructor tests
```

### Test Patterns

**Manual Mock Pattern** (function-field structs):

```go
type mockStudentProfileService struct {
    scrapeFn func(npm string) (*entity.StudentProfile, error)
    getFn    func(npm string) (*entity.StudentProfile, error)
}

func (m *mockStudentProfileService) Scrape(npm string) (*entity.StudentProfile, error) {
    return m.scrapeFn(npm)
}

func (m *mockStudentProfileService) Get(npm string) (*entity.StudentProfile, error) {
    return m.getFn(npm)
}
```

**Test App Helper**:

```go
// newTestApp creates a Fiber app with mocked dependencies for HTTP testing
app := newTestApp(mockService)
req := httptest.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
req.Header.Set("Content-Type", "application/json")
resp, err := app.Test(req)
```

**Table-Driven Subtests**:

```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"underscore format", "2020_2021", "2020/2021"},
    {"slash format", "2020/2021", "2020/2021"},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := splitTahunAjaran(tt.input)
        require.Equal(t, tt.expected, result)
    })
}
```

### Test Helpers

- `newTestApp()` — Creates Fiber app with mocked services
- `newTestService()` — Creates service with mocked dependencies
- `newTestConfig()` — Creates test config
- `jsonBody()` — Creates JSON request body
- `readBody()` — Reads and unmarshals response body
- `captureLogger()` — Captures logger output to bytes.Buffer
- `newAuthTestHandler()` — Creates auth handler with test config

### Coverage

- No coverage tooling configured by default
- Run with: `go test -cover ./...`
- No enforced coverage thresholds

---

## Runtime & Tooling Preferences

### Required Runtime

- **Go**: 1.26.4+ (module version)
- **Node.js**: Required for tmp/ utility scripts (smoke tests, Mermaid rendering)
- **Package Manager**: Go modules (go mod)
- **OS**: Windows 10 / WSL2 / Linux

### Package Manager

- Go modules only (`go mod tidy`, `go mod download`)
- No vendoring
- Node.js dependencies in `tmp/package.json` for utility scripts

### Tooling Constraints

- **Shell**: Git Bash (MSYS2) on Windows. Use Git-MSYS2-compatible commands, paths, and quoting
- **Paths**: Windows-style paths; use Git Bash-compatible forms
- **Avoid**: Linux-only tools or pure PowerShell unless requested
- **TMPDIR**: Must be appuser-owned in Docker (for Chromium temp files)
- **Browser Timeout**: Minimum 60s for cold starts (`BROWSER_TIMEOUT=60s`)

### Docker Runtime

- Multi-stage build: `golang:1.26.4-alpine` → `alpine:3.20`
- Runtime includes Chromium, nss, freetype, harfbuzz, font-noto
- Non-root `appuser` execution
- `cap_drop ALL`, `cap_add SYS_ADMIN` security hardening
- `tmpfs 512M`, `shm_size 1gb` for Chromium
- Healthcheck on `/api/v1/health`

---

## Important Notes

### Known Inconsistencies

1. **Response Envelopes**: Inconsistent shapes across handlers
   - `response.Success()` → `{status, data, message}`
   - `response.Error()` → `{status, message, errors}` (no trace_id)
   - `fibererror.New()` → `{status, message, trace_id, errors}`
   - `eval_handler.StudentList` → `{status, data}` (missing message)
   - `auth_handler` → returns HTML, not JSON

2. **Login Endpoint**: Returns HTTP 200 for both credential success and failure (distinguished by `data.success` boolean)

3. **Before msgpack migration**: Response envelopes should be unified

### go-rod Eval Rules

- **MUST use arrow functions**, NOT function declarations
- Correct: `page.Eval(`() => document.querySelector('input').value`)`
- Wrong: `page.Eval(`function() { return ... }`)`

### Session Management

- Always `defer session.Close()` after obtaining a session
- Always `defer sessionMgr.Stop()` on application shutdown
- Sessions are keyed by NPM with per-NPM locks
- TTL-based eviction with background cleanup loop

### NPM Validation Rules

- Digits only (`^[0-9]+$`)
- 8–12 characters long
- Implemented in `internal/interfaces/http/handler/validateNPM`

### Image Handling

- HTTP gzip does NOT compress JPEGs — use image-level compression
- Photos are cached with TTL-based eviction in `internal/infrastructure/photocache/`

### Git Conventions

- `.gitignore` excludes: binaries, `.env`, IDE files, OS files, vendor, logs, `.codegraph`, `.omp`, `.superpowers`
- `AGENTS.md` is gitignored — use `git add -f AGENTS.md` to track it

### Documentation Status

- ✅ Comprehensive README.md
- ✅ 16 API docs in `docs/api/`
- ✅ Architecture diagrams in `docs/architecture/`
- ❌ No CLAUDE.md or .cursorrules
- ❌ No OpenAPI/Swagger specs
- ❌ No CI/CD pipelines configured
- ❌ No linting configuration (.golangci.yml)

---

## Quick Reference

```bash
# Start development server
go run cmd/server/main.go

# Run all tests
go test ./...

# Build for production
go build -o bin/server.exe cmd/server/main.go

# Docker build and run
docker build -t lonceng_unman_be . && docker compose up -d

# View logs
docker compose logs -f lonceng-api

# Health check
curl http://localhost:3000/api/v1/health
```
