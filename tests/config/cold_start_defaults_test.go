package config

import (
	"testing"
	"time"

	"lonceng_unman_be/internal/config"
)

// TestColdStart_Defaults_ApplyWhenEnvUnset verifies that the Tier-1 + Tier-2
// cold-start defaults are applied when the corresponding env vars are
// unset. These defaults are what make the binary run out-of-the-box without
// 40s–3min first-run downloads or hangs.
func TestColdStart_Defaults_ApplyWhenEnvUnset(t *testing.T) {
	t.Setenv("APP_NAME", "test")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")

	// Explicitly unset the cold-start knobs we're testing.
	for _, k := range []string{
		"DNS_TIMEOUT", "SESSION_TTL", "MAX_SESSIONS",
		"PHOTO_RENDER_WAIT", "PHOTO_RETRY_WAIT",
		"SCRAPE_FORM_WAIT", "BROWSER_LAUNCH_TIMEOUT",
	} {
		t.Setenv(k, "")
	}

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("config.New() error = %v", err)
	}

	// Tier-1: DNS_TIMEOUT default raised 2s → 5s (Docker DNS overload).
	if cfg.App.DNSTimeout != 5*time.Second {
		t.Errorf("DNS_TIMEOUT default = %v, want 5s", cfg.App.DNSTimeout)
	}

	// Pure ephemeral spec 2026-09-09: SESSION_TTL default 15m (was 24h).
	if cfg.App.SessionTTL != 15*time.Minute {
		t.Errorf("SESSION_TTL default = %v, want 15m", cfg.App.SessionTTL)
	}

	// Tier-1: MAX_SESSIONS 15 → 20.
	if cfg.App.MaxSessions != 20 {
		t.Errorf("MAX_SESSIONS default = %d, want 20", cfg.App.MaxSessions)
	}

	// Tier-2: hardcoded 8s/3s/3s sleeps replaced by configurable knobs.
	if cfg.App.PhotoRenderWait != 3*time.Second {
		t.Errorf("PHOTO_RENDER_WAIT default = %v, want 3s", cfg.App.PhotoRenderWait)
	}
	if cfg.App.PhotoRetryWait != 1*time.Second {
		t.Errorf("PHOTO_RETRY_WAIT default = %v, want 1s", cfg.App.PhotoRetryWait)
	}
	if cfg.App.ScrapeFormWait != 2*time.Second {
		t.Errorf("SCRAPE_FORM_WAIT default = %v, want 2s", cfg.App.ScrapeFormWait)
	}
	if cfg.App.BrowserLaunchTimeout != 60*time.Second {
		t.Errorf("BROWSER_LAUNCH_TIMEOUT default = %v, want 60s", cfg.App.BrowserLaunchTimeout)
	}
}

// TestColdStart_EnvOverrides verifies operators can override the new
// defaults via env vars without breaking the config pipeline.
func TestColdStart_EnvOverrides(t *testing.T) {
	t.Setenv("APP_NAME", "test")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "3000")
	t.Setenv("APP_HOST", "0.0.0.0")

	t.Setenv("DNS_TIMEOUT", "3s")
	t.Setenv("SESSION_TTL", "1h")
	t.Setenv("MAX_SESSIONS", "50")
	t.Setenv("PHOTO_RENDER_WAIT", "5s")
	t.Setenv("PHOTO_RETRY_WAIT", "2s")
	t.Setenv("SCRAPE_FORM_WAIT", "4s")
	t.Setenv("BROWSER_LAUNCH_TIMEOUT", "90s")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("config.New() error = %v", err)
	}

	if cfg.App.DNSTimeout != 3*time.Second {
		t.Errorf("DNS_TIMEOUT = %v, want 3s", cfg.App.DNSTimeout)
	}
	if cfg.App.SessionTTL != time.Hour {
		t.Errorf("SESSION_TTL = %v, want 1h", cfg.App.SessionTTL)
	}
	if cfg.App.MaxSessions != 50 {
		t.Errorf("MAX_SESSIONS = %d, want 50", cfg.App.MaxSessions)
	}
	if cfg.App.PhotoRenderWait != 5*time.Second {
		t.Errorf("PHOTO_RENDER_WAIT = %v, want 5s", cfg.App.PhotoRenderWait)
	}
	if cfg.App.PhotoRetryWait != 2*time.Second {
		t.Errorf("PHOTO_RETRY_WAIT = %v, want 2s", cfg.App.PhotoRetryWait)
	}
	if cfg.App.ScrapeFormWait != 4*time.Second {
		t.Errorf("SCRAPE_FORM_WAIT = %v, want 4s", cfg.App.ScrapeFormWait)
	}
	if cfg.App.BrowserLaunchTimeout != 90*time.Second {
		t.Errorf("BROWSER_LAUNCH_TIMEOUT = %v, want 90s", cfg.App.BrowserLaunchTimeout)
	}
}
