package browser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Browser wraps go-rod's browser and launcher for lifecycle management.
type Browser struct {
	rod             *rod.Browser
	launcher        *launcher.Launcher
	preserveProfile bool
}

// New creates a Browser instance (does not connect yet).
func New() *Browser {
	return &Browser{}
}

// Connect launches a headless Chrome instance and connects to it.
func (b *Browser) Connect(headless bool) error {
	l := launcher.New().
		Headless(headless).
		Set("no-sandbox").
		Set("disable-gpu")

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	b.launcher = l

	r := rod.New().ControlURL(url)
	if err := r.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}
	b.rod = r
	return nil
}

// ConnectWithProfile launches Chrome with a persistent user-data-dir.
// The profile preserves cookies, localStorage, and IndexedDB across restarts.
// Close() will NOT call launcher.Cleanup() — the profile directory persists.
func (b *Browser) ConnectWithProfile(headless bool, profileDir string) error {
	cleanStaleLock(profileDir)

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}

	l := launcher.New().
		UserDataDir(profileDir).
		Headless(headless).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-http-cache").
		Set("disk-cache-size=0").
		Set("media-cache-size=0")

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	b.launcher = l
	b.preserveProfile = true

	r := rod.New().ControlURL(url)
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
