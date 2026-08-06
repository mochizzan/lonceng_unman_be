package config

import (
	"fmt"
	"os"
	"strconv"
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
	ActionTimeout   time.Duration
	// Document Download
	DownloadDir string
	// Session Management
	SessionTTL  time.Duration
	MaxSessions int
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
			BrowserTimeout:  getEnvDuration("BROWSER_TIMEOUT", 30*time.Second),
			ActionTimeout:   getEnvDuration("ACTION_TIMEOUT", 10*time.Second),
			DownloadDir:     getEnv("DOWNLOAD_DIR", "./downloads"),
			SessionTTL:      getEnvDuration("SESSION_TTL", 15*time.Minute),
			MaxSessions:     getEnvInt("MAX_SESSIONS", 10),
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

	if c.App.BrowserTimeout <= 0 {
		return fmt.Errorf("BROWSER_TIMEOUT must be a positive duration; got %v", c.App.BrowserTimeout)
	}

	if c.App.ActionTimeout <= 0 {
		return fmt.Errorf("ACTION_TIMEOUT must be a positive duration; got %v", c.App.ActionTimeout)
	}

	if c.App.DownloadDir == "" {
		return fmt.Errorf("DOWNLOAD_DIR must not be empty")
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
