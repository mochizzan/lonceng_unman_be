# Foundation Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the existing Fiber v3 clean-architecture scaffold into a production-grade foundation by adding interface boundaries, custom error handling, request ID tracing, config validation, and structured logging — no database.

**Architecture:** Existing layers (`cmd/`, `internal/domain`, `internal/application`, `internal/interfaces`, `internal/infrastructure`, `internal/config`) stay in place. Each task upgrades one layer with minimal cross-cutting changes. Services become interfaces, handlers depend on those interfaces, a central error handler sanitizes all errors, request IDs flow through every log line, and `log/slog` replaces raw `log.Printf`.

**Tech Stack:** Go 1.26.4, Fiber v3.4.0, `log/slog` (stdlib), `github.com/google/uuid` (already in go.mod for request IDs), `github.com/gofiber/fiber/v3/middleware/requestid`.

---

## File Structure (target state)

```
lonceng_unman_be/
├── cmd/server/main.go                        # MODIFY — structured logger, error handler, DI wiring
├── internal/
│   ├── config/
│   │   ├── config.go                         # MODIFY — add Validate()
│   │   └── config_test.go                    # CREATE — config validation tests
│   ├── domain/
│   │   └── entity/
│   │       └── health.go                     # UNCHANGED
│   ├── apperror/
│   │   ├── apperror.go                       # CREATE — AppError type (shared across layers)
│   │   └── apperror_test.go                  # CREATE — AppError unit tests
│   ├── application/
│   │   └── service/
│   │       ├── health_service.go             # MODIFY — extract HealthChecker interface
│   │       └── health_service_test.go        # CREATE — service unit tests
│   ├── interfaces/
│   │   └── http/
│   │       ├── handler/
│   │       │   └── health_handler.go         # MODIFY — depend on interface, use apperror.AppError
│   │       ├── router/
│   │       │   └── router.go                 # UNCHANGED (interface injected from main)
│   │       └── response/
│   │           └── response.go               # UNCHANGED
│   └── infrastructure/
│       ├── middleware/
│       │   └── middleware.go                 # MODIFY — add requestid middleware
│       └── fibererror/
│           └── handler.go                    # CREATE — Fiber ErrorHandler (uses apperror + requestid)
│       └── fibererror/
│           └── handler_test.go               # CREATE — ErrorHandler unit tests
├── go.mod                                    # MODIFY (via go mod tidy)
├── go.sum                                    # MODIFY (via go mod tidy)
└── docs/superpowers/plans/
    └── 2026-08-05-foundation-hardening.md    # THIS FILE
```

### Dependency Diagram (Clean Architecture)

```
cmd/server/main.go
    ├── internal/config              (no deps)
    ├── internal/infrastructure/logger      (no deps)
    ├── internal/infrastructure/middleware   → config
    ├── internal/apperror                   → fiber/v3 (status codes only)
    ├── internal/infrastructure/fibererror  → apperror, requestid, slog
    ├── internal/application/service        → config, domain/entity
    └── internal/interfaces/http/handler    → application/service, apperror, response
```

**Key rule:** Handlers depend on `apperror` (shared type) and `application/service` (interface). They NEVER depend on `infrastructure/`. The `infrastructure/fibererror` handler is only wired in `cmd/server/main.go`.

---

## Global Constraints

- Go ≥ 1.25 (Fiber v3 requirement) — project uses 1.26.4
- Fiber v3 import path: `github.com/gofiber/fiber/v3` — never v2
- No database, no GORM, no postgres driver
- No external logging library — use stdlib `log/slog`
- No hardcoding — all config from env vars with fallback defaults
- Every handler returns errors through the central error handler — never `c.SendStatus()` directly
- Services are interfaces — handlers never import concrete service types
- `apperror.AppError` is the shared error type — handlers construct it, `fibererror` handler reads it
- All new code must compile: `go build ./...` must pass after each task

---

## Task 1: Verify go.sum Integrity

**Files:**
- Modify: `go.mod` (if tidied)
- Modify: `go.sum` (if tidied)

**Interfaces:**
- Consumes: existing `go.mod` with all current dependencies
- Produces: verified, consistent `go.mod` + `go.sum`

- [ ] **Step 1: Run go mod tidy**

```bash
go mod tidy
```

Expected: completes with no output (clean). If it prints dependency changes, those are additions/removals to align `go.mod` with actual imports.

- [ ] **Step 2: Verify build still passes**

```bash
go build ./...
```

Expected: no output (clean compile).

- [ ] **Step 3: Verify go.sum is non-empty and consistent**

```bash
go mod verify
```

Expected: `all modules verified` — confirms every module in `go.sum` matches its expected hash.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: verify and tidy module dependencies"
```

---

## Task 2: Add Structured Logger (`log/slog`)

**Files:**
- Create: `internal/infrastructure/logger/logger.go`
- Create: `internal/infrastructure/logger/logger_test.go`
- Modify: `cmd/server/main.go` — replace `log` with `slog`

**Interfaces:**
- Consumes: `config.AppConfig` (env name from `cfg.App.Env`)
- Produces: `*slog.Logger` — used by main.go for startup/shutdown logs; available to all packages via `slog.Default()`

- [ ] **Step 1: Create logger package**

Create `internal/infrastructure/logger/logger.go`:

```go
package logger

import (
	"log/slog"
	"os"
)

// New creates a structured logger configured for the given environment.
// In development it logs at DEBUG level with human-readable output.
// In production it logs at INFO level as JSON.
func New(env string) *slog.Logger {
	var handler slog.Handler

	if env == "development" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return slog.New(handler)
}
```

- [ ] **Step 2: Write tests for logger**

Create `internal/infrastructure/logger/logger_test.go`:

```go
package logger_test

import (
	"log/slog"
	"testing"

	"lonceng_unman_be/internal/infrastructure/logger"
)

func TestNew_DevelopmentReturnsTextHandler(t *testing.T) {
	l := logger.New("development")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	// TextHandler produces human-readable output (not JSON)
	// We verify by checking the handler type name via String()
	handler := l.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	// Verify log level is DEBUG in development
	// slog.Logger doesn't expose level directly, but handler.Options does
	opts := handler.(*slog.TextHandler).Options()
	if opts.Level != slog.LevelDebug {
		t.Errorf("expected DEBUG level in development, got %v", opts.Level)
	}
}

func TestNew_ProductionReturnsJSONHandler(t *testing.T) {
	l := logger.New("production")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	handler := l.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	// Verify log level is INFO in production
	opts := handler.(*slog.JSONHandler).Options()
	if opts.Level != slog.LevelInfo {
		t.Errorf("expected INFO level in production, got %v", opts.Level)
	}
}

func TestNew_AllEnvironmentsReturnLogger(t *testing.T) {
	envs := []string{"development", "staging", "production", "testing"}
	for _, env := range envs {
		l := logger.New(env)
		if l == nil {
			t.Errorf("expected non-nil logger for env %q", env)
		}
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/infrastructure/logger/ -v
```

Expected: all 3 tests PASS. Tests verify actual handler types and log levels, not just non-nil.

- [ ] **Step 4: Update main.go to use structured logger**

Replace the entire `cmd/server/main.go`:

```go
package main

import (
	"log/slog"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/logger"
	"lonceng_unman_be/internal/infrastructure/middleware"
	"lonceng_unman_be/internal/interfaces/http/handler"
	"lonceng_unman_be/internal/interfaces/http/router"

	"github.com/gofiber/fiber/v3"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Initialize structured logger and set as global default
	log := logger.New(cfg.App.Env)
	slog.SetDefault(log)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: cfg.App.Name,
	})

	// Register middleware (recover → requestid → logger → cors)
	middleware.Register(app, cfg.CORS)

	// Wire application services
	healthService := service.NewHealthService(cfg.App)

	// Wire HTTP handlers
	healthHandler := handler.NewHealthHandler(healthService)

	// Register routes
	router.Setup(app, healthHandler)

	// Start server
	log.Info("starting server",
		"app", cfg.App.Name,
		"addr", cfg.Addr(),
		"env", cfg.App.Env,
	)
	if err := app.Listen(cfg.Addr()); err != nil {
		log.Error("server error", "err", err)
	}
}
```

Note: `requestid.New()` is NOT added here — it will be consolidated into `middleware.Register` in Task 4. The middleware stack order is: recover → requestid → logger → cors.

- [ ] **Step 5: Run build**

```bash
go build ./...
```

Expected: no output (clean compile).

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/logger/ cmd/server/main.go
git commit -m "feat: add structured logger with slog, replace log.Printf in main"
```

---

## Task 3: AppError Type (Shared Domain Error)

**Files:**
- Create: `internal/apperror/apperror.go`
- Create: `internal/apperror/apperror_test.go`

**Interfaces:**
- Consumes: Fiber status code constants (import `github.com/gofiber/fiber/v3` for status codes only — no framework logic)
- Produces: `AppError` type — used by all handlers to construct errors; consumed by `fibererror` handler (Task 4) to read them

- [ ] **Step 1: Create AppError type**

Create `internal/apperror/apperror.go`:

```go
package apperror

import "github.com/gofiber/fiber/v3"

// AppError is a structured error that separates public messages (safe to send
// to clients) from internal details (logged server-side only, never exposed).
//
// Usage in handlers:
//
//	return apperror.NotFound("user not found", err)
//	return apperror.BadRequest("name is required")
//	return apperror.Internal("database connection failed", err)
type AppError struct {
	StatusCode int
	PublicMsg  string
	Internal   error
}

// Error implements the error interface. Returns the public message.
func (e *AppError) Error() string {
	return e.PublicMsg
}

// Unwrap allows errors.Is / errors.As to reach the internal error.
func (e *AppError) Unwrap() error {
	return e.Internal
}

// StatusCodeInt returns the HTTP status code as an int (for Fiber).
func (e *AppError) StatusCodeInt() int {
	return e.StatusCode
}

// --- Constructors ---

// NotFound creates a 404 error.
func NotFound(msg string, internal error) *AppError {
	return &AppError{
		StatusCode: fiber.StatusNotFound,
		PublicMsg:  msg,
		Internal:   internal,
	}
}

// BadRequest creates a 400 error.
func BadRequest(msg string) *AppError {
	return &AppError{
		StatusCode: fiber.StatusBadRequest,
		PublicMsg:  msg,
	}
}

// Unauthorized creates a 401 error.
func Unauthorized(msg string) *AppError {
	return &AppError{
		StatusCode: fiber.StatusUnauthorized,
		PublicMsg:  msg,
	}
}

// Forbidden creates a 403 error.
func Forbidden(msg string) *AppError {
	return &AppError{
		StatusCode: fiber.StatusForbidden,
		PublicMsg:  msg,
	}
}

// Internal creates a 500 error with an internal cause that is logged but not exposed.
func Internal(msg string, internal error) *AppError {
	return &AppError{
		StatusCode: fiber.StatusInternalServerError,
		PublicMsg:  msg,
		Internal:   internal,
	}
}
```

- [ ] **Step 2: Write tests for AppError**

Create `internal/apperror/apperror_test.go`:

```go
package apperror_test

import (
	"errors"
	"testing"

	"lonceng_unman_be/internal/apperror"

	"github.com/gofiber/fiber/v3"
)

func TestNotFound(t *testing.T) {
	cause := errors.New("db: no rows")
	err := apperror.NotFound("user not found", cause)

	if err.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404, got %d", err.StatusCode)
	}
	if err.Error() != "user not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "user not found")
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is should find the internal cause")
	}
}

func TestBadRequest(t *testing.T) {
	err := apperror.BadRequest("invalid input")
	if err.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", err.StatusCode)
	}
	if err.Error() != "invalid input" {
		t.Errorf("Error() = %q, want %q", err.Error(), "invalid input")
	}
}

func TestUnauthorized(t *testing.T) {
	err := apperror.Unauthorized("missing token")
	if err.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", err.StatusCode)
	}
}

func TestForbidden(t *testing.T) {
	err := apperror.Forbidden("insufficient permissions")
	if err.StatusCode != fiber.StatusForbidden {
		t.Errorf("expected status 403, got %d", err.StatusCode)
	}
}

func TestInternal(t *testing.T) {
	cause := errors.New("connection refused")
	err := apperror.Internal("service unavailable", cause)

	if err.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", err.StatusCode)
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is should find the internal cause")
	}
	if err.Unwrap() != cause {
		t.Fatal("Unwrap should return the internal cause")
	}
}

func TestErrorsAs_AppError(t *testing.T) {
	err := apperror.NotFound("not found", nil)
	var target *apperror.AppError
	if !errors.As(err, &target) {
		t.Fatal("errors.As should extract *AppError")
	}
	if target.StatusCode != fiber.StatusNotFound {
		t.Errorf("extracted AppError has wrong status: %d", target.StatusCode)
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/apperror/ -v
```

Expected: all 6 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/apperror/
git commit -m "feat: add AppError shared error type for structured error handling"
```

---

## Task 4: Custom Fiber ErrorHandler + Request ID Middleware

**Files:**
- Create: `internal/infrastructure/fibererror/handler.go`
- Create: `internal/infrastructure/fibererror/handler_test.go`
- Modify: `internal/infrastructure/middleware/middleware.go` — add requestid.New()
- Modify: `cmd/server/main.go` — wire ErrorHandler into fiber.Config

**Interfaces:**
- Consumes: `apperror.AppError` (from Task 3), `requestid.FromContext(c)` (from requestid middleware)
- Produces: `fiber.ErrorHandler` function — registered once on `fiber.Config.ErrorHandler`

- [ ] **Step 1: Create Fiber error handler**

Create `internal/infrastructure/fibererror/handler.go`:

```go
package fibererror

import (
	"errors"
	"log/slog"

	"lonceng_unman_be/internal/apperror"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// apiResponse is the standard error JSON envelope returned to clients.
type apiResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// New returns a Fiber ErrorHandler that:
//   - Extracts apperror.AppError or *fiber.Error for controlled status codes/messages
//   - Logs the full internal error server-side via slog (never exposed to client)
//   - Sends a sanitized JSON response to the client
//   - Includes the request ID as trace_id when available
func New() fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		message := "An unexpected error occurred"

		var appErr *apperror.AppError
		var fiberErr *fiber.Error

		switch {
		case errors.As(err, &appErr):
			code = appErr.StatusCode
			message = appErr.PublicMsg
			if appErr.Internal != nil {
				slog.Error("request error",
					"method", c.Method(),
					"path", c.Path(),
					"status", code,
					"internal", appErr.Internal,
				)
			}
		case errors.As(err, &fiberErr):
			code = fiberErr.Code
			message = fiberErr.Message
		default:
			slog.Error("unhandled error",
				"method", c.Method(),
				"path", c.Path(),
				"err", err,
			)
		}

		traceID := requestid.FromContext(c)

		return c.Status(code).JSON(apiResponse{
			Status:  "error",
			Message: message,
			TraceID: traceID,
		})
	}
}
```

- [ ] **Step 2: Write tests for ErrorHandler**

Create `internal/infrastructure/fibererror/handler_test.go`:

```go
package fibererror_test

import (
	"errors"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/infrastructure/fibererror"

	"github.com/gofiber/fiber/v3"
)

func TestErrorHandler_AppError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})

	app.Get("/test", func(c fiber.Ctx) error {
		return apperror.BadRequest("name is required")
	})

	// Use Fiber's test method
	resp, err := app.Test(fiber.Request{
		Method: fiber.MethodGet,
		URL:    "/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestErrorHandler_UnhandledError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})

	app.Get("/test", func(c fiber.Ctx) error {
		return errors.New("something broke")
	})

	resp, err := app.Test(fiber.Request{
		Method: fiber.MethodGet,
		URL:    "/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/infrastructure/fibererror/ -v
```

Expected: both tests PASS.

- [ ] **Step 4: Add requestid to middleware.go**

Replace `internal/infrastructure/middleware/middleware.go`:

```go
package middleware

import (
	"strings"

	"lonceng_unman_be/internal/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// Register attaches the standard middleware stack to the Fiber app.
// Order matters: recover must be first, then requestid, then logger.
func Register(app *fiber.App, corsCfg config.CORSConfig) {
	// Recover from panics (must be first)
	app.Use(recover.New())

	// Assign a unique request ID to every request
	app.Use(requestid.New())

	// Request logger (runs after requestid is set, so logs include the ID)
	app.Use(logger.New())

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Split(corsCfg.AllowOrigins, ","),
		AllowMethods: strings.Split(corsCfg.AllowMethods, ","),
		AllowHeaders: strings.Split(corsCfg.AllowHeaders, ","),
	}))
}
```

- [ ] **Step 5: Wire ErrorHandler into main.go**

Update `cmd/server/main.go` — add the ErrorHandler and fibererror import:

```go
package main

import (
	"log/slog"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/infrastructure/logger"
	"lonceng_unman_be/internal/infrastructure/middleware"
	"lonceng_unman_be/internal/interfaces/http/handler"
	"lonceng_unman_be/internal/interfaces/http/router"

	"github.com/gofiber/fiber/v3"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Initialize structured logger and set as global default
	log := logger.New(cfg.App.Env)
	slog.SetDefault(log)

	// Create Fiber app with custom error handler
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: fibererror.New(),
	})

	// Register middleware (recover → requestid → logger → cors)
	middleware.Register(app, cfg.CORS)

	// Wire application services
	healthService := service.NewHealthService(cfg.App)

	// Wire HTTP handlers
	healthHandler := handler.NewHealthHandler(healthService)

	// Register routes
	router.Setup(app, healthHandler)

	// Start server
	log.Info("starting server",
		"app", cfg.App.Name,
		"addr", cfg.Addr(),
		"env", cfg.App.Env,
	)
	if err := app.Listen(cfg.Addr()); err != nil {
		log.Error("server error", "err", err)
	}
}
```

- [ ] **Step 6: Run build**

```bash
go build ./...
```

Expected: clean compile.

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/fibererror/ internal/infrastructure/middleware/middleware.go cmd/server/main.go
git commit -m "feat: add Fiber error handler and requestid middleware, wire into main"
```

---

## Task 5: Interface-Based Service Layer

**Files:**
- Modify: `internal/application/service/health_service.go` — extract `HealthChecker` interface
- Modify: `internal/interfaces/http/handler/health_handler.go` — depend on interface
- Create: `internal/application/service/health_service_test.go`

**Interfaces:**
- Consumes: `config.AppConfig` (unchanged)
- Produces: `HealthChecker` interface — handler depends only on this; later services follow the same pattern

- [ ] **Step 1: Extract interface from health_service.go**

Replace `internal/application/service/health_service.go`:

```go
package service

import (
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
)

// HealthChecker defines the contract for health check operations.
// Handlers depend on this interface, not the concrete implementation.
type HealthChecker interface {
	Check() entity.HealthStatus
}

// healthService implements HealthChecker.
type healthService struct {
	appName string
	env     string
}

// NewHealthService creates a HealthService from application config.
func NewHealthService(cfg config.AppConfig) HealthChecker {
	return &healthService{
		appName: cfg.Name,
		env:     cfg.Env,
	}
}

// Check returns the current health status of the service.
func (s *healthService) Check() entity.HealthStatus {
	return entity.HealthStatus{
		Status:  "ok",
		Service: s.appName,
		Version: s.env,
	}
}
```

- [ ] **Step 2: Update HealthHandler to depend on interface**

Replace `internal/interfaces/http/handler/health_handler.go`:

```go
package handler

import (
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// HealthHandler handles HTTP requests for health checks.
type HealthHandler struct {
	healthService service.HealthChecker
}

// NewHealthHandler creates a HealthHandler with its dependencies.
func NewHealthHandler(healthService service.HealthChecker) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

// Check handles GET /api/v1/health and returns the service health status.
func (h *HealthHandler) Check(c fiber.Ctx) error {
	status := h.healthService.Check()
	return response.Success(c, fiber.StatusOK, status, "Service is healthy")
}
```

- [ ] **Step 3: Write tests for HealthService**

Create `internal/application/service/health_service_test.go`:

```go
package service_test

import (
	"testing"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
)

func TestHealthService_Check(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-service",
		Env:  "testing",
	}

	svc := service.NewHealthService(cfg)
	status := svc.Check()

	if status.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", status.Status)
	}
	if status.Service != "test-service" {
		t.Errorf("expected service 'test-service', got %q", status.Service)
	}
	if status.Version != "testing" {
		t.Errorf("expected version 'testing', got %q", status.Version)
	}
}

func TestHealthService_ImplementsInterface(t *testing.T) {
	cfg := config.AppConfig{Name: "x", Env: "x"}
	var _ service.HealthChecker = service.NewHealthService(cfg)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/application/service/ -v
```

Expected: PASS — both tests pass.

- [ ] **Step 5: Run build**

```bash
go build ./...
```

Expected: clean compile. `main.go` already calls `service.NewHealthService(cfg.App)` which now returns `HealthChecker` — no caller changes needed.

- [ ] **Step 6: Commit**

```bash
git add internal/application/service/ internal/interfaces/http/handler/
git commit -m "feat: extract HealthChecker interface, decouple handler from concrete service"
```

---

## Task 6: Config Validation

**Files:**
- Modify: `internal/config/config.go` — add `Validate()` method with concrete validation rules
- Create: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing (validates its own fields)
- Produces: validated `Config` — main.go can trust all fields after `New()` returns

- [ ] **Step 1: Add validation logic to config.go**

Replace `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	App  AppConfig
	CORS CORSConfig
}

// AppConfig holds server-related configuration.
type AppConfig struct {
	Name string
	Env  string
	Port string
	Host string
}

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowOrigins string
	AllowMethods string
	AllowHeaders string
}

// New loads configuration from environment variables and validates it.
// It attempts to load a .env file first; if missing, it reads from the OS env only.
func New() (*Config, error) {
	// Load .env file if present (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "lonceng_unman_be"),
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "3000"),
			Host: getEnv("APP_HOST", "0.0.0.0"),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv("CORS_ALLOW_ORIGINS", "*"),
			AllowMethods: getEnv("CORS_ALLOW_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS"),
			AllowHeaders: getEnv("CORS_ALLOW_HEADERS", "Content-Type,Authorization"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// Validate checks that all configuration values are within acceptable ranges.
func (c *Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("APP_NAME must not be empty")
	}

	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[c.App.Env] {
		return fmt.Errorf("APP_ENV must be one of: development, staging, production; got %q", c.App.Env)
	}

	port, err := strconv.Atoi(c.App.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("APP_PORT must be a valid port number (1-65535); got %q", c.App.Port)
	}

	if c.App.Host == "" {
		return fmt.Errorf("APP_HOST must not be empty")
	}

	return nil
}

// Addr returns the listen address in "host:port" format.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%s", c.App.Host, c.App.Port)
}

// IsDevelopment returns true when running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// getEnv reads an environment variable and returns a fallback if unset or empty.
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
```

- [ ] **Step 2: Write config validation tests**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"testing"

	"lonceng_unman_be/internal/config"
)

// NOTE: config.New() calls godotenv.Load() which loads the project .env file.
// t.Setenv() values are set BEFORE godotenv.Load() runs, and godotenv does NOT
// override existing env vars. So t.Setenv() values always win.

func TestNew_ValidConfig(t *testing.T) {
	t.Setenv("APP_NAME", "test")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("APP_HOST", "127.0.0.1")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.Name != "test" {
		t.Errorf("expected name 'test', got %q", cfg.App.Name)
	}
	if cfg.Addr() != "127.0.0.1:8080" {
		t.Errorf("expected addr '127.0.0.1:8080', got %q", cfg.Addr())
	}
}

func TestNew_InvalidPort(t *testing.T) {
	t.Setenv("APP_NAME", "test")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "99999")
	t.Setenv("APP_HOST", "0.0.0.0")

	_, err := config.New()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestNew_InvalidEnv(t *testing.T) {
	t.Setenv("APP_NAME", "test")
	t.Setenv("APP_ENV", "invalid")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")

	_, err := config.New()
	if err == nil {
		t.Fatal("expected error for invalid env")
	}
}

func TestNew_EmptyName(t *testing.T) {
	// t.Setenv sets the env var BEFORE godotenv.Load() runs.
	// godotenv does NOT override existing vars, so APP_NAME stays empty.
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")

	_, err := config.New()
	if err == nil {
		t.Fatal("expected error for empty APP_NAME")
	}
}

func TestNew_WithExplicitValues(t *testing.T) {
	// Verify that explicit env vars are used (not defaults from .env file)
	t.Setenv("APP_NAME", "my-app")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("APP_HOST", "10.0.0.1")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", cfg.App.Name)
	}
	if cfg.App.Env != "staging" {
		t.Errorf("expected env 'staging', got %q", cfg.App.Env)
	}
	if cfg.App.Port != "9090" {
		t.Errorf("expected port '9090', got %q", cfg.App.Port)
	}
	if cfg.App.Host != "10.0.0.1" {
		t.Errorf("expected host '10.0.0.1', got %q", cfg.App.Host)
	}
}

func TestIsDevelopment(t *testing.T) {
	t.Setenv("APP_NAME", "x")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsDevelopment() {
		t.Error("expected IsDevelopment() to return true")
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/config/ -v
```

Expected: all 6 tests PASS. Note: `config.New()` calls `godotenv.Load()` which loads the project `.env` file. `t.Setenv()` values are set BEFORE `godotenv.Load()` runs, and godotenv does NOT override existing env vars, so `t.Setenv()` values always win.

- [ ] **Step 4: Run build**

```bash
go build ./...
```

Expected: clean compile.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config validation with concrete rules and test suite"
```

---

## Task 7: End-to-End Smoke Test

**Files:**
- No file changes — verification only

**Interfaces:**
- Consumes: all tasks 1-6
- Produces: confirmed working server with all improvements

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -v
```

Expected: all tests from apperror, config, logger, fibererror, and service packages PASS.

- [ ] **Step 2: Build the binary**

```bash
go build -o ./bin/server ./cmd/server
```

Expected: `./bin/server` created, no compile errors.

- [ ] **Step 3: Run the server and hit the health endpoint**

```bash
./bin/server &
sleep 2
curl -s http://localhost:3000/api/v1/health | python -m json.tool
```

Expected output:

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

- [ ] **Step 4: Kill the server**

```bash
kill %1 2>/dev/null || true
```

- [ ] **Step 5: Final commit (if any remaining changes)**

```bash
git status
# If clean, no commit needed
```

---

## Summary of Changes

| Task | What Changes | Files Modified | Files Created |
|---|---|---|---|
| 1 | Verify go.sum | `go.mod`, `go.sum` | — |
| 2 | Structured logger | `cmd/server/main.go` | `logger/logger.go`, `logger/logger_test.go` |
| 3 | AppError type | — | `apperror/apperror.go`, `apperror/apperror_test.go` |
| 4 | Error handler + requestid | `middleware/middleware.go`, `cmd/server/main.go` | `fibererror/handler.go`, `fibererror/handler_test.go` |
| 5 | Interface-based service | `health_service.go`, `health_handler.go` | `health_service_test.go` |
| 6 | Config validation | `config.go` | `config_test.go` |
| 7 | Smoke test | — | — |

**Total: 4 files modified, 8 files created, 0 files deleted**

### Clean Architecture Compliance

| Principle | How It's Enforced |
|---|---|
| **Dependency Rule** | `interfaces/` depends on `application/` + `apperror/`. Never on `infrastructure/`. |
| **Interface Segregation** | `HealthChecker` interface in `application/service/` — handler depends only on the contract. |
| **Single Responsibility** | `apperror` = error types only. `fibererror` = Fiber HTTP error handler. `logger` = logging setup. |
| **No Framework Leakage** | `apperror` imports Fiber only for status code constants — zero framework logic. Handler uses Fiber's `c` (that's its job). |
| **Testability** | Every package has `_test.go`. Service tests don't need Fiber. Handler tests use `app.Test()`. Config tests use `t.Setenv()`. |
| **Maintainability** | Adding a new error type = add one function to `apperror/`. Adding a new service = add interface + implementation in `application/service/`. Adding a new handler = add struct in `interfaces/http/handler/`. |
