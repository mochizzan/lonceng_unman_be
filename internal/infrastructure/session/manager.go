package session

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/port"
	browserInfra "lonceng_unman_be/internal/infrastructure/browser"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// cachedSession holds an authenticated browser session for a single NPM.
// activeCount tracks how many request goroutines currently hold a rodSession
// derived from this cachedSession. Eviction and cleanup must not close a
// session whose activeCount > 0.
type cachedSession struct {
	page        *rod.Page
	browser     *browserInfra.Browser
	createdAt   time.Time
	lastUsed    time.Time
	mu          sync.Mutex // guards page operations within this session
	activeCount int32      // atomic: number of active rodSession holders
}

// Manager is an in-memory session cache keyed by NPM.
// It is safe for concurrent use by multiple goroutines.
type Manager struct {
	cfg      *config.Config
	mu       sync.RWMutex // guards the sessions map
	sessions map[string]*cachedSession
	ttl      time.Duration
	stopCh   chan struct{} // signals background cleanup to stop
}

// NewManager creates a session manager with the given config.
// It starts a background goroutine that evicts expired sessions.
func NewManager(cfg *config.Config) *Manager {
	ttl := cfg.App.SessionTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	m := &Manager{
		cfg:      cfg,
		sessions: make(map[string]*cachedSession),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

// GetOrCreate returns an existing valid session for the NPM,
// or creates a new one by logging in with the provided credentials.
func (m *Manager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	// Fast path: check if a valid session already exists (read lock only).
	m.mu.RLock()
	sess, ok := m.sessions[npm]
	m.mu.RUnlock()

	if ok && time.Since(sess.lastUsed) < m.ttl {
		sess.mu.Lock()
		sess.lastUsed = time.Now()
		sess.mu.Unlock()
		atomic.AddInt32(&sess.activeCount, 1)
		slog.Info("session reused", "npm", npm)
		return newSession(sess), nil
	}

	// Slow path: create a new session (write lock).
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have created it).
	if sess, ok := m.sessions[npm]; ok && time.Since(sess.lastUsed) < m.ttl {
		sess.mu.Lock()
		sess.lastUsed = time.Now()
		sess.mu.Unlock()
		atomic.AddInt32(&sess.activeCount, 1)
		slog.Info("session reused (double-check)", "npm", npm)
		return newSession(sess), nil
	}

	// Evict expired session if exists.
	if old, ok := m.sessions[npm]; ok {
		old.mu.Lock()
		_ = old.browser.Close()
		old.mu.Unlock()
		delete(m.sessions, npm)
	}

	// Enforce max sessions limit.
	if m.cfg.App.MaxSessions > 0 && len(m.sessions) >= m.cfg.App.MaxSessions {
		m.evictOldest()
	}

	// Create new browser and login.
	slog.Info("creating new session", "npm", npm)
	sess, err := m.createSession(npm, password)
	if err != nil {
		return nil, err
	}

	atomic.AddInt32(&sess.activeCount, 1)
	m.sessions[npm] = sess
	return newSession(sess), nil
}

// Close releases the session for the given NPM.
func (m *Manager) Close(npm string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[npm]; ok {
		sess.mu.Lock()
		err := sess.browser.Close()
		sess.mu.Unlock()
		delete(m.sessions, npm)
		slog.Info("session closed", "npm", npm)
		return err
	}
	return nil
}

// CloseAll releases all cached sessions.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for npm, sess := range m.sessions {
		sess.mu.Lock()
		_ = sess.browser.Close()
		sess.mu.Unlock()
		delete(m.sessions, npm)
	}
	slog.Info("all sessions closed")
}

// Stop signals the background cleanup goroutine to exit.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// checkDNS verifies that the LMS hostname resolves before we attempt a
// browser connection. A broken DNS resolver (common after PC restart when
// the OS DNS cache is cleared) would otherwise cause a 30 s timeout
// inside go-rod with no actionable error message.
func (m *Manager) checkDNS(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("DNS check: invalid LMS URL %q: %w", rawURL, err)
	}
	host := u.Hostname()

	dnsTimeout := m.cfg.App.DNSTimeout
	if dnsTimeout == 0 {
		dnsTimeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf(
			"DNS check: cannot resolve LMS host %q: %w. "+
				"Verify your DNS settings (try setting DNS to 8.8.8.8)",
			host, err,
		)
	}
	slog.Debug("DNS resolved", "host", host, "addrs", addrs)
	return nil
}

// createSession launches a browser, logs in, and returns the cached session.
func (m *Manager) createSession(npm, password string) (*cachedSession, error) {
	// DNS pre-flight: fail fast if the LMS host is unreachable,
	// avoiding a 30 s timeout when DNS is broken.
	if err := m.checkDNS(m.cfg.App.LMSBaseURL); err != nil {
		return nil, err
	}

	br := browserInfra.New()
	if err := br.Connect(m.cfg.App.BrowserHeadless); err != nil {
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
		return nil, fmt.Errorf("login: %w", err)
	}

	slog.Info("login successful", "npm", npm)

	now := time.Now()
	return &cachedSession{
		page:      page,
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
			return fmt.Errorf("login failed (could not read error text): %w", err)
		}
		return fmt.Errorf("login failed: %s", errorText)
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
		return fmt.Errorf("login failed: page did not redirect to dashboard (url: %s)", info.URL)
	}

	return nil
}

// evictOldest closes and removes the session with the oldest lastUsed time,
// skipping any session that is currently in active use.
func (m *Manager) evictOldest() {
	var oldestNPM string
	var oldestTime time.Time

	for npm, sess := range m.sessions {
		if atomic.LoadInt32(&sess.activeCount) > 0 {
			continue // don't evict active sessions
		}
		if oldestNPM == "" || sess.lastUsed.Before(oldestTime) {
			oldestNPM = npm
			oldestTime = sess.lastUsed
		}
	}

	if oldestNPM != "" {
		sess := m.sessions[oldestNPM]
		sess.mu.Lock()
		_ = sess.browser.Close()
		sess.mu.Unlock()
		delete(m.sessions, oldestNPM)
		slog.Info("evicted oldest session", "npm", oldestNPM)
	}
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
// Sessions with active users (activeCount > 0) are skipped.
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for npm, sess := range m.sessions {
		if now.Sub(sess.lastUsed) > m.ttl {
			if atomic.LoadInt32(&sess.activeCount) > 0 {
				slog.Info("session expired but still active, skipping", "npm", npm)
				continue
			}
			sess.mu.Lock()
			_ = sess.browser.Close()
			sess.mu.Unlock()
			delete(m.sessions, npm)
			slog.Info("session expired and evicted", "npm", npm)
		}
	}
}
