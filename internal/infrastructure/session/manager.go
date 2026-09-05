package session

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/port"
	browserInfra "lonceng_unman_be/internal/infrastructure/browser"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	// maxSessionLifetime adalah hard limit untuk session, terlepas dari aktivitas.
	// Mencegah memory leak untuk session yang sangat lama aktif.
	maxSessionLifetime = 2 * time.Hour

	// gracePeriod adalah waktu tambahan untuk session yang sedang aktif
	// sebelum di-force evict.
	gracePeriod = 5 * time.Minute
)

// cachedSession holds an authenticated browser for a single NPM.
//
// Architecture: The browser maintains authentication state (cookies via
// UserDataDir). Each request creates a fresh rod.Page from the browser
// via browser.Page("about:blank"). This avoids the "stale page state" bug
// where reusing a single page across requests caused subsequent navigations
// to fail immediately (3-10ms) after the first successful scrape.
//
// activeCount tracks how many request goroutines currently hold a rodSession
// derived from this cachedSession. Eviction and cleanup must not close a
// session whose activeCount > 0.
type cachedSession struct {
	npm         string
	browser     *browserInfra.Browser
	createdAt   time.Time
	lastUsed    time.Time
	mu          sync.Mutex // guards lastUsed, AND activeCount
	pageMu      sync.Mutex // serializes browser.Page() calls
	activeCount int32      // protected by mu, NOT atomic
	pageCount   int32      // number of open pages
}

// maxPagesPerBrowser limits the number of pages that can be open in a
// single browser instance. When this limit is exceeded, the browser is
// considered corrupted and a new session must be created.
const maxPagesPerBrowser = 10

// Manager is an in-memory session cache keyed by NPM.
// It is safe for concurrent use by multiple goroutines.
type Manager struct {
	cfg            *config.Config
	mu             sync.RWMutex // guards sessions map only (read/write)
	sessions       map[string]*cachedSession
	npmLocks       sync.Map // map[string]*sync.Mutex, per-NPM lock for session creation
	ttl            time.Duration
	stopCh         chan struct{} // signals background cleanup to stop
	browserFactory func() *browserInfra.Browser
}

// NewManager creates a session manager with the given config.
// It starts a background goroutine that evicts expired sessions.
//
// The browserFactory wraps browserInfra.New so that each new browser
// receives the configured BrowserLaunchTimeout. This bounds the
// rod.New().ControlURL().Connect() window (Tier-2 fix). The launch itself
// (launcher.Launch()) does not take a context, but the post-launch
// DevTools dial and any subsequent page ops now share the configured cap.
func NewManager(cfg *config.Config) *Manager {
	ttl := cfg.App.SessionTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	launchTimeout := cfg.App.BrowserLaunchTimeout
	if launchTimeout <= 0 {
		launchTimeout = 60 * time.Second
	}

	m := &Manager{
		cfg:      cfg,
		sessions: make(map[string]*cachedSession),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
		browserFactory: func() *browserInfra.Browser {
			b := browserInfra.New()
			b.SetLaunchTimeout(launchTimeout)
			return b
		},
	}
	go m.cleanupLoop()
	return m
}

// NewManagerWithFactory creates a session manager with a custom browser factory.
// This is primarily used for testing to inject mock browsers. The factory is
// NOT wrapped with BrowserLaunchTimeout — tests typically don't need it.
func NewManagerWithFactory(cfg *config.Config, factory func() *browserInfra.Browser) *Manager {
	ttl := cfg.App.SessionTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	m := &Manager{
		cfg:            cfg,
		sessions:       make(map[string]*cachedSession),
		ttl:            ttl,
		stopCh:         make(chan struct{}),
		browserFactory: factory,
	}
	go m.cleanupLoop()
	return m
}

// SessionCount returns the number of sessions currently cached.
// This is primarily used for testing.
func (m *Manager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// InjectTestSession adds a session directly to the cache for testing purposes.
// It bypasses the normal creation flow and does not enforce MaxSessions.
func (m *Manager) InjectTestSession(npm string, browser *browserInfra.Browser, _ *rod.Page) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[npm] = &cachedSession{
		npm:       npm,
		browser:   browser,
		createdAt: now,
		lastUsed:  now,
	}
}

// Cleanup triggers the background cleanup logic immediately.
// This is primarily for testing to avoid waiting for the ticker.
func (m *Manager) Cleanup() {
	m.cleanup()
}

// getNPMLock returns a per-NPM mutex. Different NPMs can create sessions in parallel.
func (m *Manager) getNPMLock(npm string) *sync.Mutex {
	val, _ := m.npmLocks.LoadOrStore(npm, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// profileDir returns the Chrome profile directory path for the given NPM.
func (m *Manager) profileDir(npm string) string {
	return filepath.Join(m.cfg.App.ProfileBaseDir, npm)
}

// validateSession checks whether the restored session is still authenticated
// by navigating to the dashboard URL and checking for dashboard-specific DOM.
func (m *Manager) validateSession(page *rod.Page) error {
	page = page.Timeout(m.cfg.App.BrowserTimeout)

	dashboardURL := m.cfg.App.LMSDashboardURL
	if dashboardURL == "" {
		dashboardURL = "/admin/"
	}

	if err := page.Navigate(dashboardURL); err != nil {
		return fmt.Errorf("navigate to dashboard: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait dashboard load: %w", err)
	}

	info, err := page.Info()
	if err != nil {
		return fmt.Errorf("get page info: %w", err)
	}
	if !strings.Contains(info.URL, "/admin/") {
		return fmt.Errorf("session expired: redirected to %s", info.URL)
	}

	if _, err := page.Element(browserInfra.SelSuccessIndicator); err != nil {
		return fmt.Errorf("session expired: .wrapper not found on %s", info.URL)
	}

	return nil
}

// createSessionWithRestore launches Chrome with a restored profile,
// validates the session, and either returns it or falls back to full login.
//
// DNS pre-flight is performed at GetOrCreate top-level (single call). This
// function only handles browser launch + session validation.
func (m *Manager) createSessionWithRestore(npm, password string, profileDir string) (*cachedSession, error) {
	br := m.browserFactory()
	if err := br.ConnectWithProfile(m.cfg.App.BrowserHeadless, profileDir); err != nil {
		slog.Warn("profile launch failed, falling back to full login",
			"npm", npm, "error", err)
		return m.createSession(npm, password)
	}

	page, err := br.Page(m.cfg.App.LMSDashboardURL)
	if err != nil {
		_ = br.Close()
		return m.createSession(npm, password)
	}
	if err := page.WaitLoad(); err != nil {
		_ = br.Close()
		return m.createSession(npm, password)
	}

	if err := m.validateSession(page); err != nil {
		slog.Warn("restored session expired, performing full login",
			"npm", npm, "reason", err.Error())
		_ = br.Close()
		return m.createSession(npm, password)
	}

	slog.Info("restored session validated successfully", "npm", npm)
	now := time.Now()
	return &cachedSession{
		npm:       npm,
		browser:   br,
		createdAt: now,
		lastUsed:  now,
	}, nil
}

// GetOrCreate returns an existing valid session for the NPM,
// or creates a new one by logging in with the provided credentials.
//
// Each call creates a fresh rod.Page from the shared browser. The browser
// maintains authentication state (cookies via UserDataDir), so each new
// page is automatically authenticated.
//
// If the browser is corrupted (too many open pages, context deadline exceeded),
// the session is evicted and a new one is created.
func (m *Manager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	// Fast path: read-only check (concurrent across all NPMs).
	m.mu.RLock()
	sess, ok := m.sessions[npm]
	m.mu.RUnlock()

	if ok {
		sess.mu.Lock()
		// Read lastUsed under sess.mu to avoid data race with the
		// double-check block which writes lastUsed under the same lock.
		if time.Since(sess.lastUsed) < m.ttl {
			sess.lastUsed = time.Now()
			sess.activeCount++
			sess.mu.Unlock()
			slog.Info("session reused", "npm", npm)
			newSess, err := newSession(sess)
			if err != nil {
				// Rollback activeCount before eviction.
				sess.mu.Lock()
				if sess.activeCount > 0 {
					sess.activeCount--
				}
				sess.mu.Unlock()
				// H1-aware: Page() hang with "context deadline exceeded" means
				// the browser's CDP target is wedged — the session's pages are
				// all suspect. Evict at once so the next retry gets a fresh
				// browser instead of looping on the same dead target.
				if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") ||
					strings.Contains(strings.ToLower(err.Error()), "deadline") {
					slog.Warn("session corrupted (CDP hang), evicting", "npm", npm, "error", err)
				} else {
					slog.Warn("session corrupted, evicting", "npm", npm, "error", err)
				}
				m.evictSession(npm)
				return m.createNewSession(npm, password)
			}
			return newSess, nil
		}
		sess.mu.Unlock()
	}

	// Per-NPM lock: NPM-A does not block NPM-B.
	npmMu := m.getNPMLock(npm)
	npmMu.Lock()
	defer npmMu.Unlock()

	// Double-check after acquiring per-NPM lock.
	m.mu.RLock()
	sess, ok = m.sessions[npm]
	m.mu.RUnlock()

	if ok {
		sess.mu.Lock()
		// Read lastUsed under sess.mu to avoid data race with the
		// fast-path block which writes lastUsed under the same lock.
		if time.Since(sess.lastUsed) < m.ttl {
			sess.lastUsed = time.Now()
			sess.activeCount++
			sess.mu.Unlock()
			slog.Info("session reused (double-check)", "npm", npm)
			newSess, err := newSession(sess)
			if err != nil {
				// Rollback activeCount before eviction.
				sess.mu.Lock()
				if sess.activeCount > 0 {
					sess.activeCount--
				}
				sess.mu.Unlock()
				if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") ||
					strings.Contains(strings.ToLower(err.Error()), "deadline") {
					slog.Warn("session corrupted (CDP hang), evicting", "npm", npm, "error", err)
				} else {
					slog.Warn("session corrupted, evicting", "npm", npm, "error", err)
				}
				m.evictSession(npm)
				return m.createNewSession(npm, password)
			}
			return newSess, nil
		}
		sess.mu.Unlock()
	}

	// Evict expired session for this NPM if it exists in map.
	if ok {
		sess.mu.Lock()
		_ = sess.browser.Close()
		sess.mu.Unlock()
		m.mu.Lock()
		delete(m.sessions, npm)
		m.mu.Unlock()
	}

	return m.createNewSession(npm, password)
}

// createNewSession enforces max sessions, creates a new session, and stores it.
func (m *Manager) createNewSession(npm, password string) (port.BrowserSession, error) {
	// Enforce max sessions limit.
	if m.cfg.App.MaxSessions > 0 {
		m.mu.RLock()
		count := len(m.sessions)
		m.mu.RUnlock()
		if count >= m.cfg.App.MaxSessions {
			m.evictOldest()
		}
	}

	// Try restore from disk before full login.
	profileDir := m.profileDir(npm)

	// Tier-2 fix: single DNS pre-flight per cold-start attempt. Previously
	// this ran twice on the restore→fallback path (once in
	// createSessionWithRestore, once in createSession). The OS DNS cache
	// absorbs the second call but it's still wasted work; more importantly,
	// we want a fail-fast at the top level so callers see a clear error
	// instead of two timeouts chained together.
	if err := m.CheckDNS(m.cfg.App.LMSBaseURL); err != nil {
		return nil, err
	}

	var err error
	var sess *cachedSession
	if _, statErr := os.Stat(profileDir); statErr == nil {
		sess, err = m.createSessionWithRestore(npm, password, profileDir)
	} else {
		// No profile on disk — full login.
		slog.Info("creating new session", "npm", npm)
		sess, err = m.createSession(npm, password)
	}
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.sessions[npm] = sess
	m.mu.Unlock()

	sess.mu.Lock()
	sess.activeCount = 1
	sess.mu.Unlock()
	newSess, err := newSession(sess)
	if err != nil {
		// Rollback: activeCount was incremented but newSession failed;
		// clean up the map entry so we don't leak a zombie session.
		sess.mu.Lock()
		if sess.activeCount > 0 {
			sess.activeCount--
		}
		sess.mu.Unlock()
		m.mu.Lock()
		// Only delete if still our sess (no concurrent replacement).
		if cur, ok := m.sessions[npm]; ok && cur == sess {
			delete(m.sessions, npm)
		}
		m.mu.Unlock()
		// Best-effort browser close outside locks.
		sess.mu.Lock()
		_ = sess.browser.Close()
		sess.mu.Unlock()
		return nil, err
	}
	return newSess, nil
}

// evictSession removes a specific NPM session from the cache.
func (m *Manager) evictSession(npm string) {
	m.mu.Lock()
	sess, ok := m.sessions[npm]
	if ok {
		delete(m.sessions, npm)
	}
	m.mu.Unlock()

	if ok {
		sess.mu.Lock()
		_ = sess.browser.Close()
		sess.mu.Unlock()
		slog.Info("session evicted", "npm", npm)
	}
}

// Close releases the session for the given NPM.
func (m *Manager) Close(npm string) error {
	npmMu := m.getNPMLock(npm)
	npmMu.Lock()
	defer npmMu.Unlock()

	m.mu.RLock()
	sess, ok := m.sessions[npm]
	m.mu.RUnlock()
	if !ok {
		return nil
	}

	sess.mu.Lock()
	err := sess.browser.Close()
	sess.mu.Unlock()

	m.mu.Lock()
	delete(m.sessions, npm)
	m.mu.Unlock()

	slog.Info("session closed", "npm", npm)
	return err
}

// CloseAll releases all cached sessions.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	// Collect sessions to close, then release m.mu before calling Close()
	// so that browser.Close() does not block other goroutines waiting on m.mu.
	toClose := make([]*cachedSession, 0, len(m.sessions))
	for _, sess := range m.sessions {
		toClose = append(toClose, sess)
	}
	m.sessions = make(map[string]*cachedSession) // reset map under lock
	m.mu.Unlock()

	for _, sess := range toClose {
		sess.mu.Lock()
		_ = sess.browser.Close()
		sess.mu.Unlock()
	}
	slog.Info("all sessions closed")
}

// Stop signals the background cleanup goroutine to exit.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// IsTransientDNSError reports whether err is a transient DNS failure
// worth retrying (timeout, no such host, network unreachable). Parse
// errors and protocol errors fail immediately.
func IsTransientDNSError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "connection refused")
}

// CheckDNS verifies that the LMS hostname resolves before we attempt a
// browser connection. A broken DNS resolver (common after PC restart when
// the OS DNS cache is cleared) would otherwise cause a 30 s timeout
// inside go-rod with no actionable error message.
//
// Docker-internal DNS resolvers are notoriously flaky under high concurrent
// load, so transient failures (timeout, no such host) are retried up to
// maxAttempts times. Non-transient failures fail immediately.
//
// Exported as CheckDNS so tests in package session_test can exercise
// retry behavior without breaking encapsulation of unexported fields.
func (m *Manager) CheckDNS(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("DNS check: invalid LMS URL %q: %w", rawURL, err)
	}
	host := u.Hostname()
	if host == "" {
		// Stray test helper InjectTestSession used before with empty LMSBaseURL.
		// Skip DNS check for empty host — caller already verifies config.
		return nil
	}

	dnsTimeout := m.cfg.App.DNSTimeout
	if dnsTimeout == 0 {
		dnsTimeout = 5 * time.Second
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
		resolver := &net.Resolver{}
		addrs, err := resolver.LookupHost(ctx, host)
		cancel()
		if err == nil {
			if attempt > 0 {
				slog.Debug("DNS resolved after retry",
					"host", host, "addrs", addrs, "attempt", attempt+1)
			} else {
				slog.Debug("DNS resolved", "host", host, "addrs", addrs)
			}
			return nil
		}
		lastErr = err
		if !IsTransientDNSError(err) {
			break
		}
		slog.Warn("DNS check transient failure, will retry",
			"host", host, "attempt", attempt+1, "error", err)
	}
	return fmt.Errorf(
		"DNS check: cannot resolve LMS host %q after %d attempts: %w. "+
			"Verify your DNS settings (try setting DNS to 8.8.8.8)",
		host, maxAttempts, lastErr,
	)
}

// createSession launches a browser, logs in, and returns the cached session.
//
// DNS pre-flight is performed at GetOrCreate top-level (single call). This
// function only handles browser launch + login form submission.
//
// The login page is intentionally NOT closed after successful login.
// Closing the login page before cookies are fully persisted to the
// UserDataDir profile causes subsequent pages to lose authentication
// and redirect to the login page. Instead, we keep the login page open
// and create fresh pages for each request via browser.Page("about:blank").
// The browser maintains authentication state via the shared profile, so
// new pages are automatically authenticated once cookies are persisted.
func (m *Manager) createSession(npm, password string) (*cachedSession, error) {
	br := m.browserFactory()

	// Use persistent profile so cookies survive browser restarts.
	profileDir := m.profileDir(npm)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}

	if err := br.ConnectWithProfile(m.cfg.App.BrowserHeadless, profileDir); err != nil {
		return nil, fmt.Errorf("browser connect: %w", err)
	}

	page, err := br.Page(m.cfg.App.LMSBaseURL)
	if err != nil {
		_ = br.Close()
		return nil, fmt.Errorf("open login page: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		_ = br.Close()
		return nil, fmt.Errorf("wait login page load: %w", err)
	}

	// Perform login.
	if err := m.login(page, npm, password); err != nil {
		_ = br.Close()
		return nil, err
	}

	slog.Info("login successful", "npm", npm)

	// Do NOT close the login page. The browser's cookie jar is shared
	// across all pages in the same browser instance, but cookies may
	// not be fully persisted until the page is kept open. Closing the
	// login page can cause subsequent pages to lose authentication.
	// The login page will be closed when the browser is closed during
	// session eviction.

	now := time.Now()
	return &cachedSession{
		npm:       npm,
		browser:   br,
		createdAt: now,
		lastUsed:  now,
	}, nil
}

// login fills the login form and submits it, detecting success or failure.
func (m *Manager) login(page *rod.Page, npm, password string) error {
	page = page.Timeout(m.cfg.App.BrowserTimeout)

	usernameEl, err := page.Element(browserInfra.SelUsernameInput)
	if err != nil {
		return fmt.Errorf("find username field: %w", err)
	}
	if err := usernameEl.Input(npm); err != nil {
		return fmt.Errorf("fill username: %w", err)
	}

	passwordEl, err := page.Element(browserInfra.SelPasswordInput)
	if err != nil {
		return fmt.Errorf("find password field: %w", err)
	}
	if err := passwordEl.Input(password); err != nil {
		return fmt.Errorf("fill password: %w", err)
	}

	race := page.Race().
		Element(browserInfra.SelSuccessIndicator).
		Element(browserInfra.SelErrorIndicator)

	submitEl, err := page.Element(browserInfra.SelSubmitButton)
	if err != nil {
		return fmt.Errorf("find submit button: %w", err)
	}
	if err := submitEl.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("click submit: %w", err)
	}

	result, err := race.Do()
	if err != nil {
		return fmt.Errorf("login timed out: %w", err)
	}
	if result == nil {
		return fmt.Errorf("login timed out: no response detected")
	}

	matched, err := result.Matches(browserInfra.SelErrorIndicator)
	if err != nil {
		return fmt.Errorf("detect login result: %w", err)
	}
	if matched {
		errorText, err := result.Text()
		if err != nil {
			return fmt.Errorf("read login error text: %w", err)
		}
		return fmt.Errorf("%s", errorText)
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait dashboard load: %w", err)
	}

	info, err := page.Info()
	if err != nil {
		return fmt.Errorf("get page info: %w", err)
	}
	dashboardURL := m.cfg.App.LMSDashboardURL
	if dashboardURL == "" {
		dashboardURL = "/admin/"
	}
	if !strings.Contains(info.URL, dashboardURL) {
		return fmt.Errorf("page did not redirect to dashboard: %s", info.URL)
	}

	return nil
}

// evictOldest closes and removes the session with the oldest lastUsed time,
// skipping any session that is currently in active use.
func (m *Manager) evictOldest() {
	m.mu.Lock()

	var oldestNPM string
	var oldestTime time.Time
	for npm, sess := range m.sessions {
		sess.mu.Lock()
		isActive := sess.activeCount > 0
		lastUsed := sess.lastUsed
		sess.mu.Unlock()

		if isActive {
			continue
		}
		if oldestNPM == "" || lastUsed.Before(oldestTime) {
			oldestNPM = npm
			oldestTime = lastUsed
		}
	}

	if oldestNPM == "" {
		m.mu.Unlock()
		return
	}

	sess, ok := m.sessions[oldestNPM]
	if !ok {
		m.mu.Unlock()
		return
	}

	delete(m.sessions, oldestNPM)
	m.mu.Unlock() // release m.mu before Close()

	sess.mu.Lock()
	_ = sess.browser.Close()
	sess.mu.Unlock()

	slog.Info("evicted oldest session", "npm", oldestNPM)
}

// cleanupLoop periodically evicts expired sessions.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// cleanup removes all sessions that have exceeded the TTL.
// Sessions with active users (activeCount > 0) are skipped unless hard limit hit.
func (m *Manager) cleanup() {
	m.mu.Lock()
	now := time.Now()
	type evictionTarget struct {
		npm  string
		sess *cachedSession
	}
	var toEvict []evictionTarget
	for npm, sess := range m.sessions {
		sess.mu.Lock()
		isActive := sess.activeCount > 0
		lastUsed := sess.lastUsed
		createdAt := sess.createdAt
		sess.mu.Unlock()

		// Hard limit: force evict setelah maxSessionLifetime + gracePeriod
		if now.Sub(createdAt) > maxSessionLifetime+gracePeriod {
			if isActive {
				slog.Warn("session hit hard limit but active, force evicting",
					"npm", npm,
					"age", now.Sub(createdAt).Round(time.Second))
			}
			toEvict = append(toEvict, evictionTarget{npm, sess})
			continue
		}

		// Soft limit: TTL based on lastUsed + grace period untuk active sessions
		if now.Sub(lastUsed) > m.ttl {
			if isActive && now.Sub(lastUsed) < m.ttl+gracePeriod {
				slog.Info("session expired but within grace period, skipping",
					"npm", npm,
					"inactive_duration", now.Sub(lastUsed).Round(time.Second))
				continue
			}
			toEvict = append(toEvict, evictionTarget{npm, sess})
		}
	}

	// Remove evicted sessions from map under lock, then release m.mu
	// before calling browser.Close() so that Close() does not block
	// other goroutines waiting on m.mu.
	for _, t := range toEvict {
		delete(m.sessions, t.npm)
	}
	m.mu.Unlock()

	// Close browsers outside m.mu critical section.
	for _, t := range toEvict {
		// Double-check: pastikan masih expired dan tidak aktif
		t.sess.mu.Lock()
		if t.sess.activeCount > 0 || time.Since(t.sess.lastUsed) < m.ttl {
			t.sess.mu.Unlock()
			slog.Info("session revived before eviction, skipping", "npm", t.npm)
			continue
		}
		_ = t.sess.browser.Close()
		t.sess.mu.Unlock()
		slog.Info("session evicted", "npm", t.npm)
	}
}
