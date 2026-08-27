package config

import (
	"path/filepath"
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

// TestNew_RelativeDirsResolvedToAbsolute verifies that directory paths
// supplied as relative values ("./downloads", "./extracted", etc.) are
// resolved to absolute paths at config-load time. Without this, the
// eval preview handler would search for PDFs under whatever CWD the
// process happened to be launched from — which is often NOT the project
// root when the server is started via Windows shortcut, Task Scheduler,
// or a service host. The PDF would be present on disk but the server
// would still report "tidak ditemukan".
func TestNew_RelativeDirsResolvedToAbsolute(t *testing.T) {
	t.Setenv("APP_NAME", "x")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")
	t.Setenv("DOWNLOAD_DIR", "./downloads")
	t.Setenv("EXTRACT_DIR", "./extracted")
	t.Setenv("EVAL_DIR", "./eval/ground_truth")
	t.Setenv("PROFILE_BASE_DIR", "./profiles")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirs := map[string]string{
		"DownloadDir":    cfg.App.DownloadDir,
		"ExtractDir":     cfg.App.ExtractDir,
		"EvalDir":        cfg.App.EvalDir,
		"ProfileBaseDir": cfg.App.ProfileBaseDir,
	}
	for name, p := range dirs {
		if !filepath.IsAbs(p) {
			t.Errorf("%s = %q is not absolute", name, p)
		}
	}
}

// TestNew_AbsoluteDirsLeftUnchanged verifies that an operator-supplied
// absolute path is preserved verbatim (no double-resolution that could
// produce a surprising prefix on Windows like "C:\C:\path").
func TestNew_AbsoluteDirsLeftUnchanged(t *testing.T) {
	t.Setenv("APP_NAME", "x")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")
	t.Setenv("DOWNLOAD_DIR", `D:\abs\dl`)

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.DownloadDir != `D:\abs\dl` {
		t.Errorf("absolute DOWNLOAD_DIR was modified: got %q", cfg.App.DownloadDir)
	}
}
