package session

import (
	"fmt"
	"sync"
	"sync/atomic"

	"lonceng_unman_be/internal/infrastructure/browser"

	"github.com/go-rod/rod"
)

// rodSession implements port.BrowserSession using a go-rod page.
// Each page operation is serialized via mu to prevent concurrent
// access to the shared rod.Page from multiple request goroutines.
// The activeCount on cachedSession tracks whether any rodSession
// is still in use, preventing eviction during active requests.
type rodSession struct {
	page       *rod.Page
	mu         *sync.Mutex    // shared with cachedSession; serializes page ops
	cachedSess *cachedSession // back-reference for release tracking
}

// newSession wraps a cachedSession's page into a BrowserSession.
// The caller must not use cachedSess.page directly after this call.
func newSession(cachedSess *cachedSession) *rodSession {
	return &rodSession{
		page:       cachedSess.page,
		mu:         &cachedSess.mu,
		cachedSess: cachedSess,
	}
}

// Navigate loads the given URL and waits for the page to be ready.
// It acquires the session mutex so only one request uses the page at a time.
func (s *rodSession) Navigate(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.page.Navigate(url); err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}
	if err := s.page.WaitLoad(); err != nil {
		return fmt.Errorf("wait load %s: %w", url, err)
	}
	return nil
}

// Eval executes JavaScript on the page and returns the result as a string.
// It acquires the session mutex so only one request uses the page at a time.
func (s *rodSession) Eval(js string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("eval js: %w", err)
	}
	return result.Value.Str(), nil
}

// ElementAttribute returns the value of an attribute on the first element matching the selector.
// It acquires the session mutex so only one request uses the page at a time.
func (s *rodSession) ElementAttribute(selector, attr string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	el, err := s.page.Element(selector)
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
// It acquires the session mutex so only one request uses the page at a time.
func (s *rodSession) ElementExists(selector string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.page.Element(selector)
	if err != nil {
		// go-rod returns error when element not found
		return false, nil
	}
	return true, nil
}

// ElementHref returns the href attribute of the first element matching the selector.
func (s *rodSession) ElementHref(selector string) (string, error) {
	return s.ElementAttribute(selector, "href")
}

// DownloadPDF downloads a PDF from the given URL and saves it to savePath.
// Delegates to browser.DownloadAndSave for the actual fetch and file I/O.
// It acquires the session mutex so only one request uses the page at a time.
func (s *rodSession) DownloadPDF(url, savePath string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return browser.DownloadAndSave(s.page, url, savePath)
}

// Close signals that this session holder is done with the browser session,
// decrementing the active use count so the session can be evicted if needed.
// The underlying browser and page are managed by the session manager.
func (s *rodSession) Close() error {
	atomic.AddInt32(&s.cachedSess.activeCount, -1)
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
	Close() error
} = (*rodSession)(nil)

// Ensure selectors are used (prevents import cycle if browser package changes).
var _ = browser.SelUsernameInput
