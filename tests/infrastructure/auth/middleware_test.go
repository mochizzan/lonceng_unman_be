package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/auth"
	"lonceng_unman_be/internal/infrastructure/fibererror"

	"github.com/gofiber/fiber/v3"
)

func newTestConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name:       "test",
			Env:        "development",
			EvalSecret: "test-secret",
		},
	}
}

func TestEvalAuth_SkipsPublicPaths(t *testing.T) {
	cfg := newTestConfig()
	limiter := auth.NewLoginLimiter()
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Use(auth.EvalAuth(cfg, limiter))
	app.Get("/eval/login", func(c fiber.Ctx) error {
		return c.SendString("login page")
	})
	app.Get("/eval/logout", func(c fiber.Ctx) error {
		return c.SendString("logout")
	})
	app.Get("/eval/static/css/main.css", func(c fiber.Ctx) error {
		return c.SendString("css")
	})

	tests := []struct {
		path string
	}{
		{"/eval/login"},
		{"/eval/logout"},
		{"/eval/static/css/main.css"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s): %v", tt.path, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("%s: status = %d, want %d", tt.path, resp.StatusCode, fiber.StatusOK)
		}
	}
}

func TestEvalAuth_AutoLoginBypasses(t *testing.T) {
	cfg := newTestConfig()
	cfg.App.AutoLogin = true
	limiter := auth.NewLoginLimiter()
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Use(auth.EvalAuth(cfg, limiter))
	app.Get("/eval", func(c fiber.Ctx) error {
		return c.SendString("dashboard")
	})

	req := httptest.NewRequest(http.MethodGet, "/eval", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestEvalAuth_EmptySecretDenies(t *testing.T) {
	cfg := newTestConfig()
	cfg.App.EvalSecret = ""
	limiter := auth.NewLoginLimiter()
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Use(auth.EvalAuth(cfg, limiter))
	app.Get("/eval", func(c fiber.Ctx) error {
		return c.SendString("dashboard")
	})

	req := httptest.NewRequest(http.MethodGet, "/eval", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect to login)", resp.StatusCode, fiber.StatusSeeOther)
	}
}

func TestEvalAuth_ValidCookieAllows(t *testing.T) {
	cfg := newTestConfig()
	limiter := auth.NewLoginLimiter()
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Use(auth.EvalAuth(cfg, limiter))
	app.Get("/eval", func(c fiber.Ctx) error {
		return c.SendString("dashboard")
	})

	token := auth.SignToken([]byte("test-secret"))
	req := httptest.NewRequest(http.MethodGet, "/eval", nil)
	req.AddCookie(&http.Cookie{Name: "eval_session", Value: token})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestEvalAuth_InvalidCookieRedirects(t *testing.T) {
	cfg := newTestConfig()
	limiter := auth.NewLoginLimiter()
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Use(auth.EvalAuth(cfg, limiter))
	app.Get("/eval", func(c fiber.Ctx) error {
		return c.SendString("dashboard")
	})

	req := httptest.NewRequest(http.MethodGet, "/eval", nil)
	req.AddCookie(&http.Cookie{Name: "eval_session", Value: "wrong-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, fiber.StatusSeeOther)
	}
}

func TestEvalAuth_JSONPathReturns401(t *testing.T) {
	cfg := newTestConfig()
	limiter := auth.NewLoginLimiter()
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	api := app.Group("/api/v1", auth.EvalAuth(cfg, limiter))
	api.Get("/eval/students", func(c fiber.Ctx) error {
		return c.SendString("students")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/eval/students", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d (401 JSON)", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestLoginLimiter_AllowsUnderLimit(t *testing.T) {
	limiter := auth.NewLoginLimiter()
	for i := range 5 {
		if !limiter.Allow("127.0.0.1") {
			t.Errorf("attempt %d: Allow returned false, want true", i+1)
		}
	}
}

func TestLoginLimiter_BlocksOverLimit(t *testing.T) {
	limiter := auth.NewLoginLimiter()
	for range 5 {
		limiter.Allow("127.0.0.1")
	}
	if limiter.Allow("127.0.0.1") {
		t.Error("Allow returned true after 5 attempts, want false")
	}
}

func TestLoginLimiter_PerIP(t *testing.T) {
	limiter := auth.NewLoginLimiter()
	for range 5 {
		limiter.Allow("192.168.1.1")
	}
	if !limiter.Allow("192.168.1.2") {
		t.Error("Allow returned false for different IP, want true")
	}
}

func TestTooManyRequestsExists(t *testing.T) {
	err := apperror.TooManyRequests("rate limited")
	if err.StatusCodeInt() != fiber.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", err.StatusCodeInt(), fiber.StatusTooManyRequests)
	}
}
