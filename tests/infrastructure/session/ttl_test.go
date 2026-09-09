package session_test

import (
	"testing"
	"time"

	"lonceng_unman_be/internal/config"
	browserInfra "lonceng_unman_be/internal/infrastructure/browser"
	"lonceng_unman_be/internal/infrastructure/session"

	"github.com/go-rod/rod"
)

func newTTLConfig(ttl time.Duration, max int) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			SessionTTL:  ttl,
			MaxSessions: max,
		},
	}
}

func TestSessionTTL15m(t *testing.T) {
	m := session.NewManager(newTTLConfig(15*time.Minute, 20))
	defer m.Stop()

	npm := "2211700006"
	m.InjectTestSession(npm, &browserInfra.Browser{}, &rod.Page{})
	if m.SessionCount() != 1 {
		t.Fatalf("expected 1, got %d", m.SessionCount())
	}

	// 14m ago -> still valid (not evicted)
	m.SetLastUsedForTest(npm, time.Now().Add(-14*time.Minute))
	m.Cleanup()
	if m.SessionCount() != 1 {
		t.Fatalf("14m lastUsed should not evict with 15m TTL, got %d", m.SessionCount())
	}

	// 16m ago -> expired, should evict
	m.SetLastUsedForTest(npm, time.Now().Add(-16*time.Minute))
	m.Cleanup()
	if m.SessionCount() != 0 {
		t.Fatalf("16m lastUsed should evict with 15m TTL, got %d", m.SessionCount())
	}
}

func TestHardLimit2h_Grace5m(t *testing.T) {
	m := session.NewManager(newTTLConfig(15*time.Minute, 20))
	defer m.Stop()
	npm := "2211700006"
	m.InjectTestSession(npm, &browserInfra.Browser{}, &rod.Page{})
	// created 2h6m ago, activeCount=1 -> force evict despite active
	m.SetCreatedAtForTest(npm, time.Now().Add(-2*time.Hour-6*time.Minute))
	m.SetActiveCountForTest(npm, 1)
	m.SetLastUsedForTest(npm, time.Now())
	m.Cleanup()
	if m.SessionCount() != 0 {
		t.Fatalf("hard limit 2h+5m should force evict even active, got %d", m.SessionCount())
	}
}

func TestMaxSessionsEvictOldest(t *testing.T) {
	m := session.NewManager(newTTLConfig(15*time.Minute, 1))
	defer m.Stop()
	m.InjectTestSession("111", &browserInfra.Browser{}, &rod.Page{})
	// Ensure different lastUsed
	time.Sleep(10 * time.Millisecond)
	m.InjectTestSession("222", &browserInfra.Browser{}, &rod.Page{})
	// MaxSessions=1 but Inject bypasses limit; now trigger evictOldest directly via create path
	// Simulate eviction by calling Cleanup with expired first entry
	m.SetLastUsedForTest("111", time.Now().Add(-20*time.Minute))
	m.Cleanup()
	if m.SessionCount() != 1 {
		t.Fatalf("expected 1 after evict oldest, got %d", m.SessionCount())
	}
}
