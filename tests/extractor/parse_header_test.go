package extractor

import (
	"testing"

	"lonceng_unman_be/internal/infrastructure/extractor"
)

func TestParseKRSHeader_SeparateLabelAndValue(t *testing.T) {
	t.Run("NormalizeLabel", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"N P M", "NPM"},
			{"N P M I", "NPMI"},
			{"Nama", "Nama"},
			{"Program Studi", "Program Studi"},
		}
		for _, tt := range tests {
			got := extractor.NormalizeLabel(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeLabel(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		}
	})
}
