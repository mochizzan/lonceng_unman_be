package session_test

import (
	"testing"
	"time"

	browserInfra "lonceng_unman_be/internal/infrastructure/browser"
	"lonceng_unman_be/internal/infrastructure/session"

	"github.com/go-rod/rod"
)

func TestMarkStale_EvictsAndNextGetOrCreateFresh(t *testing.T) {
	m := session.NewManager(newTTLConfig(15*time.Minute, 20))
	defer m.Stop()
	npm := "2211700006"
	m.InjectTestSession(npm, &browserInfra.Browser{}, &rod.Page{})
	if m.SessionCount() != 1 {
		t.Fatalf("inject want 1 got %d", m.SessionCount())
	}
	if !m.MarkStale(npm) {
		t.Fatalf("MarkStale want true for existing")
	}
	if m.SessionCount() != 0 {
		t.Fatalf("after MarkStale want 0 got %d", m.SessionCount())
	}
	// Next GetOrCreate should create fresh (inject again simulates fresh)
	m.InjectTestSession(npm, &browserInfra.Browser{}, &rod.Page{})
	if m.SessionCount() != 1 {
		t.Fatalf("after re-inject want 1 got %d", m.SessionCount())
	}
}

func TestMarkStale_Idempotent(t *testing.T) {
	m := session.NewManager(newTTLConfig(15*time.Minute, 20))
	defer m.Stop()
	npm := "2211700006"
	m.InjectTestSession(npm, &browserInfra.Browser{}, &rod.Page{})
	if !m.MarkStale(npm) {
		t.Fatalf("first MarkStale want true")
	}
	if m.MarkStale(npm) {
		t.Fatalf("second MarkStale want false idempotent")
	}
	if m.SessionCount() != 0 {
		t.Fatalf("after double MarkStale want 0 got %d", m.SessionCount())
	}
}

func TestIsTransient_ClosedNetwork(t *testing.T) {
	// browser.IsTransientBrowserError covers closed network/closed/eof/timeout/deadline
	cases := []struct {
		msg  string
		want bool
	}{
		{"write tcp 127.0.0.1:36283->127.0.0.1:1234: use of closed network connection", true},
		{"closed", true},
		{"EOF", true},
		{"context deadline exceeded", true},
		{"i/o timeout", true},
		{"context canceled", false},
	}
	for _, c := range cases {
		got := browserInfra.IsTransientBrowserError(errWithMsg(c.msg))
		if got != c.want {
			t.Errorf("IsTransient %q want %v got %v", c.msg, c.want, got)
		}
	}
}

func errWithMsg(s string) error { return testErr{s} }

type testErr struct{ s string }

func (e testErr) Error() string { return e.s }
