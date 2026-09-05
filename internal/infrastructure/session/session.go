package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"lonceng_unman_be/internal/infrastructure/browser"

	"github.com/go-rod/rod"
)

// pageTimeout bounds individual page operations so that a hung operation
// (e.g., WaitLoad on a corrupted page) returns an error instead of blocking
// forever. This prevents deadlocks when page operations fail.
// Set to 60s to match the maximum probe timeout for slow LMS responses.
const pageTimeout = 60 * time.Second

// rodSession implements port.BrowserSession using a go-rod page.
//
// Architecture: Each rodSession holds its own rod.Page, freshly created
// from the shared browser. The browser maintains authentication state
// (cookies via UserDataDir), so each new page is automatically authenticated.
//
// Concurrency: The cachedSession.pageMu serializes browser.Page() calls
// because go-rod's browser.Page() is not concurrent-safe. Each rodSession
// has its own mutex for page operations, so different rodSessions
// (different requests) can operate concurrently because each has its own
// page and its own mutex.
type rodSession struct {
	page       *rod.Page
	mu         sync.Mutex     // per-session mutex for this rodSession's page ops
	cachedSess *cachedSession // back-reference for release tracking
	closed     bool           // prevents double-decrement of activeCount
}

// newSession creates a fresh page from the browser and wraps it in a rodSession.
// The pageMu is held during page creation to serialize browser.Page() calls.
// Returns an error if the browser has too many open pages (resource exhaustion).
func newSession(cachedSess *cachedSession) (*rodSession, error) {
	cachedSess.pageMu.Lock()

	// Check if browser has too many open pages.
	if cachedSess.pageCount >= maxPagesPerBrowser {
		cachedSess.pageMu.Unlock()
		return nil, fmt.Errorf("browser has too many open pages (%d >= %d) — session corrupted, please retry", cachedSess.pageCount, maxPagesPerBrowser)
	}

	fmt.Printf("[SESSION] Creating new page for npm=%s (pageCount=%d)\n", cachedSess.npm, cachedSess.pageCount)
	page, err := cachedSess.browser.Page("about:blank")
	if err == nil {
		cachedSess.pageCount++
	}
	fmt.Printf("[SESSION] Page created: err=%v, pageCount=%d\n", err, cachedSess.pageCount)
	cachedSess.pageMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("create new page: %w", err)
	}

	return &rodSession{
		page:       page,
		cachedSess: cachedSess,
	}, nil
}

// touchLastUsed updates the lastUsed timestamp of the cached session.
// Guards cachedSess.mu directly so it is safe to call while s.mu is held
// or without holding s.mu. Callers do not need to hold s.mu for this call.
func (s *rodSession) touchLastUsed() {
	s.cachedSess.mu.Lock()
	s.cachedSess.lastUsed = time.Now()
	s.cachedSess.mu.Unlock()
}

// Navigate loads the given URL and waits for the page to be ready.
// It retries on timeout errors and resets page state via about:blank
// to avoid the "stale page" issue where subsequent navigations fail.
func (s *rodSession) Navigate(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	// Reset page state by navigating to about:blank first.
	// This prevents the "context deadline exceeded" error that occurs
	// when a page is reused after the previous page was closed.
	page := s.page.Timeout(pageTimeout)
	if err := page.Navigate("about:blank"); err != nil {
		return fmt.Errorf("reset page state: %w", err)
	}

	// Retry navigation up to 3 times on timeout errors.
	// The LMS server can be slow to respond, especially on subsequent
	// requests after a previous page was closed.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Wait before retry to let the server recover.
			time.Sleep(time.Duration(attempt) * time.Second)
			// Reset page state again before retry.
			_ = s.page.Timeout(pageTimeout).Navigate("about:blank")
		}

		page := s.page.Timeout(pageTimeout)
		if err := page.Navigate(url); err != nil {
			lastErr = fmt.Errorf("navigate to %s (attempt %d): %w", url, attempt+1, err)
			// Check if this is a timeout error that we should retry.
			if strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "timeout") {
				continue
			}
			// Non-timeout error, don't retry.
			return lastErr
		}
		if err := page.WaitLoad(); err != nil {
			lastErr = fmt.Errorf("wait load %s (attempt %d): %w", url, attempt+1, err)
			if strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "timeout") {
				continue
			}
			return lastErr
		}
		// Success.
		return nil
	}
	return fmt.Errorf("navigate to %s failed after 3 attempts: %w", url, lastErr)
}

// Eval executes JavaScript on the page and returns the result as a string.
func (s *rodSession) Eval(js string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	page := s.page.Timeout(pageTimeout)
	result, err := page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("eval js: %w", err)
	}
	return result.Value.Str(), nil
}

// ElementAttribute returns the value of an attribute on the first element matching the selector.
func (s *rodSession) ElementAttribute(selector, attr string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	page := s.page.Timeout(pageTimeout)
	el, err := page.Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element %s: %w", selector, err)
	}
	val, err := el.Attribute(attr)
	if err != nil {
		return "", fmt.Errorf("get attribute %s: %w", attr, err)
	}
	if val == nil {
		return "", fmt.Errorf("attribute %s is nil on %s", attr, selector)
	}
	return *val, nil
}

// ElementExists returns true if an element matching the selector exists on the page.
func (s *rodSession) ElementExists(selector string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	page := s.page.Timeout(pageTimeout)
	_, err := page.Element(selector)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// ElementHref returns the href attribute of the first element matching the selector.
func (s *rodSession) ElementHref(selector string) (string, error) {
	return s.ElementAttribute(selector, "href")
}

// DownloadPDF downloads a PDF from the given URL and saves it to savePath.
func (s *rodSession) DownloadPDF(url, savePath string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	page := s.page.Timeout(pageTimeout)
	return browser.DownloadAndSave(page, url, savePath)
}

// DownloadImage downloads an image from the given URL and saves it to savePath.
func (s *rodSession) DownloadImage(url, savePath string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	page := s.page.Timeout(pageTimeout)
	return browser.DownloadImage(page, url, savePath)
}

// Close closes the page and signals that this session holder is done with
// the browser session, decrementing the active use count so the session
// can be evicted if needed. It is safe to call Close multiple times.
func (s *rodSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	// Close the page to free browser resources.
	// Without this, the browser accumulates pages and eventually
	// fails with "context deadline exceeded" when creating new pages.
	if s.page != nil {
		pageTimeout := 5 * time.Second
		done := make(chan error, 1)
		go func() {
			done <- s.page.Close()
		}()
		select {
		case err := <-done:
			if err != nil {
				fmt.Printf("[SESSION] Page close error: %v\n", err)
			}
		case <-time.After(pageTimeout):
			fmt.Printf("[SESSION] Page close timed out after %v\n", pageTimeout)
		}
		// Decrement page count.
		s.cachedSess.pageMu.Lock()
		if s.cachedSess.pageCount > 0 {
			s.cachedSess.pageCount--
		}
		fmt.Printf("[SESSION] Page closed, pageCount=%d\n", s.cachedSess.pageCount)
		s.cachedSess.pageMu.Unlock()
	}

	s.cachedSess.mu.Lock()
	if s.cachedSess.activeCount > 0 {
		s.cachedSess.activeCount--
	}
	s.cachedSess.mu.Unlock()
	return nil
}

// Ensure rodSession implements the expected interface at compile time.
var _ interface {
	Navigate(string) error
	Eval(string) (string, error)
	ElementAttribute(string, string) (string, error)
	ElementExists(string) (bool, error)
	ElementHref(string) (string, error)
	DownloadPDF(string, string) (string, int, error)
	DownloadImage(string, string) (string, int, error)
	Close() error
} = (*rodSession)(nil)

// Ensure selectors are used (prevents import cycle if browser package changes).
var _ = browser.SelUsernameInput
