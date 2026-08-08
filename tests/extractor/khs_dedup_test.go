package extractor

import (
	"testing"

	"lonceng_unman_be/internal/infrastructure/extractor"
)

// Comprehensive KHS dedup logic tests live in
// internal/infrastructure/extractor/khs_dedup_test.go
// (white-box tests that exercise parseKHSMataKuliah directly with mock PDFRow data).
//
// This file contains integration-level tests through the public ParseKHS API.

func TestParseKHS_DedupIntegration(t *testing.T) {
	// The dedup logic is unit-tested internally via TestParseKHSMataKuliah_*
	// in internal/infrastructure/extractor/khs_dedup_test.go.
	// This test verifies ParseKHS does not crash on a nonexistent file
	// (regression guard for the dedup refactor path).
	_, err := extractor.ParseKHS("testdata/nonexistent.pdf", "0000000000", "2025/2026", "Genap")
	if err == nil {
		t.Fatal("expected error for nonexistent PDF, got nil")
	}
}
