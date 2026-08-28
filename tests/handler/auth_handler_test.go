package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/auth"
	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

func newAuthTestHandler(t *testing.T, cfg *config.Config) *handler.AuthHandler {
	t.Helper()
	h, err := handler.NewAuthHandler(cfg)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}
	return h
}

func TestLoginPage_RendersForm(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "test"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Get("/eval/login", h.LoginPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/login", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestLoginPage_AlreadyAuthenticated(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "test"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Get("/eval/login", h.LoginPage)

	token := auth.SignToken([]byte("test"))
	req := httptest.NewRequest(http.MethodGet, "/eval/login", nil)
	req.AddCookie(&http.Cookie{Name: "eval_session", Value: token})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, fiber.StatusSeeOther)
	}
}

func TestLogin_ValidKey(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "correct-key"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Post("/eval/login", h.Login)

	form := url.Values{}
	form.Add("key", "correct-key")
	req := httptest.NewRequest(http.MethodPost, "/eval/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, fiber.StatusSeeOther)
	}
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "eval_session" && c.Value != "" && c.HttpOnly && c.Path == "/" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected eval_session cookie, got: %v", cookies)
	}
}

func TestLogin_WrongKey(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "correct-key"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Post("/eval/login", h.Login)

	form := url.Values{}
	form.Add("key", "wrong-key")
	req := httptest.NewRequest(http.MethodPost, "/eval/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (re-render)", resp.StatusCode, fiber.StatusOK)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Not Allowed") {
		t.Errorf("body should contain 'Not Allowed', got: %s", body)
	}
}

func TestLogin_EmptyKey(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "correct-key"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Post("/eval/login", h.Login)

	form := url.Values{}
	form.Add("key", "")
	req := httptest.NewRequest(http.MethodPost, "/eval/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (re-render)", resp.StatusCode, fiber.StatusOK)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Key is required") {
		t.Errorf("body should contain 'Key is required', got: %s", body)
	}
}

func TestLogin_EmptySecret(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: ""}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Post("/eval/login", h.Login)

	form := url.Values{}
	form.Add("key", "any-key")
	req := httptest.NewRequest(http.MethodPost, "/eval/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (re-render)", resp.StatusCode, fiber.StatusOK)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "500") {
		t.Errorf("body should contain '500', got: %s", body)
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "test"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Get("/eval/logout", h.Logout)

	req := httptest.NewRequest(http.MethodGet, "/eval/logout", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, fiber.StatusSeeOther)
	}
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "eval_session" && c.MaxAge == -1 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected cleared eval_session cookie (MaxAge=-1), got: %v", cookies)
	}
}

func TestLogin_RateLimited(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{EvalSecret: "correct-key"}}
	h := newAuthTestHandler(t, cfg)
	app := fiber.New(fiber.Config{ErrorHandler: fibererror.New()})
	app.Post("/eval/login", h.Login)

	form := url.Values{}
	form.Add("key", "wrong-key")

	// 5 failed attempts → 6th should be rate limited
	for range 5 {
		req := httptest.NewRequest(http.MethodPost, "/eval/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.Test(req)
	}

	req := httptest.NewRequest(http.MethodPost, "/eval/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("status = %d, want %d (rate limited)", resp.StatusCode, fiber.StatusTooManyRequests)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 1<<20)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}
