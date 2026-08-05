package browser

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Browser wraps go-rod's browser and launcher for lifecycle management.
type Browser struct {
	rod      *rod.Browser
	launcher *launcher.Launcher
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
	if b.launcher != nil {
		b.launcher.Cleanup()
	}
	return firstErr
}
