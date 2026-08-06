package extractor

import (
	"strings"
	"time"
)

// ============================================================
// Shared constants used by both KRS and KHS parsers.
// ============================================================

var indonesianMonths = map[string]string{
	"Januari":   "January",
	"Februari":  "February",
	"Maret":     "March",
	"April":     "April",
	"Mei":       "May",
	"Juni":      "June",
	"Juli":      "July",
	"Agustus":   "August",
	"September": "September",
	"Oktober":   "October",
	"November":  "November",
	"Desember":  "December",
}

var dateFormats = []string{
	"02 January 2006",
	"2 January 2006",
	"01 January 2006",
	"1 January 2006",
	"January 2, 2006",
	"January 2 2006",
	"2006-01-02",
	"02/01/2006",
	"2/1/2006",
	"01/02/2006",
	"1/2/2006",
}

const dateOutputFormat = "2006-01-02T15:04:05-07:00"

// MaxSKS is the maximum SKS value allowed for validation.
const MaxSKS = 12

// ============================================================
// Shared parsing helpers used by both KRS and KHS parsers.
// ============================================================

// parseHeaderFields extracts label-value pairs from plain text lines.
// Lines are expected in "Label : Value" format.
// Returns a map of label -> value for the requested labels.
// The labels slice uses normalized forms (lowercase, single spaces)
// for matching against the parsed lines.
func parseHeaderFields(lines []string, labels []string) map[string]string {
	result := make(map[string]string)

	// Build normalized -> original label mapping
	normalizedLabels := make(map[string]string)
	for _, label := range labels {
		normalizedLabels[NormalizeLabel(label)] = label
	}

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if val == "" {
			continue
		}

		normalized := NormalizeLabel(key)
		if origLabel, ok := normalizedLabels[normalized]; ok {
			result[origLabel] = val
		}
	}

	return result
}

// Penerbitan is a generic publication info struct shared by KRS and KHS.
type Penerbitan struct {
	Tempat  string
	Tanggal string
}

// parsePenerbitanFromLines extracts publication info from text lines.
// Handles both formats:
//   - "Subang, 06 Agustus 2026"
//   - "Dikeluarkan di Subang, 06 Agustus 2026"
//
// Returns empty Penerbitan if not found.
func parsePenerbitanFromLines(lines []string) Penerbitan {
	for _, line := range lines {
		if !strings.Contains(line, ",") {
			continue
		}

		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		tempat := strings.TrimSpace(parts[0])
		dateStr := strings.TrimSpace(parts[1])

		// Check if the date part contains an Indonesian month
		monthFound := false
		for indo, eng := range indonesianMonths {
			if strings.Contains(dateStr, indo) {
				dateStr = strings.Replace(dateStr, indo, eng, 1)
				monthFound = true
				break
			}
		}
		if !monthFound {
			continue
		}

		// Try to parse the date
		for _, format := range dateFormats {
			if t, err := time.Parse(format, dateStr); err == nil {
				return Penerbitan{
					Tempat:  tempat,
					Tanggal: t.Format(dateOutputFormat),
				}
			}
		}
	}

	return Penerbitan{}
}

// extractNextNonEmptyAfterLabel finds a label in lines and returns the next
// non-empty, non-NIDN line after it. Used for extracting names from
// approval sections (persetujuan).
//
// searchFn determines whether a line matches the desired label.
// Returns empty string if not found.
func extractNextNonEmptyAfterLabel(lines []string, startIdx int, searchFn func(string) bool) string {
	for i := startIdx + 1; i < len(lines) && i < startIdx+5; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.Contains(lines[i], "NIDN") {
			continue
		}
		if searchFn(trimmed) {
			return extractNameBeforeDots(trimmed)
		}
	}
	return ""
}

// extractNameBeforeDots removes trailing dots and whitespace from a name.
// PDF names often have signature dots: "JOHN DOE .............."
func extractNameBeforeDots(name string) string {
	name = strings.TrimSpace(name)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = strings.TrimSpace(name[:len(name)-1])
	}
	return name
}
