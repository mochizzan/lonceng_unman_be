# Repository Guidelines

## Project Overview

**lonceng_unman_be** — Go/Fiber v3 Clean Architecture backend for LMS automation at Universitas Mandiri (`elearning.universitasmandiri.ac.id`). Automates headless-browser login, KRS/KHS PDF download & extraction, student-profile scraping (55+ fields), and eval ground-truth workflows via REST API.

- **Stack**: Go 1.26.4, Fiber v3.4.0, go-rod v0.116.2 (headless Chromium), gopdf v0.9.5, godotenv
- **Deploy**: Windows / WSL2 / Docker / cloudflared → `ghcr.io/mochizzan/lonceng_unman_be:latest`

## Architecture & Data Flow

### Clean Architecture Layers

```
cmd/server/                              ← Composition root: DI wiring, entry point
  └─ internal/interfaces/http/           ← Fiber handlers, router, response envelopes
       └─ internal/application/service/  ← Use cases & orchestration (DTOs, business flow)
            └─ internal/domain/          ← Entities + port interfaces (business contracts)
                 └─ internal/infrastructure/ ← Concrete impls: browser, session, PDF, cache
```

| Layer | Path | Responsibility |
|---|---|---|
| **Entry** | `cmd/server/main.go` | Load config, init `slog` logger, wire deps via constructor injection, register routes, `app.Listen` |
| **Interfaces** | `internal/interfaces/http/` | Fiber handlers, `router.Setup`, response envelopes |
| **Application** | `internal/application/service/` | 9 services: `health`, `lms`, `document`, `extraction`, `student_profile`, `eval`, `eval_compare`, `eval_save`, `eval_normalize` |
| **Domain** | `internal/domain/entity/` + `internal/domain/port/` | Entities (KRS, KHS, StudentProfile 55+ fields) + port interfaces |
| **Infrastructure** | `internal/infrastructure/` | `browser/`, `session/`, `extractor/`, `photocache/`, `evalstore/`, `logger/`, `middleware/`, `fibererror/`, `auth/` |

### Key Domain Ports

Defined in `internal/domain/port/`; infrastructure implements them (dependency inversion):

- `BrowserSession` (`browser.go`) — `Navigate`, `Eval`, `ElementAttribute`, `ElementExists`, `DownloadPDF`, `DownloadImage`, `Close`
- `SessionManager` (`session.go`) — `GetOrCreate(npm,password)`, `Close`, `CloseAll` (per-NPM authenticated Chromium sessions)
- `PDFParser` / `ExtractionCache` (`extraction.go`) — `ParseKRS`, `ParseKHS`, `MarshalToJSON` / `Get`, `Set`, `Exists`, `GetModTime`, `Invalidate`
- `EvalStore` / `EvalService` (`eval.go`) — ground-truth JSON storage + precision/recall/F1
- `StudentProfileScraper` (`student_profile_scraper.go`) — bulk JS eval for profile fields
- `LMSConfig` (`lms_config.go`) — typed config accessor for LMS URLs/timeouts

### Request Data Flow

```
HTTP Request
  → Fiber middleware: recover → requestid → logger (slog+trace_id) → compress → CORS
  → router.Setup (internal/interfaces/http/router/router.go)
  → Handler (validates NPM via validateNPM, extracts params)
    → Application Service (orchestrates use case)
      → Domain Port interface (abstract contract)
        → Infrastructure impl (go-rod browser, PDF parser, cache, session mgr)
    → Response envelope (response.Success / response.Error / fibererror)
  → HTTP Response
```

Special flows:
- **Session**: per-NPM locks prevent duplicate creation; TTL eviction (24h soft, 2h hard lifetime + 5m grace) + `activeCount`/`pageCount` under `sess.mu`; `touchLastUsed` on every op; `replacePage` on timeout; DNS pre-flight (`CheckDNS` 3× retry) before browser ops; `defer session.Close()` always.
- **Cold start**: Tier-1/2 tuning via env knobs (`PHOTO_RENDER_WAIT` 3s, `PHOTO_RETRY_WAIT` 1s, `SCRAPE_FORM_WAIT` 2s, `BROWSER_LAUNCH_TIMEOUT` 60s) replaces hardcoded sleeps; `Page()` retry `[0,1s,2s]`.
- **Documents**: per-NPM browser-phase lock, `QueryEscape` for `tahun_ajaran`, `ClassifyDocumentError` → 503/401/500, KHS file local-first fallback.

## Key Directories

| Directory | Purpose |
|---|---|
| `cmd/server/` | Main entry + DI wiring (`main.go`, `parserAdapter`) |
| `cmd/extractor/` | CLI for raw PDF text extraction (debug) |
| `cmd/batch-extract/` `cmd/batch-gt/` | Batch extraction / ground-truth generation CLIs |
| `internal/config/` | Env loading, validation, `filepath.Abs` path resolution |
| `internal/domain/entity/` | Business entities (KRS/KHS, StudentProfile 55+ fields, eval) |
| `internal/domain/port/` | Port interfaces (browser, session, extraction, eval, lms_config) |
| `internal/application/service/` | Use-case orchestration (9 services) |
| `internal/infrastructure/browser/` | go-rod wrapper (`browser.go` with `Connect`/`ConnectWithProfile`), scraper |
| `internal/infrastructure/session/` | In-memory manager: TTL, per-NPM `sync.Map` locks, `activeCount`/`pageCount`, `touchLastUsed`, `replacePage` |
| `internal/infrastructure/extractor/` | KRS/KHS PDF parsers (gopdf) + `CacheManager` |
| `internal/infrastructure/photocache/` | TTL photo cache with image compression (`MAX_PHOTO_DIMENSION` 300, `PHOTO_QUALITY` 80) |
| `internal/infrastructure/evalstore/` | JSON file-based ground-truth store |
| `internal/infrastructure/middleware/` | recover → requestid → logger → compress → CORS |
| `internal/infrastructure/logger/` | `slog` — text handler (dev/DEBUG) vs JSON (prod/INFO), injectable + `trace_id` |
| `internal/infrastructure/fibererror/` | Global Fiber error handler → `{status, message, trace_id, errors}` |
| `internal/infrastructure/auth/` | Eval dashboard auth + rate limiter |
| `internal/interfaces/http/handler/` | 9 handlers: `health`, `lms`, `document`, `extraction`, `student_profile`, `eval`, `eval_data`, `eval_gt`, `auth` |
| `internal/interfaces/http/router/` | `Setup` — registers `/api/v1/*`, `/eval/*` (auth-protected), static `evalhtml.StaticFS` |
| `internal/interfaces/http/response/` | `Success`/`Error` envelopes |
| `internal/interfaces/http/evalhtml/` | Embedded HTML templates + `StaticFS` for `/eval/static/*` |
| `internal/apperror/` | `AppError` (PublicMsg/Internal), sentinels (`ErrPDFNotFound`), `Classify` helpers (`classify.go`) |
| `tests/` | Top-level, feature-organized tests (mirrors `internal/`, not colocated) |
| `docs/api/` `docs/architecture/` | 12 endpoint docs + architecture/flow diagrams (mermaid) |
| `tmp/` | Utility scripts: smoke tests (`*.mjs`), edge-case probes, Mermaid rendering |
| `profiles/` `downloads/` `extracted/` `eval/ground_truth/` | Runtime data (per-NPM subdirs, bind-mounted in Docker) |

## Development Commands

### Build & Run

```bash
go run cmd/server/main.go                          # dev server (reads .env)
go build -o bin/server.exe cmd/server/main.go     # production binary
./run.cmd                                          # Windows build+run helper
go build -o bin/extractor.exe cmd/extractor/main.go
go build -o bin/batch-extract.exe cmd/batch-extract/main.go
go build -o bin/batch-gt.exe cmd/batch-gt/main.go
```

### Environment

```bash
cp .env.example .env          # 24 vars — see .env.example
# Key vars: APP_NAME/ENV/PORT/HOST, LMS_BASE_URL/DASHBOARD_URL,
# BROWSER_HEADLESS/TIMEOUT (60s), DNS_TIMEOUT (5s), DOWNLOAD_DIR/EXTRACT_DIR/EVAL_DIR/PROFILE_BASE_DIR,
# SESSION_TTL (24h) / MAX_SESSIONS (20), PHOTO_RENDER_WAIT (3s) / PHOTO_RETRY_WAIT (1s) / SCRAPE_FORM_WAIT (2s) / BROWSER_LAUNCH_TIMEOUT (60s),
# PHOTO_CACHE_TTL (15m), MAX_PHOTO_DIMENSION (300), PHOTO_QUALITY (80), MAX_BODY_SIZE (1MB), MAX_PDF_SIZE (50MB), SECRET_KEY, AUTO_LOGIN (forbidden in prod), TZ
```

`internal/config/config.go` resolves `DOWNLOAD_DIR`/`EXTRACT_DIR`/`EVAL_DIR`/`PROFILE_BASE_DIR` to absolute paths via `filepath.Abs` and validates `APP_ENV ∈ {development,staging,production}`.

### Docker

```bash
docker build -t lonceng_unman_be .        # multi-stage: golang:1.26.4-alpine + UPX → alpine:3.20+Chromium (~6MB)
docker compose up -d                      # uses compose.yml (ghcr.io image, env_file .env, bind mounts, tmpfs 512M, shm 1gb)
docker compose logs -f lonceng-api
curl http://localhost:3000/api/v1/health  # healthcheck endpoint (wget in container)
```

Every `/data/*` path needs `mkdir` + `ENV` + bind mount + `env_file` (not `environment` list).

### Lint & Format

```bash
gofmt -w .                    # formatting (no .golangci.yml; no enforced linter in repo)
golangci-lint run ./...       # if installed locally
```

## Code Conventions & Common Patterns

### Dependency Injection

Manual constructor injection in `cmd/server/main.go`, top-down:

```go
sessionMgr := session.NewManager(cfg)
defer sessionMgr.CloseAll()
defer sessionMgr.Stop()

studentProfileSvc := service.NewStudentProfileService(cfg, sessionMgr, scraper, cache, photoCache)
lmsService := service.NewLMSService(cfg, sessionMgr)
```

### Interface Segregation & Naming

- Small focused ports in `internal/domain/port/` (`BrowserSession`, `SessionManager`, `PDFParser`, etc.)
- Files: `snake_case.go` (`lms_service.go`, `khs_parser_test.go`); types/handlers: `PascalCase` (`LMSHandler`, `StudentProfileService`)
- No `testify`/`mockery` in tests (despite `testify` in `go.sum`); manual function-field mocks.

### Error Handling

```go
apperr := apperror.BadRequest("NPM must be 8-12 digits")
apperr := apperror.NotFound("Session not found", err)
apperr := apperror.Internal("browser launch failed", err) // PublicMsg vs Internal separation

var ErrPDFNotFound = errors.New("PDF not found")           // sentinel for flow control
status, msg := apperror.ClassifyLoginResult(body)          // -> 401/503 anti-enumeration
code := apperror.ClassifyDocumentError(err, "")            // -> 503/401/500 (infra/credential/fallback)
```

`ClassifyDocumentError` priority: `*AppError` passthrough → `IsInfrastructureError` (503) → `IsCredentialError` (401, combined phrases only) → 500.  
Fiber errors via `internal/infrastructure/fibererror/handler.go` include `trace_id` (from `requestid`).

### Response Envelopes

```go
response.Success(c, fiber.StatusOK, data, "optional message")   // → {status:"success", data, message, trace_id}
response.Error(c, fiber.StatusBadRequest, "msg", err)           // → {status:"error", message, errors, trace_id}
```

Standard envelope: `APIResponse{Status, Data, Message, TraceID, Errors}`.  
Known inconsistency: `eval_handler.StudentList` omits `message`; `auth_handler` returns HTML; `fibererror` adds `trace_id`.

### Logging

`slog` only — `fmt.Printf` forbidden. Injectable logger + `trace_id`:

```go
logger.Info("session created", "npm", npm, "ttl", ttl)
logger.Error("browser launch failed", "error", err)
```

Dev: text handler, DEBUG. Prod: JSON handler, INFO. Audit via `tmp/log-audit.mjs`.

### NPM Validation

```go
// internal/interfaces/http/handler/validateNPM
// ^[0-9]+$ and 8–12 chars; digits only
```

### go-rod Rules

```go
// MUST use arrow functions — NOT function declarations
page.Eval(`() => document.querySelector('input').value`) // correct
page.Eval(`function(){ return ... }`)                    // WRONG — will fail

// Session lifecycle
session, err := sessionMgr.GetOrCreate(npm, password)
defer session.Close()
defer sessionMgr.Stop() // on shutdown
// Never reuse hung CDP page — replacePage on timeout; Page() retry [0,1s,2s] + DNS 3x
```

### Session Management

- Keyed by NPM, per-NPM `sync.Map` locks, `activeCount`/`pageCount` under `sess.mu`, `touchLastUsed` on every op.
- Persistent profiles via `--user-data-dir` (`PROFILE_BASE_DIR`).
- Background `cleanupLoop` every 1m evicts expired (TTL 24h soft, 2h hard cap + 5m grace).

### PDF Extraction Flow

```go
pdfBytes, _ := session.DownloadPDF(pdfURL)   // 1. download via browser
result, _   := parser.ParseKRS(pdfBytes)     // 2. parse (or ParseKHS)
cache.Set(npm, jsonBytes)                    // 3. cache JSON
```

Filenames via `filepath.Join` + `KHSFilename` for traversal safety; `QueryEscape` for query params; `Total SKS` checks.

### State Management

- No ORM/DB — file-system state (`downloads/`, `extracted/`, `profiles/`, `eval/ground_truth/`) + in-memory session/photo caches.
- `EvalStore` is JSON file-based; eval metrics compute TP/FN/FP/TN, Precision, Recall, F1.
- `DocumentInventory` aggregates Raw/Extracted/GT counts per NPM.

## Important Files

| File | Purpose |
|---|---|
| `cmd/server/main.go` | Entry, DI wiring, `parserAdapter`, route registration, `app.Listen` |
| `cmd/extractor/main.go` | Raw PDF text extraction CLI (debug) |
| `cmd/batch-extract/main.go` `cmd/batch-gt/main.go` | Batch extraction / ground-truth CLIs |
| `.env.example` | 24-var template (cold-start knobs documented inline) |
| `.env` / `.env.test` | Dev / smoke-test env |
| `internal/config/config.go` | `Config` load, `Validate`, `Addr`, `IsDevelopment`, `parseByteSize` |
| `internal/domain/entity/extraction.go` `student_profile.go` | Core entities (KRS/KHS, 55+ profile fields) |
| `internal/domain/port/browser.go` `session.go` `extraction.go` `eval.go` | Port interfaces |
| `internal/application/service/lms_service.go` | LMS login (`(result,httpStatus,err)` + `classifyLoginResult`) |
| `internal/application/service/extraction_service.go` `document_service.go` | PDF orchestration + document download |
| `internal/application/service/student_profile_service.go` | Profile scraping + photo cache |
| `internal/application/service/eval_service.go` `eval_compare.go` `eval_save.go` | Eval metrics + comparison + persistence |
| `internal/infrastructure/browser/browser.go` `scraper.go` | go-rod wrapper + bulk JS eval scraper |
| `internal/infrastructure/session/manager.go` | Session manager (TTL, locks, DNS retry, page replacement) |
| `internal/infrastructure/extractor/krs.go` `khs.go` | PDF parsers |
| `internal/infrastructure/photocache/cache.go` | TTL photo cache with compression |
| `internal/infrastructure/evalstore/evalstore.go` | Ground-truth JSON store |
| `internal/infrastructure/middleware/middleware.go` | recover → requestid → logger → compress → CORS |
| `internal/infrastructure/logger/logger.go` | `slog` factory |
| `internal/infrastructure/fibererror/handler.go` | Global error handler |
| `internal/infrastructure/auth/middleware.go` | Eval auth + limiter |
| `internal/interfaces/http/router/router.go` | `Setup` — all routes, `staticHandler` |
| `internal/interfaces/http/response/response.go` | `Success`/`Error` envelopes |
| `internal/apperror/apperror.go` `classify.go` | `AppError` + classifiers |
| `Dockerfile` | Multi-stage UPX build → `alpine:3.20`+Chromium, `appuser`, `HEALTHCHECK` |
| `compose.yml` | Prod compose (`127.0.0.1:${APP_PORT}:3000`, `dns 8.8.8.8/1.1.1.1`, `shm_size 1gb`, `cap_drop ALL`) |
| `README.md` | Overview, quick start, endpoint table |
| `run.cmd` | Windows build+run helper |

## Runtime/Tooling Preferences

| Concern | Preference |
|---|---|
| **Go** | 1.26.4 (`go.mod` `go 1.26.4`) |
| **Package manager** | Go modules only (`go mod tidy`/`download`); no vendoring |
| **Node.js** | Required for `tmp/` scripts (`node tmp/smoke-test.mjs`, Mermaid rendering) — see `tmp/package.json` |
| **OS** | Windows 10 / WSL2 / Linux; **Shell**: Git Bash (MSYS2) — prefer Git-MSYS2-compatible commands/quoting/paths; avoid Linux-only or pure PowerShell |
| **Paths** | Windows-style, Git Bash-compatible; `TMPDIR` must be `appuser`-owned in Docker (`/data/tmp`) |
| **Browser** | Chromium headless; `BROWSER_TIMEOUT` min 60s (cold starts); `BROWSER_LAUNCH_TIMEOUT` caps `Launch()+Connect()` |
| **Docker** | Multi-stage `golang:1.26.4-alpine` → `alpine:3.20`; non-root `appuser`, `dumb-init`, `cap_drop ALL` + `cap_add SYS_ADMIN`, `tmpfs 512M`, `shm_size 1gb`, `healthcheck /api/v1/health` |
| **Env** | `godotenv` loads `.env`; `compose.yml` uses `env_file: .env` + container overrides for `/data/*` paths |

## Testing & QA

### Framework & Organization

- **Runner**: stdlib `testing` only (no testify/mockery — despite `testify` in `go.sum`, never imported); **HTTP**: Fiber `app.Test()` + `httptest.NewRequest`; **Mocking**: manual function-field structs.
- **Layout**: top-level `tests/` by feature (not colocated), plus one in-package test `internal/infrastructure/evalstore/evalstore_cached_test.go`:

```
tests/
  apperror/apperror_test.go
  config/config_test.go, cold_start_defaults_test.go
  extractor/split_tahun_test.go
  student_profile/handler_test.go, service_test.go
  service/eval_service_test.go, khs_h1_replace_test.go, khs_file_service_test.go, lms_login_classifier_test.go, ...
  handler/auth_handler_test.go, eval_handler_test.go, eval_data_handler_test.go, khs_file_handler_test.go
  infrastructure/auth/middleware_test.go, browser/page_retry_test.go, session/check_dns_retry_test.go
  session/manager_test.go
  logger/logger_test.go
  logging_audit_test.go, logging_regression_test.go
```

### Running Tests

```bash
go test ./...                                      # all
go test ./tests/student_profile/...                # per-feature
go test -v ./...                                   # verbose
go test -cover ./...                               # coverage (no threshold enforced)
go test -run TestLoginClassifier ./tests/service/  # single test

# Smoke tests (require .env.test or running server on :3000)
node tmp/smoke-test.mjs
go run tmp/extraction-smoke-test.go
node tmp/log-audit.mjs                 # logging audit (forbid fmt.Printf)
node tmp/edge-case-stress.mjs          # stress / burst probes
```

### Patterns

**Manual mock** (function-field struct):
```go
type mockStudentProfileService struct {
    scrapeFn func(npm string) (*entity.StudentProfile, error)
    getFn    func(npm string) (*entity.StudentProfile, error)
}
func (m *mockStudentProfileService) Scrape(npm string) (*entity.StudentProfile, error) { return m.scrapeFn(npm) }
```

**Test app helper**:
```go
app := newTestApp(mockService)
req := httptest.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", jsonBody(payload))
req.Header.Set("Content-Type", "application/json")
resp, _ := app.Test(req)
```

**Table-driven subtests**:
```go
tests := []struct{ name, input, expected string }{{"underscore", "2020_2021", "2020/2021"}}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { /* assert */ })
}
```

**Helpers**: `newTestApp()`, `newTestService()`, `newTestConfig()`, `jsonBody()`, `readBody()`, `captureLogger()`, `newAuthTestHandler()`.

### Coverage

No enforced thresholds. Use `go test -cover ./...` ad hoc. Git history: `fix(session)`, `feat(doc)`, `feat(lms)`, `docs(spec)` — conventional commits on branches `feat/*`, `fix/*`, `main`, `v1`.

## Quick Reference

```bash
go run cmd/server/main.go                          # dev
go test ./...                                      # tests
go build -o bin/server.exe cmd/server/main.go     # build
docker build -t lonceng_unman_be . && docker compose up -d
docker compose logs -f lonceng-api
curl http://localhost:3000/api/v1/health
gofmt -w .                                         # format
```

