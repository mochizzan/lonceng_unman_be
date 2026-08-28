package auth

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/config"

	"github.com/gofiber/fiber/v3"
)

// LoginLimiter tracks login attempts per IP.
type LoginLimiter struct {
	mu       sync.RWMutex
	attempts map[string][]time.Time
}

// NewLoginLimiter creates a new login rate limiter.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{attempts: make(map[string][]time.Time)}
}

// Allow checks if a login attempt from the given IP is permitted.
// Returns false if 5 or more attempts were made in the last minute.
func (rl *LoginLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	// Clean old entries
	var valid []time.Time
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	if len(valid) >= 5 {
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

// Reset clears the login attempt history for the given IP.
func (rl *LoginLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// fileRegexp matches safe filenames: alphanumeric, underscore, hyphen, dot.
// Must start with alphanumeric (rejects leading dots like "..").
var fileRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// EvalAuth returns middleware that protects eval routes.
func EvalAuth(cfg *config.Config, limiter *LoginLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()

		// Skip auth for public endpoints
		if path == "/eval/login" || path == "/eval/logout" ||
			strings.HasPrefix(path, "/eval/static/") {
			return c.Next()
		}

		// AUTO_LOGIN bypass (dev convenience, gated at config level for prod)
		if cfg.App.AutoLogin {
			return c.Next()
		}

		// Fail-closed: empty secret denies all
		if cfg.App.EvalSecret == "" {
			return deny(c, path)
		}

		// Validate :file parameter to prevent path traversal
		if file := c.Params("file"); file != "" {
			if !fileRegexp.MatchString(file) {
				return apperror.BadRequest("invalid filename format")
			}
		}

		// Validate cookie
		token := c.Cookies("eval_session")
		if token != "" && ValidateToken([]byte(cfg.App.EvalSecret), token) {
			return c.Next()
		}

		return deny(c, path)
	}
}

func deny(c fiber.Ctx, path string) error {
	// JSON API paths → 401 Unauthorized
	if strings.HasPrefix(path, "/api/v1/eval") {
		return apperror.Unauthorized("authentication required")
	}
	// HTML paths → redirect to login
	return c.Redirect().To("/eval/login")
}
