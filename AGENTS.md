# Repository Guidelines

> **IMPORTANT:** All agents MUST read this file before making any changes to the codebase.

---

## Project Overview

| Field | Value |
|---|---|
| **Project Year** | 2026 |
| **Project Type** | REST API Backend |
| **Language** | Go 1.26.4 |
| **Framework** | Fiber v3.4.0 |
| **PDF Library** | razvandimescu/gopdf v0.9.5 |
| **Browser Automation** | go-rod v0.116.2 |
| **Architecture** | Clean Architecture (4-layer) |
| **Development OS** | Windows 10 Pro (NOT Unix/Linux/Mac) |
| **Shell / Bash** | Git Bash (MSYS2) — NOT WSL, NOT PowerShell, NOT cmd |
| **Module Path** | `lonceng_unman_be` |

**Lonceng Unman Backend** is a RESTful API for automated LMS (Learning Management System) document extraction. It logs into an academic LMS via headless Chrome, downloads KRS (course registration) and KHS (grade report) PDFs, parses them into structured JSON using position-based PDF extraction, and caches results. No database — all data is in-memory or file-based.

---

## Architecture & Data Flow

### Clean Architecture (4-Layer)

```
cmd/server/main.go                    ← Composition root (DI wiring ONLY)
internal/
├── config/config.go                  ← Environment-based config + validation
├── apperror/apperror.go              ← AppError type + sentinels (cross-cutting)
├── domain/                           ← Layer 1: Pure business logic (zero dependencies)
│   ├── entity/                       ← Data models (no external imports)
│   │   ├── health.go                 ← HealthStatus
│   │   ├── lms.go                    ← LoginRequest, LoginResult
│   │   ├── document.go               ← KRS/KHS download requests/results, KHSFilename
│   │   └── extraction.go             ← KRSExtraction, KHSExtraction, ExtractionResult
│   └── port/                         ← Interfaces for outer layers
│       ├── browser.go                ← BrowserSession interface
│       ├── session.go                ← SessionManager interface
│       ├── extraction.go             ← PDFParser, ExtractionCache interfaces
│       └── lms_config.go             ← LMSConfig interface (CSS selectors, URL paths)
├── application/service/              ← Layer 2: Business logic implementations
│   ├── health_service.go             ← HealthChecker interface + impl
│   ├── lms_service.go                ← LMSLogin + LMSDocumentService interfaces + impl
│   └── extraction_service.go         ← ExtractionService interface + impl
├── interfaces/http/                  ← Layer 3: HTTP transport
│   ├── handler/                      ← Request handlers (depend on service interfaces)
│   │   ├── health_handler.go         ← HealthHandler (→ service.HealthChecker)
│   │   ├── lms_handler.go            ← LMSHandler (→ service.LMSLogin)
│   │   ├── document_handler.go       ← DocumentHandler (→ service.LMSDocumentService)
│   │   └── extraction_handler.go     ← ExtractionHandler (→ service.ExtractionService)
│   ├── router/router.go              ← 9 routes under /api/v1
│   └── response/response.go          ← Uniform JSON response envelope
└── infrastructure/                   ← Layer 4: External implementations
    ├── middleware/middleware.go       ← recover → requestid → logger → cors
    ├── logger/logger.go              ← slog-based structured logging
    ├── fibererror/handler.go         ← Global error handler
    ├── browser/
    │   ├── browser.go                ← go-rod Browser wrapper
    │   ├── selectors.go              ← CSS selectors + URL paths for LMS
    │   └── download.go               ← PDF download via JS fetch()
    ├── session/
    │   ├── manager.go                ← In-memory session cache with TTL + Chrome profile persistence
    │   └── session.go                ← rodSession: thread-safe BrowserSession impl
    └── extractor/
        ├── pdf_reader.go             ← PDF text extraction with positional data
        ├── parser_common.go          ← Shared constants + helpers (KRS & KHS)
        ├── krs_parser.go             ← Structured KRS extraction from PDF (position-based)
        ├── khs_parser.go             ← Structured KHS extraction from PDF (position-based)
        └── cache.go                  ← File-based extraction cache + MarshalJSON
tests/                                ← External test packages (mirrors internal/)
```

### Dependency Rule

Dependencies flow **inward only**:

```
domain/entity ← application/service ← interfaces/http ← infrastructure
                         ↑
                      config
```

- **Handlers** depend on service **interfaces**, NEVER on concrete types
- **Handlers** NEVER import `infrastructure/`
- **Infrastructure** is only wired in `cmd/server/main.go`
- **apperror** imports Fiber ONLY for HTTP status code constants
- **`parserAdapter`** in `cmd/server/main.go` bridges application→infrastructure by implementing `port.PDFParser` and delegating to `extractor` package functions

### Request Lifecycle

```
Client Request
  → recover middleware (panic protection)
  → requestid middleware (inject trace_id)
  → logger middleware (request logging)
  → cors middleware (CORS headers)
  → router dispatch (POST /api/v1/<path>, except GET /health)
  → handler method (parse JSON body via c.Bind().JSON)
  → service method (business logic)
  → entity response (domain data)
  → response.Success() envelope
  → JSON output: {"status":"success","data":{...},"message":"..."}
```

### Error Path

```
handler returns apperror.* → fibererror ErrorHandler
  → errors.As cascade: AppError → fiber.Error → generic
  → logs internal details via slog.Error (NEVER exposed to client)
  → returns sanitized JSON: {"status":"error","message":"...","trace_id":"..."}
```

---

## Key Directories

| Directory | Purpose |
|---|---|
| `cmd/server/` | Application entry point, dependency wiring |
| `internal/config/` | Environment-based config loading and validation |
| `internal/apperror/` | Shared error types (cross-cutting concern) |
| `internal/domain/entity/` | Pure data models (zero dependencies) |
| `internal/domain/port/` | Interface contracts for outer layers |
| `internal/application/service/` | Business logic implementations |
| `internal/interfaces/http/handler/` | HTTP request handlers |
| `internal/interfaces/http/router/` | Route definitions |
| `internal/interfaces/http/response/` | JSON response envelope |
| `internal/infrastructure/middleware/` | Fiber middleware stack |
| `internal/infrastructure/logger/` | Structured logging (slog) |
| `internal/infrastructure/fibererror/` | Global error handler |
| `internal/infrastructure/browser/` | go-rod browser automation |
| `internal/infrastructure/session/` | Session management + Chrome profiles |
| `internal/infrastructure/extractor/` | PDF extraction + caching |
| `tests/` | External test packages (mirrors internal/) |
| `downloads/` | Runtime PDF download directory (gitignored) |
| `extracted/` | Runtime extraction cache directory (gitignored) |
| `profiles/` | Persistent Chrome profile directories (gitignored) |
| `docs/` | Design documents and implementation plans |

---

## Development Commands

### Build & Run

```bash
# Run development server
go run ./cmd/server

# Build binary for current platform
go build -o ./bin/server.exe ./cmd/server

# Windows batch script (builds and runs)
run.cmd
```

### Test

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./tests/config/ -v
go test ./tests/apperror/ -v
go test ./tests/extractor/ -v
go test ./tests/service/ -v
go test ./tests/fibererror/ -v

# Run with verbose output
go test ./tests/... -v

# Run specific test function
go test ./tests/config/ -run TestLoadConfig -v
```

### Lint & Vet

```bash
# Static analysis
go vet ./...

# Tidy dependencies
go mod tidy
```

### Code Exploration

```bash
# Using codegraph (if .codegraph/ exists)
codegraph explore "DownloadPDF"
codegraph explore "BrowserSession"
```

---

## Code Conventions & Common Patterns

### File Naming

| Convention | Example |
|---|---|
| Go source files | `snake_case.go` (`health_service.go`, `apperror.go`) |
| Test files | `tests/<package>/<name>_test.go` (external packages) |
| Package names | Single lowercase word (`config`, `entity`, `service`, `handler`, `response`) |
| Parser files | `parser_common.go` for shared constants/helpers |

### Error Handling

```go
// Correct — use apperror constructors
return apperror.BadRequest("name is required")
return apperror.NotFound("user not found", err)      // with internal cause
return apperror.Internal("database failed", err)      // 500 with cause
return apperror.Unauthorized("LMS login failed")     // 401
return apperror.Forbidden("permission denied")        // 403

// WRONG — never do this
c.SendStatus(400)
c.JSON(fiber.Map{"error": "..."})
```

Handlers ALWAYS return `error`. The global `fibererror` handler processes errors.

### Response Format

All endpoints use uniform JSON envelope:

```json
{
  "status": "success|error",
  "data": {},
  "message": "...",
  "errors": {}
}
```

Use helpers:

```go
// Correct — data should be a struct, not a string
response.Success(c, fiber.StatusOK, result, "Service is healthy")
response.Error(c, fiber.StatusBadRequest, "name is required")
```

### Logging

Use `log/slog` only — no external logging libraries:

```go
// Structured key-value pairs
log.Info("starting server", "app", cfg.App.Name, "addr", cfg.Addr())
log.Error("failed to load config", "error", err)
```

- Development: `slog.TextHandler` at `DEBUG` level
- Production: `slog.JSONHandler` at `INFO` level

### Configuration

All config from environment variables with typed defaults. No hardcoding.

```go
cfg.App.Name       // from APP_NAME (default: "lonceng_unman_be")
cfg.App.Env        // from APP_ENV (default: "development")
cfg.App.Port       // from APP_PORT (default: "3000")
cfg.App.Host       // from APP_HOST (default: "0.0.0.0")
cfg.App.DownloadDir  // from DOWNLOAD_DIR (default: "./downloads")
cfg.App.ExtractDir   // from EXTRACT_DIR (default: "./extracted")
```

### Middleware Order

**Strict order** — DO NOT change without understanding implications:

```
recover → requestid → logger → cors
```

- `recover` MUST be first (catches panics from all subsequent middleware)
- `requestid` generates trace_id used by error handler and logger
- `logger` needs the requestid for correlation
- `cors` is last — headers applied after all processing

### Dependency Injection

Manual constructor injection in `cmd/server/main.go`. No DI framework.

```go
// Composition root pattern
healthService := service.NewHealthService(cfg)
healthHandler := handler.NewHealthHandler(healthService)

// Interface-based — handlers depend on interfaces, not concrete types
type HealthHandler struct {
    service service.HealthChecker  // interface, not *healthService
}
```

### Fiber v3 API

```go
// Body parsing (Fiber v3 API — NOT v2)
var req entity.LoginRequest
if err := c.Bind().JSON(&req); err != nil { ... }

// Request body access
body := c.Body()  // []byte

// NOT this (v2 API):
// c.BodyParser(&req)  // DEPRECATED in v3
```

### Compile-Time Interface Checks

```go
var _ service.HealthChecker = service.NewHealthService(cfg)
```

---

## Important Files

### Entry Points

| File | Purpose |
|---|---|
| `cmd/server/main.go` | Application entry point, DI wiring, Fiber app setup |
| `internal/interfaces/http/router/router.go` | All route definitions (9 routes) |
| `internal/config/config.go` | Config struct with env loading, validation, typed defaults |

### Configuration

| File | Purpose |
|---|---|
| `.env` | Active environment variables (gitignored) |
| `.env.example` | Template env vars with comments (tracked) |
| `internal/config/config.go` | Config struct, validation, env parsing |
| `go.mod` | Go module definition and dependencies |
| `run.cmd` | Windows build-and-run script |

### Documentation

| File | Purpose |
|---|---|
| `README.md` | Project README (Indonesian) |
| `API.md` | Comprehensive API documentation (English, 2009 lines) |
| `LICENSE` | The Unlicense (public domain) |
| `docs/SESSION_CACHE_RECOMMENDATION.md` | Chrome profile persistence design |
| `docs/superpowers/plans/` | Implementation plans |

---

## Runtime/Tooling Preferences

### Required Runtime

- **Go 1.26.4** — module requires this version
- **Windows 10 Pro** — development environment
- **Git Bash (MSYS2)** — shell for commands (NOT WSL, NOT PowerShell, NOT cmd)
- **Chrome/Chromium** — required for go-rod browser automation

### Dependencies

| Package | Version | Purpose |
|---|---|---|
| `gofiber/fiber/v3` | v3.4.0 | HTTP framework |
| `go-rod/rod` | v0.116.2 | Headless Chrome automation |
| `razvandimescu/gopdf` | v0.9.5 | PDF generation (used for extraction) |
| `joho/godotenv` | v1.5.1 | .env file loading |

### Package Manager

- **Go modules** — standard Go dependency management
- No vendoring — dependencies downloaded to module cache

### Build Tools

- `go build` — standard Go build
- `run.cmd` — Windows batch script for build + run
- No Makefile, no Docker, no golangci-lint config

### Environment Variables (20+)

Key variables from `.env.example`:

```bash
# Server
APP_NAME=lonceng_unman_be
APP_ENV=development
APP_PORT=3000
APP_HOST=0.0.0.0

# LMS
LMS_BASE_URL=https://elearning.universitasmandiri.ac.id
LMS_DASHBOARD_URL=https://elearning.universitasmandiri.ac.id/admin/

# Browser
BROWSER_HEADLESS=true
BROWSER_TIMEOUT=60s
DNS_TIMEOUT=5s

# Paths
DOWNLOAD_DIR=./downloads
EXTRACT_DIR=./extracted
PROFILE_BASE_DIR=./profiles

# Session
SESSION_TTL=15m
MAX_SESSIONS=10

# Limits
MAX_BODY_SIZE=1MB
MAX_PDF_SIZE=50MB

# CORS
CORS_ALLOW_ORIGINS=*
CORS_ALLOW_METHODS=GET,POST,OPTIONS
CORS_ALLOW_HEADERS=Content-Type
```

---

## Testing & QA

### Test Framework

- **stdlib `testing` only** — no testify, no gomock, no external test frameworks
- **Fiber's built-in `app.Test()`** — for HTTP handler integration tests
- **Table-driven tests** — preferred pattern for multiple test cases
- **Subtests** — `t.Run("name", func(t *testing.T) { ... })` for organized cases

### Test Organization

```
tests/
├── config/config_test.go          ← Config loading, validation, env parsing
├── logger/logger_test.go          ← Logger creation per environment
├── apperror/apperror_test.go      ← Error constructors, errors.As/unwrapping
├── service/health_service_test.go ← HealthService.Check() + interface compliance
├── fibererror/handler_test.go     ← Fiber error handler integration tests
├── extractor/
│   ├── parse_header_test.go       ← NormalizeLabel with table-driven subtests
│   ├── khs_parser_test.go         ← KHS parsing with conditional integration
│   └── krs_parser_test.go         ← KRS parsing with conditional integration
internal/
└── infrastructure/extractor/khs_dedup_test.go  ← In-tree dedup regression tests
```

### Test Naming Convention

```go
func TestFunctionName_Behavior(t *testing.T) {
    // Example:
    func TestLoadConfig_ValidEnv(t *testing.T) { ... }
    func TestParseKHS_FileNotFound(t *testing.T) { ... }
    func TestNotFound(t *testing.T) { ... }
    func TestNew_ValidConfig(t *testing.T) { ... }
}
```

### Running Tests

```bash
# All tests
go test ./...

# Specific package with verbose output
go test ./tests/config/ -v
go test ./tests/extractor/ -v

# Specific test function
go test ./tests/config/ -run TestLoadConfig -v

# Skip integration tests (those requiring PDF files)
go test ./tests/extractor/ -run TestParseKHS -v  # Skips if no PDF available
```

### Test Patterns

```go
// Table-driven test example
func TestNormalizeLabel(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"lowercase", "nama", "Nama"},
        {"uppercase", "NAMA", "Nama"},
        {"mixed", "No.", "No."},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := extractor.NormalizeLabel(tt.input)
            if result != tt.expected {
                t.Errorf("NormalizeLabel(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}
```

```go
// Integration test with skip pattern
func TestParseKHS(t *testing.T) {
    pdfPath := "../../downloads/test_khs.pdf"
    if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
        t.Skip("PDF file not found, skipping integration test")
    }
    // ... test implementation
}
```

```go
// Fiber handler test with app.Test()
func TestHealthHandler(t *testing.T) {
    app := fiber.New()
    handler.RegisterRoutes(app, healthHandler)

    req := httptest.NewRequest("GET", "/api/v1/health", nil)
    resp, err := app.Test(req)
    if err != nil {
        t.Fatalf("app.Test() failed: %v", err)
    }

    if resp.StatusCode != fiber.StatusOK {
        t.Errorf("expected status 200, got %d", resp.StatusCode)
    }
}
```

### Coverage Expectations

- **Unit tests** for all service methods and handlers
- **Integration tests** for HTTP endpoints using `app.Test()`
- **Parser tests** with conditional skip (require PDF fixtures)
- **Regression tests** for edge cases (deduplication, boundary conditions)
- Run `go vet ./...` before commits — catches common issues

---

## Git Conventions

### Commit Format

Conventional Commits — `type(scope): description`

```bash
feat(config): add port range validation
fix(extractor): correct KHS column boundary
refactor(session): simplify lock acquisition
test(apperror): add NotFound unwrapping test
docs(api): update endpoint documentation
chore(deps): update go-rod to v0.116.2
```

### Branch Naming

```bash
feat/<feature-name>    # e.g., feat/session-persistence
fix/<bug-description>  # e.g., fix/khs-dedup-error
```

### Commit Rules

- **Every code change** MUST be committed immediately
- **No force push** to shared branches
- **No merge** without explicit user command
- **All code and comments** in English

---

## Common Pitfalls

1. **Fiber v3 API** — Use `c.Bind().JSON()`, NOT `c.BodyParser()` (v2 API)
2. **Error handling** — Always return `apperror.*` from handlers, NEVER `c.SendStatus()`
3. **Response envelope** — Always use `response.Success()` / `response.Error()`, NEVER raw JSON
4. **Logging** — Use `log/slog` only, no external logging libraries
5. **Config** — All values from env vars, no hardcoding
6. **Middleware order** — Strict: recover → requestid → logger → cors
7. **Windows paths** — Use forward slashes in Go code, quote paths with spaces
8. **go-rod** — Use non-Must methods (return errors) in production code

---

## API Endpoints

| Method | Path | Handler | Description |
|---|---|---|---|
| `GET` | `/api/v1/health` | HealthHandler.Check | Health check |
| `POST` | `/api/v1/lms/login` | LMSHandler.Login | LMS login |
| `POST` | `/api/v1/lms/krs/download` | DocumentHandler.DownloadKRS | Download KRS PDF |
| `POST` | `/api/v1/lms/khs/semesters` | DocumentHandler.GetKHSSemesters | Get KHS semesters |
| `POST` | `/api/v1/lms/khs/download` | DocumentHandler.DownloadKHS | Download KHS PDF |
| `POST` | `/api/v1/extraction/krs` | ExtractionHandler.ExtractKRS | Extract KRS from PDF |
| `POST` | `/api/v1/extraction/khs` | ExtractionHandler.ExtractKHS | Extract KHS from PDF |
| `POST` | `/api/v1/data/krs` | ExtractionHandler.GetKRS | Get cached KRS data |
| `POST` | `/api/v1/data/khs` | ExtractionHandler.GetKHS | Get cached KHS data |

---

## Session Management

- **In-memory cache** with configurable TTL (default: 15 minutes)
- **Maximum concurrent sessions** configurable (default: 10)
- **Chrome profile persistence** — sessions survive server restarts via `PROFILE_BASE_DIR`
- **DNS pre-flight** — validates LMS reachability before browser connection
- **Background cleanup** — expired sessions removed automatically
- **Per-NPM locking** — prevents concurrent operations on same student

---

## License

**The Unlicense** — Public domain dedication

> **Note:** README badge says MIT but LICENSE file is The Unlicense (inconsistency).
