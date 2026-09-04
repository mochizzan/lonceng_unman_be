package browser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
)

// _ ensures log/slog is treated as used (Task 2 will replace this with
// real retry logging; removed once slog has real call sites in this file).
var _ = slog.Default

// chromiumPathCandidates is the ordered list of well-known installed
// Chromium/Chrome binary paths that we probe BEFORE falling back to
// go-rod's auto-download. Setting ROD_BROWSER still wins (highest priority).
//
// On Linux (Alpine/Docker) the Dockerfile installs Chromium at
// /usr/bin/chromium-browser and creates symlinks for chromium and
// google-chrome. On Windows, the project's existing manual install of
// Chrome via the system installer is honored.
var chromiumPathCandidates = []string{
	"/usr/bin/chromium-browser",
	"/usr/bin/chromium",
	"/usr/bin/google-chrome",
	`C:\Program Files\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
}

// browserBinPath resolves the Chromium binary path in this priority order:
//  1. ROD_BROWSER env var (explicit operator override, must exist on disk)
//  2. CHROMIUM_PATH env var (alternative override, must exist on disk)
//  3. go-rod's launcher.LookPath() — searches PATH and well-known dirs
//  4. First existing file in chromiumPathCandidates (well-known install paths)
//  5. Empty string — go-rod will try to find, then auto-download as last resort
//
// Returning the pre-installed path BEFORE go-rod's auto-download prevents
// the 30s–2min first-run Chromium download penalty. The pre-installed binary
// is what the Dockerfile provides.
func browserBinPath() string {
	for _, key := range []string{"ROD_BROWSER", "CHROMIUM_PATH"} {
		if v := os.Getenv(key); v != "" {
			if _, err := os.Stat(v); err == nil {
				return v
			}
		}
	}
	if found, ok := launcher.LookPath(); ok {
		return found
	}
	for _, p := range chromiumPathCandidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// parseRodFlags returns the parsed launcher flags from ROD_FLAGS env var.
// Format matches Chromium command-line flag conventions (e.g. "no-sandbox",
// "disable-gpu", "headless=new"). Unknown / duplicate flags are tolerated.
//
// Returned slice is empty (length 0, NOT nil) when ROD_FLAGS is unset, so
// callers can detect "no override" with `len(parsed) == 0`.
func parseRodFlags() []string {
	v := os.Getenv("ROD_FLAGS")
	if v == "" {
		return []string{}
	}
	return strings.Fields(v)
}

// Browser wraps go-rod's browser and launcher for lifecycle management.
type Browser struct {
	rod             *rod.Browser
	launcher        *launcher.Launcher
	preserveProfile bool
	launchTimeout   time.Duration
}

// SetLaunchTimeout caps the post-launch window (rod.New().ControlURL() and
// Browser.Connect()) with a deadline. Subsequent page ops inherit this
// timeout. Zero (default) means no override — rod uses its 3-minute default.
func (b *Browser) SetLaunchTimeout(d time.Duration) {
	b.launchTimeout = d
}

// New creates a Browser instance (does not connect yet).
func New() *Browser {
	return &Browser{}
}

// Connect launches a headless Chrome instance and connects to it.
//
// Flags applied (override-able by ROD_FLAGS env var):
//   - no-sandbox              (required when running as root / in containers)
//   - disable-gpu             (no GPU in headless servers)
//   - disable-dev-shm-usage   (prevents /dev/shm exhaustion in Docker)
//   - disable-extensions      (smaller footprint, faster cold start)
//   - no-first-run            (skip Chromium's first-run wizards)
//   - no-default-browser-check (skip "are you sure?" dialogs)
//   - disable-background-networking (no background telemetry uploads)
//   - leakless                (force-kill child if Go process exits — graceful
//     shutdown reliability; see ysmood/leakless)
//
// ROD_FLAGS env var, when set, REPLACES the default flag set above with
// operator-supplied flags (parsed space-separated). Use this to override
// per-deployment (e.g. add `--lang=id-ID` or `--proxy-server=...`).
func (b *Browser) Connect(headless bool) error {
	l := launcher.New().
		Headless(headless).
		Leakless(true).
		Set(flags.Flag("no-sandbox")).
		Set(flags.Flag("disable-gpu")).
		Set(flags.Flag("disable-dev-shm-usage")).
		Set(flags.Flag("disable-extensions")).
		Set(flags.Flag("no-first-run")).
		Set(flags.Flag("no-default-browser-check")).
		Set(flags.Flag("disable-background-networking"))

	if extra := parseRodFlags(); len(extra) > 0 {
		l = launcher.New().Headless(headless).Leakless(true)
		for _, f := range extra {
			l = l.Set(flags.Flag(f))
		}
	}

	if bin := browserBinPath(); bin != "" {
		l = l.Bin(bin)
	}

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	b.launcher = l

	r := rod.New().ControlURL(url)
	if b.launchTimeout > 0 {
		r = r.Timeout(b.launchTimeout)
	}
	if err := r.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}
	b.rod = r
	return nil
}

// ConnectWithProfile launches Chrome with a persistent user-data-dir.
// The profile preserves cookies, localStorage, and IndexedDB across restarts.
// Close() will NOT call launcher.Cleanup() — the profile directory persists.
//
// Same flag policy as Connect() — see comment above.
func (b *Browser) ConnectWithProfile(headless bool, profileDir string) error {
	cleanStaleLock(profileDir)

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}

	l := launcher.New().
		UserDataDir(profileDir).
		Headless(headless).
		Leakless(true).
		Set(flags.Flag("no-sandbox")).
		Set(flags.Flag("disable-gpu")).
		Set(flags.Flag("disable-dev-shm-usage")).
		Set(flags.Flag("disable-extensions")).
		Set(flags.Flag("no-first-run")).
		Set(flags.Flag("no-default-browser-check")).
		Set(flags.Flag("disable-background-networking"))

	if extra := parseRodFlags(); len(extra) > 0 {
		l = launcher.New().
			UserDataDir(profileDir).
			Headless(headless).
			Leakless(true)
		for _, f := range extra {
			l = l.Set(flags.Flag(f))
		}
	}

	if bin := browserBinPath(); bin != "" {
		l = l.Bin(bin)
	}

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	b.launcher = l
	b.preserveProfile = true

	r := rod.New().ControlURL(url)
	if b.launchTimeout > 0 {
		r = r.Timeout(b.launchTimeout)
	}
	if err := r.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}
	b.rod = r
	return nil
}

// cleanStaleLock removes Chrome SingletonLock files left behind after a crash.
func cleanStaleLock(profileDir string) {
	for _, name := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
		path := filepath.Join(profileDir, name)
		_ = os.Remove(path)
	}
}

// Page opens a new page and navigates to the given URL.
func (b *Browser) Page(url string) (*rod.Page, error) {
	page, err := b.rod.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open page %s: %w", url, err)
	}
	return page, nil
}

// Close gracefully shuts down the browser and cleans up the launcher.
func (b *Browser) Close() error {
	var firstErr error
	if b.rod != nil {
		if err := b.rod.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close browser: %w", err)
		}
	}
	if b.launcher != nil && !b.preserveProfile {
		b.launcher.Cleanup()
	}
	return firstErr
}
