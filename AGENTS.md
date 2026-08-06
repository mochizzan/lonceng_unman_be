# Project Agent Instructions — lonceng_unman_be

> **IMPORTANT:** All agents MUST read this file before making any changes to the codebase.

---

## Project Context

| Field | Value |
|-------|-------|
| **Project Year** | 2026 |
| **Project Type** | REST API Backend |
| **Language** | Go 1.26.4 |
| **Framework** | Fiber v3.4.0 |
| **Architecture** | Clean Architecture (4-layer) |
| **Development OS** | Windows 10 Pro (NOT Unix/Linux/Mac) |
| **Shell / Bash** | Git Bash (MSYS2) — NOT WSL, NOT Ubuntu, NOT PowerShell |
| **Module Path** | `lonceng_unman_be` |

---

## Mandatory Rules

These rules are **non-negotiable**. Violating any of them is a bug.

### 1. Line-Aware Code Changes

**NEVER modify code based on assumed line numbers.** Before any edit:

1. **Read the target file** using the `read` tool with the exact file path.
2. **Identify the exact `old_text`** — the unique text string you want to replace.
3. **Use the `edit` tool** with `old_text` matching the actual file content.
4. **Verify the edit succeeded** by reading the file again after modification.

```
# WRONG: Assuming line 12 has specific content
# RIGHT: Read file → find exact text → edit using old_text
```

If the file content does not match your expectation, **STOP and report the discrepancy** — do NOT guess or force the edit.

### 2. Read Before Edit

**NEVER edit a file without reading it first** in the current conversation. The `read` tool loads the file into context. If you haven't read it, you don't know what's in it.

### 3. Ask When Ambiguous

**When in doubt, ASK the user.** Specifically:

- If a prompt or task description is unclear or incomplete
- If multiple valid interpretations exist
- If a change could affect things not mentioned in the request
- If you're unsure about the scope of a change

Do NOT assume. Do NOT guess. Do NOT proceed with incomplete information.

### 4. LSP Linting After Every Change

**After modifying ANY `.go` file**, run LSP diagnostics to catch errors:

```
lsp(action="diagnostics", file="<modified-file-path>")
```

If LSP reports errors, fix them before proceeding. If LSP is unavailable, run `go vet ./...` as a fallback.

### 5. Commit on Current Branch — Every Change

**Every code or file change MUST be committed immediately** on the current branch. No exceptions.

```
git add <changed-files>
git commit -m "type(scope): description"
```

Commit message format: **Conventional Commits** (`feat`, `fix`, `chore`, `refactor`, `docs`, `test`).

### 6. NEVER Merge Without Explicit Command

**NEVER run `git merge`** unless the user explicitly asks you to merge. Merging is a user-directed action only.

### 7. NEVER Create Branches Without Explicit Command

**NEVER run `git checkout -b` or `git switch -c`** to create a new branch unless the user explicitly asks you to create one. Stay on the current branch.

### 8. Language and Communication

**ALL sentences must be in English.** Every comment, commit message, doc comment, and output must be:

- **Detailed** — not vague or hand-wavy
- **Clear** — unambiguous, specific
- **To the point** — no filler, no fluff, no excessive politeness

### 9. No Database — Ask Before Any DB Integration

**This project has NO database.** It is a pure REST API with in-memory or file-based data.

If any task, feature, or request **requires database integration** (PostgreSQL, MySQL, SQLite, MongoDB, Redis, or any data store):

1. **STOP immediately** — do NOT proceed with the implementation.
2. **ASK the user** — explain what requires a database and why.
3. **WAIT for explicit approval** — only proceed after the user confirms and specifies which database to use.
4. **Discuss schema and migration strategy** — before writing any DB code.

This applies to:
- Adding database drivers or ORMs (GORM, sqlx, etc.)
- Creating models/entities with DB tags
- Writing repository patterns
- Adding migration files
- Configuring connection strings
- Any feature that implies persistent storage

### 10. Sub-Agent Delegation — Full Detail Required

**When spawning, delegating, or dispatching sub-agents**, the parent agent MUST provide **complete, detailed instructions from A to Z**. Brief or vague instructions are FORBIDDEN.

Every sub-agent task MUST include:

1. **Context** — project name, working directory, Go version, current HEAD commit
2. **Input** — exact file paths to read, exact content to reference
3. **Process** — step-by-step actions (read file → identify line → edit → verify)
4. **Output** — expected result, commit message, test output, report path
5. **Constraints** — what NOT to do (no new branches, no merge, no assumed line numbers)

**WRONG (brief):**
```
"Add validation to config.go"
```

**RIGHT (complete):**
```
"Project: lonceng_unman_be, Go 1.26.4, HEAD: abc1234

Task: Add port validation to config.go

Steps:
1. Read internal/config/config.go
2. In the Validate() method, after the APP_ENV validation block (around line 45-50)
3. Add port range check: port must be between 1 and 65535
4. Use strconv.Atoi to parse cfg.App.Port
5. Return fmt.Errorf("config validation: invalid APP_PORT: must be 1-65535") if out of range
6. Run go test ./tests/config/ -v to verify
7. Commit with message: feat(config): add port range validation

Do NOT:
- Create new files
- Modify any other validation rules
- Assume line numbers without reading the file first"
```

If the parent agent cannot provide this level of detail, it should **do the work inline** instead of delegating.

### 11. Use Codegraph MCP for Code Exploration

**RECOMMENDED:** Before exploring unfamiliar code, use the `codegraph_explore` MCP tool or `codegraph explore "<query>"` shell command. Codegraph provides symbol-level code navigation including call paths and blast radius — far faster than grep/read loops.

When to use:
- Finding where a function/symbol is defined, called, or referenced
- Understanding call paths between symbols (including dynamic dispatch)
- Getting blast radius of a proposed change
- Navigating unfamiliar packages or cross-layer dependencies

How to use:
- MCP tool: `codegraph_explore` with a natural language query or symbol name
- Shell: `codegraph explore "symbolName"` or `codegraph explore "how does X work"`

Only works in repositories with a `.codegraph/` directory at the repo root.

---

## Architecture Overview

```
cmd/server/main.go                    ← Composition root (DI wiring ONLY)
internal/
├── config/config.go                  ← Environment-based config + validation (17 env vars)
├── apperror/apperror.go              ← AppError type + sentinels (cross-cutting)
├── domain/
│   ├── entity/
│   │   ├── health.go                 ← Pure data models (zero dependencies)
│   │   ├── lms.go                    ← LoginRequest, LoginResult
│   │   ├── document.go               ← KRS/KHS download requests/results, KHSFilename
│   │   └── extraction.go             ← KRSExtraction, KHSExtraction, ExtractionResult
│   └── port/
│       ├── browser.go                ← BrowserSession interface
│       └── session.go                ← SessionManager interface
├── application/service/
│   ├── health_service.go             ← HealthChecker interface + impl
│   ├── lms_service.go                ← LMSLogin + LMSDocumentService interfaces + impl
│   └── extraction_service.go         ← ExtractionService interface + impl
├── interfaces/http/
│   ├── handler/
│   │   ├── health_handler.go         ← HealthHandler (→ service.HealthChecker)
│   │   ├── lms_handler.go            ← LMSHandler (→ service.LMSLogin)
│   │   ├── document_handler.go       ← DocumentHandler (→ service.LMSDocumentService)
│   │   └── extraction_handler.go     ← ExtractionHandler (→ service.ExtractionService)
│   ├── router/router.go              ← 9 routes under /api/v1
│   └── response/response.go          ← Uniform JSON response envelope
└── infrastructure/
    ├── middleware/middleware.go       ← recover → requestid → logger → cors
    ├── logger/logger.go              ← slog-based structured logging
    ├── fibererror/handler.go         ← Global error handler
    ├── browser/
    │   ├── browser.go                ← go-rod Browser wrapper
    │   ├── selectors.go              ← CSS selectors + URL paths for LMS
    │   └── download.go               ← PDF download via JS fetch()
    ├── session/
    │   ├── manager.go                ← In-memory session cache with TTL
    │   └── session.go                ← rodSession: thread-safe BrowserSession impl
    └── extractor/
        ├── pdf_reader.go             ← PDF text extraction with positional data
        ├── krs_parser.go             ← Structured KRS extraction from PDF
        ├── khs_parser.go             ← Structured KHS extraction from PDF
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

> **Note:** Two known dependency violations exist where the application layer imports
> infrastructure packages directly (lms_service → infra/browser for URL paths/selectors,
> extraction_service → infra/extractor for PDF parsing). These should ideally be inverted
> via domain/port interfaces but are currently tolerated for pragmatic reasons.

### Request Lifecycle

```
Client Request
  → recover middleware (panic protection)
  → requestid middleware (inject trace_id)
  → logger middleware (request logging)
  → cors middleware (CORS headers)
  → router dispatch (match /api/v1/<path>)
  → handler method (e.g., HealthHandler.Check)
  → service method (e.g., HealthChecker.Check)
  → entity response (domain data)
  → response.Success() envelope
  → JSON output: {"status":"success","data":{...},"message":"..."}
```

Error path:
```
handler returns apperror.* → fibererror ErrorHandler
  → errors.As cascade: AppError → fiber.Error → generic
  → logs internal details via slog.Error (NEVER exposed to client)
  → returns sanitized JSON: {"status":"error","message":"...","trace_id":"..."}
```

---

## Code Conventions

### File Naming

- **Go source files**: `snake_case.go` (e.g., `health_service.go`, `apperror.go`)
- **Test files**: `tests/<package>/<name>_test.go` (external test packages, separate from source)
- **Package names**: single lowercase word (`config`, `entity`, `service`, `handler`, `response`)

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
cfg.App.Name    // from APP_NAME (default: "lonceng_unman_be")
cfg.App.Env     // from APP_ENV (default: "development")
cfg.App.Port    // from APP_PORT (default: "3000")
cfg.App.Host    // from APP_HOST (default: "0.0.0.0")
```

### Testing

- Tests live in `tests/<package>/` directory — **NOT** beside source files
- Use stdlib `testing` only — no testify, no gomock
- Test naming: `TestFunctionName_Behavior` (e.g., `TestNotFound`, `TestNew_ValidConfig`)
- Compile-time interface check pattern:

```go
var _ service.HealthChecker = service.NewHealthService(cfg)
```
- Test packages use same package name (not `_test` suffix) but are functionally external (separate directory)

### Middleware Order

**Strict order** — DO NOT change without understanding implications:

```
recover → requestid → logger → cors
```

- `recover` MUST be first (catches panics from all subsequent middleware)
- `requestid` generates trace_id used by error handler and logger
- `logger` needs the requestid for correlation
- `cors` is last — headers applied after all processing

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_NAME` | `lonceng_unman_be` | Application name |
| `APP_ENV` | `development` | Environment: development, staging, production |
| `APP_PORT` | `3000` | Server port (validated: 1-65535) |
| `APP_HOST` | `0.0.0.0` | Bind address |
| `LMS_BASE_URL` | `https://elearning.universitasmandiri.ac.id` | LMS base URL |
| `LMS_DASHBOARD_URL` | `https://elearning.universitasmandiri.ac.id/admin/` | LMS dashboard URL after login |
| `BROWSER_HEADLESS` | `true` | Run Chrome headless |
| `BROWSER_TIMEOUT` | `30s` | Overall browser operation timeout |
| `ACTION_TIMEOUT` | `10s` | Per-action timeout (click, fill, etc.) |
| `DOWNLOAD_DIR` | `./downloads` | PDF download directory |
| `EXTRACT_DIR` | `./extracted` | Extraction cache directory |
| `SESSION_TTL` | `15m` | Session cache TTL before expiry |
| `MAX_SESSIONS` | `10` | Maximum concurrent browser sessions |
| `MAX_BODY_SIZE` | `1MB` | Max HTTP request body size |
| `CORS_ALLOW_ORIGINS` | `*` | CORS allowed origins |
| `CORS_ALLOW_METHODS` | `GET,POST,PUT,PATCH,DELETE,OPTIONS` | CORS allowed methods |
| `CORS_ALLOW_HEADERS` | `Content-Type,Authorization` | CORS allowed headers |

---

## Git Conventions

- **Commit format**: Conventional Commits — `feat(scope): description`
- **Branch naming**: `feat/<feature-name>`
- **Commit frequency**: Every code change, no matter how small
- **Never force push** to shared branches
- **Never merge** without explicit user command

---

## Common Commands

```bash
# Run dev server
go run ./cmd/server

# Run all tests (from tests/ directory)
go test ./...

# Run specific test
go test ./tests/apperror/ -v

# Build binary for current platform
go build -o ./bin/server.exe ./cmd/server

# Tidy dependencies
go mod tidy

# Vet for common issues
go vet ./...

# Explore code with codegraph (if .codegraph/ exists)
codegraph explore "DownloadPDF"
```

---

## Where to Look

| I want to... | Look at... |
|--------------|-----------|
| Add an API endpoint | `internal/interfaces/http/router/router.go` + create handler |
| Add a handler | `internal/interfaces/http/handler/` |
| Add business logic | `internal/application/service/` (define interface + impl) |
| Add a domain model | `internal/domain/entity/` |
| Add a port interface | `internal/domain/port/` |
| Add middleware | `internal/infrastructure/middleware/middleware.go` |
| Add error type | `internal/apperror/apperror.go` |
| Change config | `internal/config/config.go` + `.env` |
| Add tests | `tests/<package-name>/` |
| Wire new dependencies | `cmd/server/main.go` |
| Change error handling | `internal/infrastructure/fibererror/handler.go` |
| Add browser automation | `internal/infrastructure/browser/` |
| Add session management | `internal/infrastructure/session/` |
| Add PDF extraction | `internal/infrastructure/extractor/` |
| Add LMS login endpoint | `internal/interfaces/http/handler/lms_handler.go` |
| Add document download | `internal/interfaces/http/handler/document_handler.go` + `service/lms_service.go` |
| Add PDF extraction | `internal/interfaces/http/handler/extraction_handler.go` + `service/extraction_service.go` |
