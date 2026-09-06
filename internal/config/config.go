package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	App  AppConfig
	CORS CORSConfig
}

// AppConfig holds server-related and LMS browser automation configuration.
type AppConfig struct {
	Name string
	Env  string
	Port string
	Host string
	// LMS Browser Automation
	LMSBaseURL      string
	LMSDashboardURL string
	BrowserHeadless bool
	BrowserTimeout  time.Duration
	DNSTimeout      time.Duration
	// Document Download
	DownloadDir string
	ExtractDir  string
	EvalDir     string
	// Session Management
	SessionTTL  time.Duration
	MaxSessions int
	// Server Limits
	MaxBodySize int64
	MaxPDFSize  int64
	// Session Persistence
	ProfileBaseDir string
	// Photo Cache
	PhotoCacheTTL time.Duration
	// Photo Compression
	MaxPhotoDimension int
	PhotoQuality      int
	// Eval Dashboard Auth
	EvalSecret string // SECRET_KEY (empty = fail-closed)
	AutoLogin  bool   // AUTO_LOGIN (dev bypass, cannot be true in production)
	// Cold-start tuning
	// PhotoRenderWait is the initial wait after navigating to the dashboard
	// so the JS-driven photo DOM has time to populate before the eval probe.
	// Replaces a hardcoded 8s sleep in student_profile_service.
	PhotoRenderWait time.Duration
	// PhotoRetryWait is the wait between photo-scrape eval retries when the
	// first attempt returns an empty src. Replaces a hardcoded 3s sleep.
	PhotoRetryWait time.Duration
	// ScrapeFormWait is the wait after the profile form is detected, allowing
	// JS to populate all 55+ fields before the bulk eval. Replaces a
	// hardcoded 3s sleep in scraper.Scrape.
	ScrapeFormWait time.Duration
	// BrowserLaunchTimeout caps l.Launch() + rod.New().Connect() together.
	// Without this, go-rod's auto-download (or a hung Chromium spawn) can
	// stall the entire cold path with no ceiling.
	BrowserLaunchTimeout time.Duration
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
			Name:            getEnv("APP_NAME", "lonceng_unman_be"),
			Env:             getEnv("APP_ENV", "development"),
			Port:            getEnv("APP_PORT", "3000"),
			Host:            getEnv("APP_HOST", "0.0.0.0"),
			LMSBaseURL:      getEnv("LMS_BASE_URL", "https://elearning.universitasmandiri.ac.id"),
			LMSDashboardURL: getEnv("LMS_DASHBOARD_URL", "https://elearning.universitasmandiri.ac.id/admin/"),
			BrowserHeadless: getEnvBool("BROWSER_HEADLESS", true),
			BrowserTimeout:  getEnvDuration("BROWSER_TIMEOUT", 60*time.Second),
			// DNS_TIMEOUT default raised from 2s → 5s: telemetry from
			// tmp/edge-case-stress.mjs shows Docker internal DNS (127.0.0.11:53)
			// needs 3-5s during heavy concurrent load before the retry (handled
			// in Manager.CheckDNS) can succeed. Operators can still override with
			// DNS_TIMEOUT env var.
			DNSTimeout:  getEnvDuration("DNS_TIMEOUT", 5*time.Second),
			DownloadDir: getEnv("DOWNLOAD_DIR", "./downloads"),
			ExtractDir:  getEnv("EXTRACT_DIR", "./extracted"),
			EvalDir:     getEnv("EVAL_DIR", "./eval/ground_truth"),
			// SESSION_TTL default raised from 15m → 24h: amortizes the cold-start
			// cost across an entire academic day. Combined with the 2h hard
			// lifetime cap (manager.go:26-30), sessions that survive past 2h
			// are force-evicted anyway, so 24h is effectively a soft ceiling.
			SessionTTL:        getEnvDuration("SESSION_TTL", 24*time.Hour),
			MaxSessions:       getEnvInt("MAX_SESSIONS", 20),
			ProfileBaseDir:    getEnv("PROFILE_BASE_DIR", "./profiles"),
			MaxBodySize:       parseByteSize(getEnv("MAX_BODY_SIZE", "1MB")),
			MaxPDFSize:        parseByteSize(getEnv("MAX_PDF_SIZE", "50MB")),
			PhotoCacheTTL:     getEnvDuration("PHOTO_CACHE_TTL", 15*time.Minute),
			MaxPhotoDimension: getEnvInt("MAX_PHOTO_DIMENSION", 300),
			PhotoQuality:      getEnvInt("PHOTO_QUALITY", 80),
			EvalSecret:        getEnv("SECRET_KEY", ""),
			AutoLogin:         getEnvBool("AUTO_LOGIN", false),
			// Cold-start tuning defaults (env-overridable).
			// PHOTO_RENDER_WAIT replaces a hardcoded 8s wait; the 3s default
			// matches observed LMS latency once the dashboard is on screen.
			PhotoRenderWait: getEnvDuration("PHOTO_RENDER_WAIT", 3*time.Second),
			// PHOTO_RETRY_WAIT replaces a hardcoded 3s retry loop wait; 1s is
			// sufficient for the JS to re-render after a missed eval.
			PhotoRetryWait: getEnvDuration("PHOTO_RETRY_WAIT", 1*time.Second),
			// SCRAPE_FORM_WAIT replaces a hardcoded 3s wait after the form is
			// detected; 2s is enough for the 55+ fields to populate.
			// scraper enforces [2s,30s] (see browser/scraper.go probeTimeout clamp).
			ScrapeFormWait: getEnvDuration("SCRAPE_FORM_WAIT", 2*time.Second),
			// BROWSER_LAUNCH_TIMEOUT caps the launcher.Launch() + Connect()
			// window. Defaults to the same 60s as BROWSER_TIMEOUT because the
			// launch path is the dominant cold-start cost.
			BrowserLaunchTimeout: getEnvDuration("BROWSER_LAUNCH_TIMEOUT", 60*time.Second),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv("CORS_ALLOW_ORIGINS", "*"), // production Validate rejects "*"; set explicit origins in .env for prod
			AllowMethods: getEnv("CORS_ALLOW_METHODS", "GET,POST,OPTIONS"),
			AllowHeaders: getEnv("CORS_ALLOW_HEADERS", "Content-Type"),
		},
	}

	// Resolve relative directory paths to absolute paths anchored at the
	// process's current working directory. This is critical because the
	// server may be launched from a CWD that differs from the project
	// root (Windows shortcut, Task Scheduler, service host, Docker
	// container with WORKDIR != /app, etc.). Without this, paths like
	// "./downloads" would resolve against an arbitrary CWD and the
	// server would fail to find files that are clearly present on disk.
	absDownload, err := filepath.Abs(cfg.App.DownloadDir)
	if err != nil {
		return nil, fmt.Errorf("resolve download_dir: %w", err)
	}
	cfg.App.DownloadDir = absDownload

	absExtract, err := filepath.Abs(cfg.App.ExtractDir)
	if err != nil {
		return nil, fmt.Errorf("resolve extract_dir: %w", err)
	}
	cfg.App.ExtractDir = absExtract

	absEval, err := filepath.Abs(cfg.App.EvalDir)
	if err != nil {
		return nil, fmt.Errorf("resolve eval_dir: %w", err)
	}
	cfg.App.EvalDir = absEval

	absProfile, err := filepath.Abs(cfg.App.ProfileBaseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve profile_base_dir: %w", err)
	}
	cfg.App.ProfileBaseDir = absProfile

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// Validate checks that all configuration values are within acceptable ranges.
func (c *Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app_name must not be empty")
	}

	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[c.App.Env] {
		return fmt.Errorf("app_env must be one of: development, staging, production; got %q", c.App.Env)
	}

	port, err := strconv.Atoi(c.App.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("app_port must be a valid port number (1-65535); got %q", c.App.Port)
	}

	if c.App.Host == "" {
		return fmt.Errorf("app_host must not be empty")
	}

	if c.App.BrowserTimeout <= 0 {
		return fmt.Errorf("browser_timeout must be a positive duration; got %v", c.App.BrowserTimeout)
	}

	if c.App.DownloadDir == "" {
		return fmt.Errorf("download_dir must not be empty")
	}

	if c.App.ExtractDir == "" {
		return fmt.Errorf("extract_dir must not be empty")
	}

	if c.App.EvalDir == "" {
		return fmt.Errorf("eval_dir must not be empty")
	}

	if c.App.MaxPhotoDimension < 50 || c.App.MaxPhotoDimension > 2000 {
		return fmt.Errorf("max_photo_dimension must be between 50 and 2000; got %d", c.App.MaxPhotoDimension)
	}

	if c.App.PhotoQuality < 1 || c.App.PhotoQuality > 100 {
		return fmt.Errorf("photo_quality must be between 1 and 100; got %d", c.App.PhotoQuality)
	}

	if c.App.AutoLogin && c.App.Env == "production" {
		return errors.New("AUTO_LOGIN cannot be true in production")
	}

	if strings.TrimSpace(c.App.LMSBaseURL) == "" {
		return fmt.Errorf("lms_base_url must not be empty")
	}
	if _, err := url.ParseRequestURI(c.App.LMSBaseURL); err != nil {
		return fmt.Errorf("lms_base_url invalid: %w", err)
	}

	if c.App.DNSTimeout <= 0 {
		return fmt.Errorf("dns_timeout must be > 0; got %v", c.App.DNSTimeout)
	}

	if c.App.SessionTTL <= 0 {
		return fmt.Errorf("session_ttl must be > 0; got %v", c.App.SessionTTL)
	}

	if c.App.BrowserLaunchTimeout <= 0 {
		return fmt.Errorf("browser_launch_timeout must be > 0; got %v", c.App.BrowserLaunchTimeout)
	}

	if strings.TrimSpace(c.App.ProfileBaseDir) == "" {
		return fmt.Errorf("profile_base_dir must not be empty")
	}

	if c.App.Env == "production" && c.CORS.AllowOrigins == "*" {
		return fmt.Errorf("cors_allow_origins must not be '*' in production")
	}

	if c.App.Env == "production" && strings.TrimSpace(c.App.EvalSecret) == "" {
		return fmt.Errorf("secret_key must not be empty in production")
	}

	if c.App.MaxSessions < 0 {
		return fmt.Errorf("max_sessions must be >= 0; got %d", c.App.MaxSessions)
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

// getEnvBool reads a boolean environment variable. Accepts "true", "1", "yes" (case-insensitive).
func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

// getEnvDuration reads a duration environment variable (e.g. "30s", "10m").
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return d
}

// getEnvInt reads an integer environment variable.
func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// parseByteSize parses a human-readable byte size string (e.g. "1MB", "512KB")
// into bytes. Supports B, KB, MB, GB suffixes (case-insensitive).
// Returns 1MB (1048576) on parse failure.
func parseByteSize(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(s, m.suffix))
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 1024 * 1024 // default 1MB
			}
			return n * m.mult
		}
	}

	// Plain number treated as bytes
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 1024 * 1024 // default 1MB
	}
	return n
}
