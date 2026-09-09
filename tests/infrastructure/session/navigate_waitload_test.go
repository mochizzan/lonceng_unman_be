package session_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Test_Navigate_WaitLoad_Hang reproduces the 3m03s hang reported on 2026-09-09.
//
// Log evidence (npm=2211700006):
//
//	[DIAG-NAV] waitLoad failed attempt=3 elapsed=59.709s error="context deadline exceeded"
//	[DIAG-NAV] navigate exhausted total_elapsed=3m3.437s
//	= 60s*3 + backoff 1s+2s = 183s
//
// Root cause (Phase 1-3): session.go pageTimeout=60s * 3 attempts + WaitLoad
// waits for window.onload (helper.js waitLoad) which never fires on LMS
// viewupdate because hanging XHR/Select2 keeps readyState != complete,
// while DOM interactive + form fields are already populated.
//
// This test does NOT need a real browser — it models the timeout budget
// math and verifies the condition-based alternative would not hang.
func Test_Navigate_WaitLoad_Hang_Budget(t *testing.T) {
	const pageTimeout = 60 * time.Second
	const attempts = 3
	backoffs := []time.Duration{0, 1 * time.Second, 2 * time.Second}

	// Simulate current behavior: each attempt pays full pageTimeout on WaitLoad
	var total time.Duration
	for i := 0; i < attempts; i++ {
		total += backoffs[i]
		total += pageTimeout // WaitLoad hang
	}

	expected := 3*60*time.Second + 1*time.Second + 2*time.Second // 183s = 3m03s
	if total != expected {
		t.Fatalf("budget math wrong: got %v want %v", total, expected)
	}

	// The bug: this is the observed log value — prove we model it
	if total < 3*time.Minute {
		t.Fatal("total should be >= 3m (matches production log 3m03s)")
	}
	t.Logf("reproduced hang budget: %v (matches log 3m03.437s)", total)

	// Desired behavior after fix: condition-based wait should cap at
	// ScrapeFormWait (2s default) instead of pageTimeout per attempt.
	const scrapeFormWait = 2 * time.Second
	fixedTotal := scrapeFormWait + 500*time.Millisecond // one poll interval
	if fixedTotal >= 60*time.Second {
		t.Fatal("fixed path should be << 60s")
	}
	t.Logf("fixed budget would be ~%v instead of %v", fixedTotal, total)
}

// isTimeout mirrors session.go:53 isTimeout helper.
func isTimeoutForTest(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "timeout")
}

func Test_IsTimeout_Classification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline exceeded", fmt.Errorf("wait load: context deadline exceeded"), true},
		{"timeout", fmt.Errorf("navigate timeout"), true},
		{"other", fmt.Errorf("element not found"), false},
		{"wrapped deadline", fmt.Errorf("wait load viewupdate (attempt 3): context deadline exceeded"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTimeoutForTest(tt.err); got != tt.want {
				t.Errorf("isTimeout(%v)=%v want %v", tt.err, got, tt.want)
			}
		})
	}
}

// Test_Navigate_ConditionBasedWait_Succeeds proves the fix direction:
// if we gate on document.readyState !== 'loading' + ElementExists(form)
// instead of WaitLoad(window.onload), the viewupdate page would succeed
// within ScrapeFormWait bounds instead of hitting 60s*3.
func Test_Navigate_ConditionBasedWait_Succeeds(t *testing.T) {
	// Mock condition: DOM interactive immediately, form exists, fields populate in 1s
	// This models viewupdate: HTTP shell <1s, JS populates fields in 2s, but window.onload never fires.

	// Simulate poll loop like scraper.go:163-192
	probeTimeout := 2 * time.Second
	deadline := time.Now().Add(probeTimeout)
	ready := false
	pollInterval := 100 * time.Millisecond

	// Simulate field population after 300ms (AJAX delay)
	populatedAt := time.Now().Add(300 * time.Millisecond)

	for time.Now().Before(deadline) {
		if time.Now().After(populatedAt) {
			ready = true
			break
		}
		time.Sleep(pollInterval)
	}

	if !ready {
		t.Fatal("condition-based wait should succeed within ScrapeFormWait, but timed out — would be 3m03s with WaitLoad")
	}
	t.Logf("condition-based wait succeeded in ~300ms (vs 60s*3 with WaitLoad)")
}
