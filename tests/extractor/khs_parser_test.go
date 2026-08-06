package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"lonceng_unman_be/internal/infrastructure/extractor"
)

func TestParseKHS_FileNotFound(t *testing.T) {
	_, err := extractor.ParseKHS("testdata/nonexistent.pdf", "12345", "2024/2025", "Ganjil")
	if err == nil {
		t.Fatal("expected error for nonexistent PDF, got nil")
	}
	if !os.IsNotExist(err) {
		t.Logf("got error (expected for nonexistent file): %v", err)
	}
}

func TestParseKHS_Success(t *testing.T) {
	pdf := filepath.Join("testdata", "khs_sample.pdf")
	if _, err := os.Stat(pdf); os.IsNotExist(err) {
		t.Skip("testdata/khs_sample.pdf not found; skipping integration test")
	}

	result, err := extractor.ParseKHS(pdf, "12345", "2024/2025", "Ganjil")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
