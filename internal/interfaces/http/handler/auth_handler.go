package handler

import (
	"crypto/subtle"
	"html/template"
	"log/slog"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/auth"
	"lonceng_unman_be/internal/interfaces/http/evalhtml"

	"github.com/gofiber/fiber/v3"
)

// AuthHandler handles eval dashboard authentication.
type AuthHandler struct {
	cfg       *config.Config
	templates *template.Template
	limiter   *auth.LoginLimiter
}

// Limiter returns the login rate limiter for use by middleware.
func (h *AuthHandler) Limiter() *auth.LoginLimiter {
	return h.limiter
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(cfg *config.Config) (*AuthHandler, error) {
	tmpl, err := template.ParseFS(evalhtml.TemplatesFS, "templates/login.html")
	if err != nil {
		return nil, err
	}
	return &AuthHandler{
		cfg:       cfg,
		templates: tmpl,
		limiter:   auth.NewLoginLimiter(),
	}, nil
}

// LoginPage handles GET /eval/login — renders the login form.
func (h *AuthHandler) LoginPage(c fiber.Ctx) error {
	// If already authenticated, redirect to dashboard
	token := c.Cookies("eval_session")
	if token != "" && auth.ValidateToken([]byte(h.cfg.App.EvalSecret), token) {
		return c.Redirect().To("/eval")
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "login.html", map[string]interface{}{
		"Error": "",
	})
}

// Login handles POST /eval/login — validates the secret key.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	key := c.FormValue("key")

	if key == "" {
		c.Set("Content-Type", "text/html")
		return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "login.html", map[string]interface{}{
			"Error": "Key is required",
		})
	}

	// Fail-closed: empty secret
	if h.cfg.App.EvalSecret == "" {
		slog.Error("eval login attempted with empty SECRET_KEY")
		c.Set("Content-Type", "text/html")
		return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "login.html", map[string]interface{}{
			"Error": "500",
		})
	}

	// Constant-time comparison
	if subtle.ConstantTimeCompare([]byte(key), []byte(h.cfg.App.EvalSecret)) == 1 {
		// Reset rate limiter on successful login
		h.limiter.Reset(c.IP())
		token := auth.SignToken([]byte(h.cfg.App.EvalSecret))
		c.Cookie(&fiber.Cookie{
			Name:     "eval_session",
			Value:    token,
			HTTPOnly: true,
			SameSite: "Lax",
			Path:     "/",
			Secure:   h.cfg.App.Env == "production",
		})
		slog.Info("eval login success", "ip", c.IP())
		return c.Redirect().To("/eval")
	}

	// Rate limiting (only count failed attempts)
	if !h.limiter.Allow(c.IP()) {
		return apperror.TooManyRequests("too many login attempts, try again later")
	}

	slog.Warn("eval login failed", "ip", c.IP())
	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "login.html", map[string]interface{}{
		"Error": "Not Allowed",
	})
}

// Logout handles GET /eval/logout — clears the session cookie.
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "eval_session",
		Value:    "",
		HTTPOnly: true,
		SameSite: "Lax",
		Path:     "/",
		MaxAge:   -1,
	})
	return c.Redirect().To("/eval/login")
}
