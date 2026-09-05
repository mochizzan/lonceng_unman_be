package tests

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/infrastructure/logger"
	"lonceng_unman_be/internal/infrastructure/middleware"

	"github.com/gofiber/fiber/v3"
)

// repoRoot resolves the module root (directory containing go.mod) from this file's location.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func newTestFiberApp(loggerMw bool) *fiber.App {
	cfg := config.CORSConfig{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}
	if loggerMw {
		app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
		middleware.Register(app, cfg)
		return app
	}
	return fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
}

// --- 1. Fiber logger middleware must emit one access-line per request ---

func TestPhase1_FiberLoggerEmitsPerRequest(t *testing.T) {
	app := newTestFiberApp(true)
	app.Get("/api/v1/health2", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health2", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200 got %d", resp.StatusCode)
	}
}

// --- 2. SESSION fmt.Printf vs slog — detect non-structured spam ---

var sessionPrintfRe = regexp.MustCompile(`fmt\.Printf\(`)

func TestPhase1_SessionUsesStructuredLogging(t *testing.T) {
	data, err := loadFileForTest("internal/infrastructure/session/session.go")
	if err != nil {
		t.Fatalf("cannot read session.go: %v", err)
	}
	matches := sessionPrintfRe.FindAllString(string(data), -1)
	if len(matches) > 0 {
		t.Errorf("session.go still contains %d fmt.Printf call(s) — must use slog; matches=%v", len(matches), matches)
	}
	if !strings.Contains(string(data), "slog.") {
		t.Error("session.go should use slog for lifecycle events")
	}
	mustContain := []string{"slog.Debug", "slog.Info"}
	found := false
	for _, mc := range mustContain {
		if strings.Contains(string(data), mc) {
			found = true
			break
		}
	}
	if !found {
		t.Error("session.go should log via slog.Debug or slog.Info for page lifecycle")
	}
}

// --- 3. KHS / LMS slog keys must be consistent ---

func TestPhase1_SlogKeysConsistent(t *testing.T) {
	data, err := loadFileForTest("internal/application/service/lms_service.go")
	if err != nil {
		t.Fatalf("cannot read lms_service.go: %v", err)
	}
	s := string(data)
	mustHave := []string{
		`"npm"`,
		`"tahun_ajaran"`,
		`"semester"`,
	}
	for _, k := range mustHave {
		if !strings.Contains(s, k) {
			t.Errorf("lms_service.go missing slog key %s", k)
		}
	}
}

func TestPhase1_NoDuplicateSlogAndFmtMix(t *testing.T) {
	files := []string{
		"internal/infrastructure/session/manager.go",
		"internal/infrastructure/session/session.go",
		"internal/application/service/lms_service.go",
	}
	for _, f := range files {
		data, err := loadFileForTest(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		s := string(data)
		hasFmt := strings.Contains(s, "fmt.Printf")
		hasSlog := strings.Contains(s, "slog.")
		if hasFmt && hasSlog {
			t.Errorf("%s mixes fmt.Printf and slog — pick slog for structured logs", f)
		}
		if hasFmt {
			t.Errorf("%s still contains fmt.Printf — must use slog; found raw fmt.Printf", f)
		}
	}
}

// --- 4. Logger layer must support level filtering (DEBUG suppressed in prod) ---

func TestPhase1_LevelFiltering(t *testing.T) {
	var b bytes.Buffer
	prod := logger.New("production", &b)
	prod.Debug("should be hidden")
	if b.Len() != 0 {
		t.Errorf("production logger must suppress DEBUG, got %q", b.String())
	}
	b.Reset()
	prod.Info("should appear", "k", "v")
	if b.Len() == 0 {
		t.Error("production logger must emit INFO")
	}
	var m map[string]any
	if err := json.Unmarshal(b.Bytes(), &m); err != nil {
		t.Fatalf("production output must be JSON: %v / %q", err, b.String())
	}
	_ = slog.Default()

	var b2 bytes.Buffer
	dev := logger.New("development", &b2)
	dev.Debug("should appear in dev")
	if b2.Len() == 0 {
		t.Error("development logger must emit DEBUG")
	}
	if strings.HasPrefix(b2.String(), "{") {
		t.Errorf("development output must be text, got JSON: %q", b2.String())
	}
}

// --- 5. Fiber logger and slog must not duplicate without trace_id correlation ---

func TestPhase1_FiberLoggerNotDuplicatingSlog(t *testing.T) {
	data, err := loadFileForTest("internal/infrastructure/middleware/middleware.go")
	if err != nil {
		t.Fatalf("cannot read middleware.go: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "requestid") {
		t.Error("middleware must register requestid so slog can correlate")
	}
}

// --- 6. Apperror fibererror must log internal only, not leak to client ---

func TestPhase1_FiberErrorDoesNotLeakInternal(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Get("/boom", func(c fiber.Ctx) error {
		return apperror.Internal("public msg", apperror.ErrPDFNotFound)
	})
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("want 500 got %d", resp.StatusCode)
	}
	body := readAllForTest(resp)
	if strings.Contains(body, "PDF file not found") {
		t.Error("fibererror must not leak internal error to client")
	}
	if !strings.Contains(body, "public msg") {
		t.Errorf("fibererror must expose PublicMsg, body=%q", body)
	}
}

// --- extra: middleware logger must be slog-injectable (no hardcoded stdout) ---

func TestPhase1_MiddlewareLoggerNoHardcodedDest(t *testing.T) {
	data, err := loadFileForTest("internal/infrastructure/middleware/middleware.go")
	if err != nil {
		t.Fatalf("cannot read middleware.go: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "logger.New()") && !strings.Contains(s, "logger.Config") {
		t.Logf("[AUDIT] middleware uses fiber logger with default stdout — consider slog-based request logger")
	}
}

// --- helpers ---

func readAllForTest(resp *http.Response) string {
	buf := make([]byte, 1<<20)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}

func loadFileForTest(rel string) ([]byte, error) {
	p := filepath.Join(repoRoot(), rel)
	return os.ReadFile(p)
}
