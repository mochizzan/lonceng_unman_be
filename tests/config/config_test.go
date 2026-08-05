package config

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

func TestValidate_EmptyName(t *testing.T) {
	// Test Validate() directly since t.Setenv("", "") + getEnv fallback
	// means New() never receives an empty name.
	cfg := &config.Config{
		App: config.AppConfig{Name: "", Env: "development", Port: "3000", Host: "0.0.0.0"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty APP_NAME")
	}
}

func TestNew_WithExplicitValues(t *testing.T) {
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
