package session_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/session"
)

// newCheckDNSTestConfig builds a config with a specific DNSTimeout.
// The Manager constructor reads AppConfig.SessionTTL and AppConfig.BrowserLaunchTimeout;
// other fields are unused by checkDNS so default values are fine.
func newCheckDNSTestConfig(dnsTimeout time.Duration) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			DNSTimeout: dnsTimeout,
		},
	}
}

// Test 1: TestIsTransientDNSError — helper covers DNS-specific transient patterns
func TestIsTransientDNSError(t *testing.T) {
	cases := []struct {
		err      error
		name     string
		expected bool
	}{
		{nil, "nil", false},
		{errors.New("i/o timeout"), "i/o timeout", true},
		{errors.New("lookup foo: no such host"), "no such host", true},
		{errors.New("dial tcp: network is unreachable"), "network unreachable", true},
		{errors.New("dial tcp: connection refused"), "connection refused", true},
		{errors.New("invalid URL format"), "non-transient", false},
		{errors.New("permission denied"), "non-transient generic", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := session.IsTransientDNSError(tc.err)
			if got != tc.expected {
				t.Errorf("IsTransientDNSError(%v) = %v, want %v", tc.err, got, tc.expected)
			}
		})
	}
}

// Test 2: TestCheckDNS_ParseError_FailsImmediately — invalid URL short-circuits
// before any DNS lookup happens. Uses a real Manager constructed via the
// public constructor; checkDNS is invoked via the exported CheckDNS accessor
// added in Task 3.
func TestCheckDNS_ParseError_FailsImmediately(t *testing.T) {
	m := session.NewManager(newCheckDNSTestConfig(1 * time.Millisecond))
	defer m.Stop()

	err := m.CheckDNS("://invalid-url-no-scheme")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid LMS URL") {
		t.Errorf("error must indicate invalid URL, got %q", err.Error())
	}
}

// Test 3: TestCheckDNS_DefaultTimeoutApplied — DNSTimeout=0 falls back to
// the 5 s default; the retry loop must not stretch execution beyond 30 s.
func TestCheckDNS_DefaultTimeoutApplied(t *testing.T) {
	m := session.NewManager(newCheckDNSTestConfig(0))
	defer m.Stop()

	start := time.Now()
	_ = m.CheckDNS("http://0.0.0.0.invalid.:9999")
	elapsed := time.Since(start)
	if elapsed > 30*time.Second {
		t.Errorf("checkDNS took too long (%v); retry loop may be unbounded", elapsed)
	}
}

// Test 4: TestCheckDNS_RetryOnTransientError — verifies DNS transient errors
// are classified as retryable via the helper gate.
func TestCheckDNS_RetryOnTransientError(t *testing.T) {
	transient := session.IsTransientDNSError(errors.New("lookup elearning.universitasmandiri.ac.id: i/o timeout"))
	if !transient {
		t.Error("i/o timeout (real Docker DNS symptom) must be classified as transient")
	}
	transient = session.IsTransientDNSError(errors.New("no such host"))
	if !transient {
		t.Error("no such host must be classified as transient")
	}
}

// Test 5: TestCheckDNS_NonTransientErrorNoRetry — non-transient errors fail fast
func TestCheckDNS_NonTransientErrorNoRetry(t *testing.T) {
	nonTransient := session.IsTransientDNSError(errors.New("invalid URL format"))
	if nonTransient {
		t.Error("invalid URL format must NOT be classified as transient")
	}
	nonTransient = session.IsTransientDNSError(errors.New("permission denied"))
	if nonTransient {
		t.Error("permission denied must NOT be classified as transient")
	}
	nonTransient = session.IsTransientDNSError(nil)
	if nonTransient {
		t.Error("nil error must NOT be classified as transient")
	}
}
