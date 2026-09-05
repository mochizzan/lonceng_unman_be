package tests

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 5 — Regression seam assessment
//
// The logging bugs are structural (fmt.Printf interleaving + unstructured page lifecycle
// + uncorrelated fiber access log). There is NO runtime crash — the bug is noisy logs
// that make KHS flow undebuggable. Correct seam for a regression test is file-content
// invariant (source must use slog), not HTTP behavior.
//
// This file codifies seams that CAN be tested at unit layer, and documents seams
// that CANNOT be tested without architectural change (flagged for /improve-codebase-architecture).

var fmtPrintfRe = regexp.MustCompile(`fmt\.Printf\(`)

// Seam 1 (CORRECT): session lifecycle must be structured — testable via file read.
// This is a valid regression lock: if someone reintroduces fmt.Printf for pages,
// CI goes red immediately without needing a live browser.
func TestRegression_SessionLifecycleIsStructured(t *testing.T) {
	data, err := loadFileForTest("internal/infrastructure/session/session.go")
	if err != nil {
		t.Fatalf("read session.go: %v", err)
	}
	s := string(data)
	if fmtPrintfRe.MatchString(s) {
		t.Error("REGRESSION: session.go reintroduced fmt.Printf — page lifecycle must stay on slog")
	}
	for _, want := range []string{`slog.Debug`, `slog.Info`, `"npm"`, `"pageCount"`} {
		if !strings.Contains(s, want) {
			t.Errorf("REGRESSION: session.go missing %q — structured keys degraded", want)
		}
	}
	// Lifecycle messages should be Debug (noisy) not Info, except errors
	if strings.Contains(s, `slog.Info("creating new page"`) || strings.Contains(s, `slog.Info("page created"`) {
		t.Logf("INFO: page lifecycle is Info — consider Debug to reduce prod noise (H4)")
	}
}

// Seam 2 (CORRECT): manager fast-path/double-check logs must carry npm.
// Testable via file read; prevents silent removal of correlation key.
func TestRegression_ManagerLogsCarryNPM(t *testing.T) {
	data, err := loadFileForTest("internal/infrastructure/session/manager.go")
	if err != nil {
		t.Fatalf("read manager.go: %v", err)
	}
	for _, needle := range []string{`slog.Info("session reused"`, `slog.Warn("session corrupted`} {
		if !strings.Contains(string(data), needle) {
			t.Errorf("REGRESSION: manager.go missing log %q", needle)
		}
	}
	if !strings.Contains(string(data), `"npm"`) {
		t.Error("REGRESSION: manager.go logs lost npm key")
	}
}

// Seam 3 (NO CORRECT SEAM — documented): fiber access log ↔ slog trace correlation.
// The fiber logger is a middleware that writes directly to stdout. There is no
// injectable writer or slog handler in the current architecture, so tests cannot
// assert that access lines contain X-Request-Id/trace_id without refactoring
// middleware to accept a Writer or to emit via slog. This is flagged as
// architecture debt — /improve-codebase-architecture handoff: make middleware
// logger accept io.Writer or replace with slog-based request logger.
func TestRegression_FiberSlogCorrelation_RequiresArchitectureChange(t *testing.T) {
	t.Logf("ARCH Debt: no correct seam to assert fiber access log contains trace_id")
	t.Logf("Handoff to /improve-codebase-architecture: middleware.Register should accept logger config or slog handler")
	t.Logf("Until then, tmp/log-audit.mjs R4 covers this at log-file level, not unit-test level")
}

// Seam 4 (NO CORRECT SEAM — documented): KHS flow triple (detail → downloading → complete)
// is an end-to-end ordering property across lms_service + browser.DownloadAndSave.
// No single unit integrates both; mocking one side gives false confidence.
// Correct seam would be an interface-level contract test that asserts both services
// emit logs in order with the same (npm, tahun_ajaran, semester) keys — requires
// a test harness that captures slog output from both layers in one call path.
func TestRegression_KHSFlowTriple_RequiresIntegrationSeam(t *testing.T) {
	t.Logf("ARCH Debt: KHS flow triple has no unit seam — needs integration harness capturing slog from lms_service + browser")
	t.Logf("Current coverage: tmp/log-audit.mjs R2 audits log file ordering; tmp/log-probe.mjs audits site counts")
}

func TestRegression_NoFmtMixAnywhere(t *testing.T) {
	files := []string{
		"internal/infrastructure/session/manager.go",
		"internal/infrastructure/session/session.go",
		"internal/application/service/lms_service.go",
		"internal/infrastructure/browser/browser.go",
	}
	for _, f := range files {
		data, err := loadFileForTest(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if fmtPrintfRe.MatchString(string(data)) && strings.Contains(string(data), "slog.") {
			t.Errorf("REGRESSION: %s mixes fmt.Printf and slog", f)
		}
		// browser.go legitimately has no fmt.Printf except errors; session.go is the offender
		if f == "internal/infrastructure/session/session.go" && fmtPrintfRe.MatchString(string(data)) {
			t.Errorf("REGRESSION: %s still has fmt.Printf", f)
		}
	}
}
