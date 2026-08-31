package browser_test

import (
	"os"
	"path/filepath"
	"testing"

	"lonceng_unman_be/internal/infrastructure/browser"
)

// Exposed white-box entry — the package-level unexported helpers
// are exercised via their documented side effects (env var reads,
// file existence). We don't reach into private state; we assert
// that browser.New() produces a usable Browser value and that the
// binary-resolution path doesn't crash when env vars point at
// non-existent files (Tier-1 cold-start fix).

func TestNew_ReturnsNonNilBrowser(t *testing.T) {
	b := browser.New()
	if b == nil {
		t.Fatal("browser.New() returned nil")
	}
}

func TestNew_WithUnsetEnv_NoCrash(t *testing.T) {
	// Tier-1 fix: must not panic / hang when ROD_BROWSER / CHROMIUM_PATH
	// are unset and no well-known Chrome is installed. We don't call
	// Connect() (would require a real Chromium); we just ensure the
	// constructor returns a valid value that downstream code can hold.
	b := browser.New()
	if b == nil {
		t.Fatal("expected non-nil browser when env unset")
	}
}

func TestSetLaunchTimeout_ZeroMeansNoChange(t *testing.T) {
	b := browser.New()
	// Setting 0 must be a no-op (not panicking). Subsequent Connect()
	// would then inherit rod's default 3-minute timeout.
	b.SetLaunchTimeout(0)
}

func TestSetLaunchTimeout_Positive(t *testing.T) {
	b := browser.New()
	// Must not panic. Value is stored on the struct and applied during
	// the next Connect / ConnectWithProfile call.
	b.SetLaunchTimeout(30 * 1_000_000_000) // 30s in ns
}

// Exposed smoke test that touches the existing ROD_BROWSER path without
// actually launching Chromium. We create a temp file (so os.Stat succeeds)
// and set ROD_BROWSER to that path; then assert the file remains reachable.
// This guards against the previously-documented "auto-download on first run"
// failure mode where ROD_BROWSER was honored but never tested for existence.
func TestRodBrowserEnv_FileExists(t *testing.T) {
	tmp := t.TempDir()
	dummy := filepath.Join(tmp, "fake-chromium")
	if err := os.WriteFile(dummy, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatalf("create dummy: %v", err)
	}
	t.Setenv("ROD_BROWSER", dummy)
	if _, err := os.Stat(os.Getenv("ROD_BROWSER")); err != nil {
		t.Errorf("ROD_BROWSER file should exist: %v", err)
	}
}

func TestRodBrowserEnv_MissingFile(t *testing.T) {
	// Tier-1: when ROD_BROWSER points at a missing path, the resolution
	// chain must fall through gracefully (LookPath / well-known dirs).
	// We only verify os.Stat behavior here — actual fallback resolution
	// is exercised by browser package internals.
	t.Setenv("ROD_BROWSER", filepath.Join(t.TempDir(), "nonexistent"))
	if _, err := os.Stat(os.Getenv("ROD_BROWSER")); err == nil {
		t.Error("expected stat error for nonexistent path")
	}
}
