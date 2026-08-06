package extractor

import (
	"testing"

	"lonceng_unman_be/internal/infrastructure/extractor"
)

func TestParseKRSHeader_SeparateLabelAndValue(t *testing.T) {
	// Simulate the PDF line structure where label and value are on separate lines
	// This is the format the user reported: "Nama" on one line, ": VALUE" on next line
	lines := []string{
		"KARTU RENCANA STUDI",
		"Nama",
		": MOCHAMAD IZZAN FIRASYANSYAH",
		"N P M",
		": 2211700006",
		"Program Studi",
		": Sistem Informasi",
		"Tahun Ajaran",
		": 2025/2026",
		"Semester",
		": GENAP",
	}

	_ = lines // used in subtests below

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

	t.Run("FindNextValueLine", func(t *testing.T) {
		// Test finding value on next line
		val := extractor.FindNextValueLine(lines, 1) // "Nama" at index 1
		if val != "MOCHAMAD IZZAN FIRASYANSYAH" {
			t.Errorf("FindNextValueLine(lines, 1) = %q, want %q", val, "MOCHAMAD IZZAN FIRASYANSYAH")
		}

		val = extractor.FindNextValueLine(lines, 3) // "N P M" at index 3
		if val != "2211700006" {
			t.Errorf("FindNextValueLine(lines, 3) = %q, want %q", val, "2211700006")
		}

		val = extractor.FindNextValueLine(lines, 5) // "Program Studi" at index 5
		if val != "Sistem Informasi" {
			t.Errorf("FindNextValueLine(lines, 5) = %q, want %q", val, "Sistem Informasi")
		}
	})
}
