# RESTful API Backend — Go Fiber v3 + Clean Architecture Setup

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold a production-ready RESTful API backend using Go Fiber v3 with Clean Architecture, environment-based configuration, and zero hardcoding.

**Architecture:** Three-layer Clean Architecture — `domain` (entities + business rules), `application` (use cases / services), `interfaces` (HTTP handlers + Fiber routes), and `infrastructure` (config, external adapters). Every dependency points inward. Config is loaded from environment variables via `joho/godotenv`. Responses use a standardized JSON envelope.

**Tech Stack:** Go 1.26+, Fiber v3.4.0, `joho/godotenv` for `.env` loading, `go-playground/validator/v10` for request validation, `google/uuid` for ID generation, `slog` (stdlib) for structured logging.

## Global Constraints

- Go version: `>= 1.25` (Fiber v3 minimum)
- Module path: `lonceng_unman_be`
- No database — all state is in-memory or mock
- No hardcoded values — every configurable value comes from environment variables
- Response format: standardized JSON envelope `{"status": "success|error", "data": ..., "message": "...", "meta": ...}`
- Commit messages: Conventional Commits format (`feat:`, `fix:`, `chore:`, etc.)
- Each task ends with a passing `go build ./...` and `go vet ./...`

---

## File Structure

```
lonceng_unman_be/
├── cmd/
│   └── server/
│       └── main.go                  # Entry point — wires everything, starts Fiber
├── internal/
│   ├── config/
│   │   └── config.go                # Loads env vars into typed Config struct
│   ├── domain/
│   │   └── entity/
│   │       └── health.go            # Health check entity (response model)
│   ├── application/
│   │   └── service/
│   │       └── health_service.go    # Health check use case
│   ├── interfaces/
│   │   └── http/
│   │       ├── handler/
│   │       │   └── health_handler.go    # HTTP handler for health check
│   │       ├── router/
│   │       │   └── router.go           # Registers all routes on the Fiber app
│   │       └── response/
│   │           └── response.go         # Standardized JSON response helpers
│   └── infrastructure/
│       └── middleware/
│           └── middleware.go           # CORS, Logger, Recover middleware stack
├── .env.example                        # Template for required env vars
├── .gitignore                          # Ignores .env, binaries, etc.
├── go.mod
└── go.sum
```

---

### Task 1: Initialize Go module and install dependencies

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `.env.example`

**Interfaces:**
- Consumes: (nothing — first task)
- Produces: initialized Go module at `lonceng_unman_be`, ready for dependency installation

- [ ] **Step 1: Initialize Go module**

```bash
cd D:/TUGAS-AKHIR/app/v2/lonceng_unman_be
go mod init lonceng_unman_be
```

- [ ] **Step 2: Install core dependencies**

```bash
go get github.com/gofiber/fiber/v3
go get github.com/joho/godotenv
go get github.com/go-playground/validator/v10
go get github.com/google/uuid
```

- [ ] **Step 3: Create `.gitignore`**

```gitignore
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/bin/
/server

# Test binary
*.test

# Output of go coverage
*.out

# Environment
.env

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

- [ ] **Step 4: Create `.env.example`**

```env
# Server
APP_NAME=lonceng_unman_be
APP_ENV=development
APP_PORT=3000
APP_HOST=0.0.0.0

# CORS
CORS_ALLOW_ORIGINS=*
CORS_ALLOW_METHODS=GET,POST,PUT,PATCH,DELETE,OPTIONS
CORS_ALLOW_HEADERS=Content-Type,Authorization
```

- [ ] **Step 5: Verify module initializes correctly**

```bash
go mod tidy
go build ./...
```

Expected: no errors, `go.sum` created.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: initialize Go module with Fiber v3 and dependencies"
```

---

### Task 2: Config package — load environment variables

**Files:**
- Create: `internal/config/config.go`
- Create: `.env`

**Interfaces:**
- Consumes: (nothing — standalone)
- Produces: `config.New() (*Config, error)` — returns a populated `Config` struct. Fields: `App` (Name, Env, Port, Host), `CORS` (AllowOrigins, AllowMethods, AllowHeaders)

- [ ] **Step 1: Create `.env` with development defaults**

```env
APP_NAME=lonceng_unman_be
APP_ENV=development
APP_PORT=3000
APP_HOST=0.0.0.0

CORS_ALLOW_ORIGINS=*
CORS_ALLOW_METHODS=GET,POST,PUT,PATCH,DELETE,OPTIONS
CORS_ALLOW_HEADERS=Content-Type,Authorization
```

- [ ] **Step 2: Write the config package**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

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

// New loads configuration from environment variables.
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

	return cfg, nil
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

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/config/...
go vet ./internal/config/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go .env .env.example
git commit -m "feat(config): add environment-based configuration loader"
```

---

### Task 3: Standardized JSON response helpers

**Files:**
- Create: `internal/interfaces/http/response/response.go`

**Interfaces:**
- Consumes: (nothing — standalone utility)
- Produces: `Success(c fiber.Ctx, status int, data any, message string) error`, `Error(c fiber.Ctx, status int, message string, errors ...any) error`

- [ ] **Step 1: Write the response package**

Create `internal/interfaces/http/response/response.go`:

```go
package response

import (
	"github.com/gofiber/fiber/v3"
)

// APIResponse is the standard JSON envelope returned by all endpoints.
type APIResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
	Errors  any    `json:"errors,omitempty"`
}

// Success sends a success response with the given status code.
func Success(c fiber.Ctx, status int, data any, message string) error {
	return c.Status(status).JSON(APIResponse{
		Status:  "success",
		Data:    data,
		Message: message,
	})
}

// Error sends an error response with the given status code.
func Error(c fiber.Ctx, status int, message string, errors ...any) error {
	resp := APIResponse{
		Status:  "error",
		Message: message,
	}
	if len(errors) > 0 && errors[0] != nil {
		resp.Errors = errors[0]
	}
	return c.Status(status).JSON(resp)
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/interfaces/http/response/...
go vet ./internal/interfaces/http/response/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/interfaces/http/response/
git commit -m "feat(response): add standardized JSON envelope helpers"
```

---

### Task 4: Domain entity — Health

**Files:**
- Create: `internal/domain/entity/health.go`

**Interfaces:**
- Consumes: (nothing — pure data model)
- Produces: `HealthStatus` struct with fields `Status string`, `Service string`, `Version string`

- [ ] **Step 1: Write the entity**

Create `internal/domain/entity/health.go`:

```go
package entity

// HealthStatus represents the health state of the service.
type HealthStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/domain/entity/...
go vet ./internal/domain/entity/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/entity/
git commit -m "feat(domain): add HealthStatus entity"
```

---

### Task 5: Application service — Health check use case

**Files:**
- Create: `internal/application/service/health_service.go`

**Interfaces:**
- Consumes: `config.AppConfig` (for service name + version info)
- Produces: `HealthService.Check() entity.HealthStatus`

- [ ] **Step 1: Write the service**

Create `internal/application/service/health_service.go`:

```go
package service

import (
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
)

// HealthService implements health check business logic.
type HealthService struct {
	appName string
	env     string
}

// NewHealthService creates a HealthService from application config.
func NewHealthService(cfg config.AppConfig) *HealthService {
	return &HealthService{
		appName: cfg.Name,
		env:     cfg.Env,
	}
}

// Check returns the current health status of the service.
func (s *HealthService) Check() entity.HealthStatus {
	return entity.HealthStatus{
		Status:  "ok",
		Service: s.appName,
		Version: s.env,
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/application/service/...
go vet ./internal/application/service/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/application/service/
git commit -m "feat(application): add HealthService use case"
```

---

### Task 6: HTTP handler — Health check endpoint

**Files:**
- Create: `internal/interfaces/http/handler/health_handler.go`

**Interfaces:**
- Consumes: `*service.HealthService` (injected via constructor)
- Produces: `HealthHandler.Check(c fiber.Ctx) error` — returns JSON `{"status":"success","data":{...},"message":"Service is healthy"}`

- [ ] **Step 1: Write the handler**

Create `internal/interfaces/http/handler/health_handler.go`:

```go
package handler

import (
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// HealthHandler handles HTTP requests for health checks.
type HealthHandler struct {
	healthService *service.HealthService
}

// NewHealthHandler creates a HealthHandler with its dependencies.
func NewHealthHandler(healthService *service.HealthService) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

// Check handles GET /api/v1/health and returns the service health status.
func (h *HealthHandler) Check(c fiber.Ctx) error {
	status := h.healthService.Check()
	return response.Success(c, fiber.StatusOK, status, "Service is healthy")
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/interfaces/http/handler/...
go vet ./internal/interfaces/http/handler/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/interfaces/http/handler/
git commit -m "feat(handler): add HealthHandler for /api/v1/health"
```

---

### Task 7: Router — register all routes

**Files:**
- Create: `internal/interfaces/http/router/router.go`

**Interfaces:**
- Consumes: `*fiber.App`, `*handler.HealthHandler`
- Produces: `Setup(app *fiber.App, healthHandler *handler.HealthHandler)` — registers all routes under `/api/v1`

- [ ] **Step 1: Write the router**

Create `internal/interfaces/http/router/router.go`:

```go
package router

import (
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// Setup registers all application routes on the Fiber app.
func Setup(app *fiber.App, healthHandler *handler.HealthHandler) {
	v1 := app.Group("/api/v1")

	// Health
	v1.Get("/health", healthHandler.Check)
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/interfaces/http/router/...
go vet ./internal/interfaces/http/router/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/interfaces/http/router/
git commit -m "feat(router): register v1 route group with health endpoint"
```

---

### Task 8: Infrastructure middleware stack

**Files:**
- Create: `internal/infrastructure/middleware/middleware.go`

**Interfaces:**
- Consumes: `*fiber.App`, `config.CORSConfig`
- Produces: `Register(app *fiber.App, corsCfg config.CORSConfig)` — attaches Logger, Recover, CORS to the app

- [ ] **Step 1: Write the middleware registrar**

Create `internal/infrastructure/middleware/middleware.go`:

```go
package middleware

import (
	"lonceng_unman_be/internal/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// Register attaches the standard middleware stack to the Fiber app.
func Register(app *fiber.App, corsCfg config.CORSConfig) {
	// Recover from panics
	app.Use(recover.New())

	// Request logger
	app.Use(logger.New())

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsCfg.AllowOrigins,
		AllowMethods: corsCfg.AllowMethods,
		AllowHeaders: corsCfg.AllowHeaders,
	}))
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/infrastructure/middleware/...
go vet ./internal/infrastructure/middleware/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/middleware/
git commit -m "feat(middleware): register Logger, Recover, and CORS stack"
```

---

### Task 9: Entry point — wire everything and start server

**Files:**
- Create: `cmd/server/main.go`

**Interfaces:**
- Consumes: `config.New()`, `middleware.Register()`, `service.NewHealthService()`, `handler.NewHealthHandler()`, `router.Setup()`
- Produces: a running Fiber server on the configured host:port

- [ ] **Step 1: Write the entry point**

Create `cmd/server/main.go`:

```go
package main

import (
	"log"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/middleware"
	"lonceng_unman_be/internal/interfaces/http/handler"
	"lonceng_unman_be/internal/interfaces/http/router"

	"github.com/gofiber/fiber/v3"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: cfg.App.Name,
	})

	// Register middleware
	middleware.Register(app, cfg.CORS)

	// Wire application services
	healthService := service.NewHealthService(cfg.App)

	// Wire HTTP handlers
	healthHandler := handler.NewHealthHandler(healthService)

	// Register routes
	router.Setup(app, healthHandler)

	// Start server
	log.Printf("Starting %s on %s [%s]", cfg.App.Name, cfg.Addr(), cfg.App.Env)
	if err := app.Listen(cfg.Addr()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

- [ ] **Step 2: Verify full build**

```bash
go build ./...
go vet ./...
```

Expected: no errors, binary produced.

- [ ] **Step 3: Run and smoke test**

```bash
go run ./cmd/server/
```

In another terminal:

```bash
curl http://localhost:3000/api/v1/health
```

Expected response:

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

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): wire entry point and start Fiber server"
```

---

### Task 10: Final cleanup and verification

**Files:**
- No new files — verify all existing files

**Interfaces:**
- Consumes: (all previous tasks)
- Produces: clean build, all vet checks pass, project ready for feature development

- [ ] **Step 1: Run full build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 2: Run the server and test endpoints**

```bash
go run ./cmd/server/
```

Test endpoints:

```bash
# Health check
curl -s http://localhost:3000/api/v1/health | jq .

# 404 test
curl -s http://localhost:3000/api/v1/nonexistent | jq .
```

Expected:
- `/api/v1/health` → `200` with success envelope
- `/api/v1/nonexistent` → `404` with Fiber default or error envelope

- [ ] **Step 3: Commit final state**

```bash
git add -A
git commit -m "chore: verify clean build and all endpoints functional"
```

---

## Summary of deliverables

| Task | Deliverable | Endpoint |
|------|------------|----------|
| 1 | Go module + dependencies | — |
| 2 | Config from `.env` | — |
| 3 | JSON response helpers | — |
| 4 | HealthStatus entity | — |
| 5 | HealthService use case | — |
| 6 | HealthHandler | — |
| 7 | Router with `/api/v1` group | — |
| 8 | Middleware stack (Logger, Recover, CORS) | — |
| 9 | Entry point — full wiring | `GET /api/v1/health` |
| 10 | Final verification | All endpoints |
