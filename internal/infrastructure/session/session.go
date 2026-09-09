package session

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"lonceng_unman_be/internal/domain/port"
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
	page *rod.Page
	mu   sync.Mutex // per-session mutex for this rodSession's page ops
	// cachedSess is the owning cachedSession; rodSession is a per-request handle.
	// This back-reference is intentional and does not create an import cycle
	// because session imports browser, not vice versa.
	cachedSess *cachedSession
	closed     bool // prevents double-decrement of activeCount
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

	slog.Debug("[DIAG-PAGE] creating new page", "npm", cachedSess.npm, "pageCount", cachedSess.pageCount)
	page, err := cachedSess.browser.Page("about:blank")
	if err == nil {
		cachedSess.pageCount++
	}
	if err != nil {
		slog.Warn("Page() failed", "npm", cachedSess.npm, "pageCount", cachedSess.pageCount, "error", err)
	} else {
		slog.Debug("[DIAG-PAGE] page created", "npm", cachedSess.npm, "pageCount", cachedSess.pageCount)
	}
	cachedSess.pageMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("create new page: %w", err)
	}

	return &rodSession{
		page:       page,
		cachedSess: cachedSess,
	}, nil
}

// isTimeout reports whether err is a timeout/deadline failure worth retrying
// with a fresh page. Used to distinguish H1 (CDP target hang) from non-timeout errors.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "timeout")
}

// replacePage closes the current stale page and creates a fresh one from the
// shared browser. It is called while s.mu is held; it acquires pageMu internally.
// On timeout-induced Navigate failures (H1), the old CDP target is dead and
// reusing it guarantees subsequent attempts also fail. Replacing the page
// breaks the corruption chain without evicting the entire browser session.
func (s *rodSession) replacePage() error {
	oldPage := s.page
	if oldPage != nil {
		done := make(chan error, 1)
		go func() { done <- oldPage.Close() }()
		select {
		case err := <-done:
			if err != nil {
				slog.Warn("[DIAG-PAGE] replacePage close old failed", "npm", s.cachedSess.npm, "error", err)
			} else {
				slog.Debug("[DIAG-PAGE] replacePage close old ok", "npm", s.cachedSess.npm)
			}
		case <-time.After(5 * time.Second):
			slog.Warn("[DIAG-PAGE] replacePage close old timed out", "npm", s.cachedSess.npm, "timeout", 5*time.Second)
		}
		s.cachedSess.pageMu.Lock()
		if s.cachedSess.pageCount > 0 {
			s.cachedSess.pageCount--
		}
		slog.Debug("[DIAG-PAGE] replacePage decremented", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount)
		s.cachedSess.pageMu.Unlock()
	}
	s.cachedSess.pageMu.Lock()
	defer s.cachedSess.pageMu.Unlock()
	if s.cachedSess.pageCount >= maxPagesPerBrowser {
		return fmt.Errorf("browser has too many open pages (%d >= %d) — session corrupted, please retry", s.cachedSess.pageCount, maxPagesPerBrowser)
	}
	slog.Debug("[DIAG-PAGE] replacePage creating new page", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount)
	page, err := s.cachedSess.browser.Page("about:blank")
	if err != nil {
		slog.Warn("[DIAG-PAGE] replacePage create failed", "npm", s.cachedSess.npm, "error", err)
		return fmt.Errorf("replace page: %w", err)
	}
	s.cachedSess.pageCount++
	slog.Info("[DIAG-PAGE] replacePage success", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount)
	s.page = page
	return nil
}

// touchLastUsed updates the lastUsed timestamp of the cached session.
// Guards cachedSess.mu directly so it is safe to call while s.mu is held
// or without holding s.mu. Callers do not need to hold s.mu for this call.
func (s *rodSession) touchLastUsed() {
	s.cachedSess.mu.Lock()
	s.cachedSess.lastUsed = time.Now()
	s.cachedSess.mu.Unlock()
}

// Navigate loads the given URL and waits until a page-specific landmark element
// appears (Method D — e.g. form/KHS table) or, if unknown URL, until DOM
// interactive (fallback). Retries on timeout and resets via about:blank to
// avoid stale page reuse.
//
// [DIAG-NAV] instrumentation: every phase (reset, navigate, waitReady)
// logs elapsed latency — H1 (CDP hang), H2 (deadline), H4 (LMS hang).
func (s *rodSession) Navigate(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.touchLastUsed()

	// Reset page state by navigating to about:blank first.
	// This prevents the "context deadline exceeded" error that occurs
	// when a page is reused after the previous page was closed.
	navStart := time.Now()
	slog.Debug("[DIAG-NAV] reset start", "npm", s.cachedSess.npm, "url", url)
	page := s.page.Timeout(pageTimeout)
	if err := page.Navigate("about:blank"); err != nil {
		slog.Warn("[DIAG-NAV] reset failed", "npm", s.cachedSess.npm, "url", url, "elapsed", time.Since(navStart), "error", err)
		return fmt.Errorf("reset page state: %w", err)
	}
	slog.Debug("[DIAG-NAV] reset ok", "npm", s.cachedSess.npm, "elapsed", time.Since(navStart))

	// Retry navigation up to 3 times on timeout errors.
	// The LMS server can be slow to respond, especially on subsequent
	// requests after a previous page was closed.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Wait before retry to let the server recover.
			slog.Debug("[DIAG-NAV] retry backoff", "npm", s.cachedSess.npm, "attempt", attempt+1, "sleep", time.Duration(attempt)*time.Second)
			time.Sleep(time.Duration(attempt) * time.Second)
			// H1 fix: on timeout, the old CDP target is dead — replace the page
			// instead of reusing it. Without this, retry attempt 2/3 re-enters
			// the same hung target and also hits context deadline exceeded.
			if isTimeout(lastErr) {
				slog.Info("[DIAG-NAV] timeout on prior attempt, replacing page", "npm", s.cachedSess.npm, "attempt", attempt+1)
				if err := s.replacePage(); err != nil {
					return fmt.Errorf("replace page before retry: %w", err)
				}
			} else {
				// Non-timeout retry (should not happen — we return early below —
				// but keep a reset for completeness).
				_ = s.page.Timeout(pageTimeout).Navigate("about:blank")
			}
		}

		slog.Debug("[DIAG-NAV] navigate attempt start", "npm", s.cachedSess.npm, "url", url, "attempt", attempt+1)
		attemptStart := time.Now()
		page := s.page.Timeout(pageTimeout)
		if err := page.Navigate(url); err != nil {
			elapsed := time.Since(attemptStart)
			lastErr = fmt.Errorf("navigate to %s (attempt %d): %w", url, attempt+1, err)
			slog.Warn("[DIAG-NAV] navigate failed", "npm", s.cachedSess.npm, "url", url, "attempt", attempt+1, "elapsed", elapsed, "error", err)
			if isTimeout(err) {
				continue
			}
			// Non-timeout error, don't retry.
			return lastErr
		}
		slog.Debug("[DIAG-NAV] navigate ok, waiting ready", "npm", s.cachedSess.npm, "url", url, "attempt", attempt+1, "elapsed", time.Since(attemptStart))
		waitStart := time.Now()
		if err := s.waitReady(page, url); err != nil {
			elapsed := time.Since(waitStart)
			lastErr = fmt.Errorf("wait load %s (attempt %d): %w", url, attempt+1, err)
			slog.Warn("[DIAG-NAV] waitReady failed", "npm", s.cachedSess.npm, "url", url, "attempt", attempt+1, "elapsed", elapsed, "error", err)
			// If the page is a logout/login shell after waitReady timeout,
			// don't spin 3×15s — fail fast with ErrLMSExpired so the caller
			// can evict and re-login once (backend 24h vs LMS menit).
			if sel := navigateReadySelector(url); sel != "" {
				if s.isLMSExpiredProbe(page, sel) {
					return fmt.Errorf("%w: probe %q missing after %v: %w", ErrLMSExpired, sel, elapsed, lastErr)
				}
			}
			if isTimeout(err) {
				continue
			}
			return lastErr
		}
		slog.Info("[DIAG-NAV] navigate success", "npm", s.cachedSess.npm, "url", url, "attempt", attempt+1, "total_elapsed", time.Since(navStart))
		// Success.
		return nil
	}
	// If all 3 retries exhausted because waitElementReady timed out, check if it
	// is actually LMS expiry (s.page is the last attempt's page — replacePage
	// keeps s.page in sync). Probe comment: s.page valid here because each
	// attempt either reused or replaced it; if Navigate never succeeded,
	// probe returns false (generic exhausted is correct).
	if sel := navigateReadySelector(url); sel != "" {
		if s.isLMSExpiredProbe(s.page, sel) {
			slog.Warn("[DIAG-NAV] navigate exhausted but LMS expired — bubbling sentinel", "npm", s.cachedSess.npm, "url", url, "total_elapsed", time.Since(navStart))
			return fmt.Errorf("%w: after 3 attempts %v: %w", ErrLMSExpired, sel, lastErr)
		}
	}
	slog.Warn("[DIAG-NAV] navigate exhausted", "npm", s.cachedSess.npm, "url", url, "total_elapsed", time.Since(navStart), "lastErr", lastErr)
	return fmt.Errorf("navigate to %s failed after 3 attempts: %w", url, lastErr)
}

// navigateReadySelector returns the element selector to wait for after Navigate(url).
// Methode D: condition-based wait on a specific DOM element, bukan window.onload.
// Map URL → selector spesifik halaman (lebih presisi dari waitDomReady).
// Jika URL tidak match map, fallback ke waitDomReady (poll readyState).
func navigateReadySelector(url string) string {
	switch {
	case strings.Contains(url, "op=data_mahasiswa&act=viewupdate"):
		return "form" // student-profile viewupdate: mega-form 55 field
	case strings.Contains(url, "op=master_mahasiswa&act=konversi_upd_mhs"):
		return "input[name='semester']" // KRS page: SelKRSSemesterInput
	case strings.Contains(url, "op=mahasiswa_khs&act=cetak"):
		// cetak (list) dan cetak_detail share prefix — dibedakan order:
		if strings.Contains(url, "cetak_detail") {
			return "a[href*='khs_pdf.php']" // KHS detail: CETAK button SelKHSCetakBtn
		}
		return ".table-bordered" // KHS list: SelKHSTable
	default:
		return "" // fallback → waitDomReady
	}
}

// ErrLMSExpired aliases port.ErrLMSExpired for callers that already import session.
// New code should import port.ErrLMSExpired directly.
var ErrLMSExpired = port.ErrLMSExpired

// isElementTimeout reports a hard CDP/page timeout (target detached/hung) that
// should abort immediately. A plain "element not found" must NOT be treated
// as hard timeout — it means the DOM is up but selector is absent (e.g.
// logout page has no form). The previous bug treated the 3s per-poll
// Element timeout as hard deadline and returned after 3s instead of polling
// until elemTimeout 15s.
func isElementTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Rod returns "context deadline exceeded" for both poll timeout and page
	// hang. Distinguish: "target closed" / "detached" / "browser has been closed"
	// are genuine CDP failures → hard abort. Plain element-not-found contains
	// "could not find element" / "element not found" → soft, keep polling.
	if strings.Contains(msg, "target closed") ||
		strings.Contains(msg, "detached") ||
		strings.Contains(msg, "browser has been closed") ||
		strings.Contains(msg, "session closed") {
		return true
	}
	// If it is a plain 3s poll timeout without CDP detachment, let the outer
	// 15s loop continue — the form may appear late.
	return false
}

// waitElementReady polls until selector exists in DOM (method D).
// Lebih spesifik dari waitDomReady — yang ditunggu element, bukan readyState.
func (s *rodSession) waitElementReady(page *rod.Page, selector string) error {
	const elemTimeout = 15 * time.Second
	const pollInterval = 200 * time.Millisecond
	const pollTimeout = 3 * time.Second
	deadline := time.Now().Add(elemTimeout)
	for time.Now().Before(deadline) {
		el, err := page.Timeout(pollTimeout).Element(selector)
		if err == nil && el != nil {
			slog.Debug("[DIAG-NAV] element ready", "npm", s.cachedSess.npm, "selector", selector)
			return nil
		}
		if err != nil && isElementTimeout(err) {
			return err
		}
		// Not found / poll timeout (expected until DOM paints) — poll.
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("wait element %q: context deadline exceeded (not found for %v)", selector, elemTimeout)
}

// waitDomReady polls document.readyState until it leaves "loading"
// (interactive or complete) — the DOM is then usable for ElementExists/Eval.
// Fallback jika URL tidak punya selector spesifik di navigateReadySelector.
// See: rod lib/js/helper.js waitLoad() = Promise(window.addEventListener('load'))
// vs. document.readyState polling.
func (s *rodSession) waitDomReady(page *rod.Page) error {
	const domReadyTimeout = 15 * time.Second
	const pollInterval = 200 * time.Millisecond
	const pollTimeout = 3 * time.Second
	deadline := time.Now().Add(domReadyTimeout)
	for time.Now().Before(deadline) {
		result, err := page.Timeout(pollTimeout).Eval(`() => document.readyState`)
		if err != nil {
			if isElementTimeout(err) {
				return err
			}
			// Transient eval error (e.g. execution context not yet created
			// during navigation commit) — brief backoff then retry.
			time.Sleep(pollInterval)
			continue
		}
		raw := result.Value.Str()
		// rod Eval returns JSON-encoded string: "\"interactive\""
		raw = strings.Trim(raw, "\"' ")
		raw = strings.ToLower(raw)
		if raw == "interactive" || raw == "complete" {
			slog.Debug("[DIAG-NAV] dom ready", "npm", s.cachedSess.npm, "readyState", raw)
			return nil
		}
		// Still "loading" (or empty during commit) — poll.
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("wait dom ready: context deadline exceeded (readyState stayed loading for %v)", domReadyTimeout)
}

// isLMSExpiredProbe checks whether the current page is a logout/login shell
// instead of the requested resource. LMS PHP expired returns 200 HTML with
// `alert('Anda Sudah Logout')` / `.alert-danger` / href logout and no target
// landmark, not a 302. Lightweight: one Eval + one Info (each 3s capped).
func (s *rodSession) isLMSExpiredProbe(page *rod.Page, expectedSelector string) bool {
	// Fast URL check first (no Eval cost if already redirected).
	if info, err := page.Info(); err == nil {
		u := strings.ToLower(info.URL)
		if strings.Contains(u, "logout") || strings.Contains(u, "op=login") {
			slog.Info("[DIAG-NAV] lms expired detected via URL", "npm", s.cachedSess.npm, "url", info.URL)
			return true
		}
	}
	// HTML content check — LMS expired pages contain alert() + no expected element.
	selJSON, _ := json.Marshal(expectedSelector)
	js := fmt.Sprintf(`() => {
		const sel = %s;
		const h = document.documentElement ? document.documentElement.outerHTML : "";
		const hasAlert = h.includes("Anda Sudah Logout") || h.includes("Sesi berakhir") || h.includes("Sudah Logout") || h.includes("session expired") || h.includes("alert-danger");
		let hasExpected = false;
		try { hasExpected = !!document.querySelector(sel); } catch(e) {}
		return JSON.stringify({hasAlert, hasExpected, len: h.length});
	}`, string(selJSON))
	res, err := page.Timeout(3 * time.Second).Eval(js)
	if err != nil {
		return false
	}
	raw := strings.ToLower(res.Value.Str())
	// raw is JSON-encoded string like "\"{\\\"hasAlert\\\":true,...}\""
	if strings.Contains(raw, "hasalert") && strings.Contains(raw, "true") {
		// Confirm missing expected element to avoid false positive on error banners inside real page.
		if strings.Contains(raw, "hasexpected") && strings.Contains(raw, "false") {
			slog.Info("[DIAG-NAV] lms expired detected via HTML alert + missing selector", "npm", s.cachedSess.npm, "selector", expectedSelector)
			return true
		}
		// Even without confirming absence, an alert-danger on the page is strong signal.
		if strings.Contains(raw, "alert-danger") {
			slog.Info("[DIAG-NAV] lms expired detected via alert-danger", "npm", s.cachedSess.npm)
			return true
		}
	}
	return false
}

// waitReady dispatches to method D (element-specific) or fallback B (domReady).
func (s *rodSession) waitReady(page *rod.Page, url string) error {
	if sel := navigateReadySelector(url); sel != "" {
		return s.waitElementReady(page, sel)
	}
	return s.waitDomReady(page)
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
				slog.Warn("Page close error", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount, "error", err)
			} else {
				slog.Debug("[DIAG-PAGE] page close ok", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount)
			}
		case <-time.After(pageTimeout):
			slog.Warn("Page close timed out", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount, "timeout", pageTimeout)
		}
		// Decrement page count.
		s.cachedSess.pageMu.Lock()
		if s.cachedSess.pageCount > 0 {
			s.cachedSess.pageCount--
		}
		slog.Debug("[DIAG-PAGE] page closed", "npm", s.cachedSess.npm, "pageCount", s.cachedSess.pageCount)
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
