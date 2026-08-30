package session_test

import (
	"sync"
	"testing"
	"time"

	"lonceng_unman_be/internal/config"
	browserInfra "lonceng_unman_be/internal/infrastructure/browser"
	"lonceng_unman_be/internal/infrastructure/session"

	"github.com/go-rod/rod"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockBrowser wraps the real Browser type but never connects.
type mockBrowser struct {
	*browserInfra.Browser
	mu      sync.Mutex
	closed  bool
	closeFn func() error
}

func newMockBrowser() *mockBrowser {
	return &mockBrowser{
		Browser: &browserInfra.Browser{},
	}
}

func (m *mockBrowser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func (m *mockBrowser) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestConfig(ttl time.Duration, maxSessions int) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			SessionTTL:  ttl,
			MaxSessions: maxSessions,
		},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestManager_GetOrCreate_ParallelSameNPM(t *testing.T) {
	// This test verifies that concurrent GetOrCreate calls with the same NPM
	// only create one session (the per-NPM lock ensures serialization).
	// We inject a session first so GetOrCreate reuses it instead of calling
	// the browser factory (which would fail without a real Chrome).
	m := session.NewManager(newTestConfig(15*time.Minute, 100))
	defer m.Stop()

	npm := "2211700006"

	// Inject a session directly so GetOrCreate can reuse it
	br := &browserInfra.Browser{}
	page := &rod.Page{}
	m.InjectTestSession(npm, br, page)

	var wg sync.WaitGroup
	const numGoroutines = 50
	errs := make([]error, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := m.GetOrCreate(npm, "test123")
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// Verify all calls succeeded
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: GetOrCreate error: %v", i, err)
		}
	}

	// Verify only ONE session exists for this NPM
	if m.SessionCount() != 1 {
		t.Errorf("expected 1 session in map, got %d", m.SessionCount())
	}
}

func TestManager_Close_DoubleClose(t *testing.T) {
	// This test verifies that calling Close() twice does not cause
	// activeCount to underflow or panic.
	m := session.NewManager(newTestConfig(15*time.Minute, 100))
	defer m.Stop()

	npm := "2211700006"

	// Inject a session directly
	br := &browserInfra.Browser{}
	page := &rod.Page{}
	m.InjectTestSession(npm, br, page)

	// Get the session to create a rodSession wrapper
	sess, err := m.GetOrCreate(npm, "")
	if err != nil {
		t.Fatalf("GetOrCreate error: %v", err)
	}
	if sess == nil {
		t.Fatal("GetOrCreate returned nil session")
	}

	// First Close: should decrement activeCount
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close error: %v", err)
	}

	// Second Close: should be a no-op, not panic or underflow
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}

	// Verify no panic occurred and session is still in the map
	if m.SessionCount() != 1 {
		t.Errorf("expected session to remain in map after rodSession.Close, got %d", m.SessionCount())
	}

	// Now close via Manager.Close — should succeed
	if err := m.Close(npm); err != nil {
		t.Fatalf("Manager.Close error: %v", err)
	}

	// Second Manager.Close — should be no-op (session not in map)
	if err := m.Close(npm); err != nil {
		t.Fatalf("second Manager.Close error: %v", err)
	}
}

func TestManager_Cleanup_EvictExpired(t *testing.T) {
	// This test verifies that the cleanup logic evicts expired sessions.
	// We use a very short TTL and wait for the session to expire.
	m := session.NewManager(newTestConfig(100*time.Millisecond, 100))
	defer m.Stop()

	npm := "2211700006"
	br := &browserInfra.Browser{}
	page := &rod.Page{}
	m.InjectTestSession(npm, br, page)

	// Wait for session to expire
	time.Sleep(200 * time.Millisecond)

	// Trigger cleanup
	m.Cleanup()

	// Verify session was evicted
	if m.SessionCount() != 0 {
		t.Errorf("expected 0 sessions after cleanup, got %d", m.SessionCount())
	}
}

func TestManager_CloseAll(t *testing.T) {
	// This test verifies that CloseAll properly closes all sessions.
	m := session.NewManager(newTestConfig(15*time.Minute, 100))
	defer m.Stop()

	// Inject 3 sessions
	for i := range 3 {
		npm := "221170000" + string(rune('1'+i))
		br := newMockBrowser()
		page := &rod.Page{}
		m.InjectTestSession(npm, br.Browser, page)
	}

	if m.SessionCount() != 3 {
		t.Fatalf("expected 3 sessions, got %d", m.SessionCount())
	}

	// CloseAll should close all sessions
	m.CloseAll()

	// Verify all sessions are gone
	if m.SessionCount() != 0 {
		t.Errorf("expected 0 sessions after CloseAll, got %d", m.SessionCount())
	}
}
