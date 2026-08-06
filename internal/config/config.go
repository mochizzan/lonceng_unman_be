package config

import (
	"fmt"
	"os"
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
	// Session Management
	SessionTTL  time.Duration
	MaxSessions int
	// Server Limits
	MaxBodySize int64
	MaxPDFSize  int64
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
			DNSTimeout:      getEnvDuration("DNS_TIMEOUT", 5*time.Second),
			DownloadDir:     getEnv("DOWNLOAD_DIR", "./downloads"),
			ExtractDir:      getEnv("EXTRACT_DIR", "./extracted"),
			SessionTTL:      getEnvDuration("SESSION_TTL", 15*time.Minute),
			MaxSessions:     getEnvInt("MAX_SESSIONS", 10),
			MaxBodySize:     parseByteSize(getEnv("MAX_BODY_SIZE", "1MB")),
			MaxPDFSize:      parseByteSize(getEnv("MAX_PDF_SIZE", "50MB")),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv("CORS_ALLOW_ORIGINS", "*"),
			AllowMethods: getEnv("CORS_ALLOW_METHODS", "GET,POST,OPTIONS"),
			AllowHeaders: getEnv("CORS_ALLOW_HEADERS", "Content-Type"),
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

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 1024 * 1024 // default 1MB
			}
			return n * mult
		}
	}

	// Plain number treated as bytes
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 1024 * 1024 // default 1MB
	}
	return n
}
