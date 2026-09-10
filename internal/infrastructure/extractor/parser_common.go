package extractor

import (
	"strings"
	"time"
	_ "time/tzdata"

	"lonceng_unman_be/internal/domain/entity"
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

// NormalizeLabel normalizes a line for pattern matching.
// It handles two cases:
// 1. Collapses spaces between single uppercase letters: "N P M" → "NPM"
// 2. Inserts spaces before uppercase letters after lowercase: "ProgramStudi" → "Program Studi"
func NormalizeLabel(s string) string {
	var result strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		// Skip spaces between single uppercase letters
		if r == ' ' && i > 0 && i+1 < len(runes) {
			if IsUpperLetter(runes[i-1]) && IsUpperLetter(runes[i+1]) {
				continue
			}
		}

		// Insert space before uppercase letter that follows a lowercase letter
		if i > 0 && IsUpperLetter(r) && isLowerLetter(runes[i-1]) && runes[i-1] != ' ' {
			result.WriteRune(' ')
		}

		result.WriteRune(r)
	}
	return result.String()
}

// splitTahunAjaran splits a year range into Awal/Akhir.
// Handles both slash-separated ("2025/2026") and underscore-separated ("2020_2021") formats.
// The underscore format comes from KHS filenames like "2020_2021_GENAP.json".
func splitTahunAjaran(ta string) entity.TahunAjaran {
	if ta == "" {
		return entity.TahunAjaran{}
	}

	// Try slash separator first (e.g., "2025/2026")
	if strings.Contains(ta, "/") {
		parts := strings.SplitN(ta, "/", 2)
		if len(parts) == 2 {
			return entity.TahunAjaran{
				Awal:  strings.TrimSpace(parts[0]),
				Akhir: strings.TrimSpace(parts[1]),
			}
		}
	}

	// Try underscore separator (e.g., "2020_2021" from filename "2020_2021_GENAP.json")
	// Only split if the string looks like two 4-digit years separated by underscore
	if strings.Contains(ta, "_") {
		parts := strings.SplitN(ta, "_", 2)
		if len(parts) == 2 {
			awal := strings.TrimSpace(parts[0])
			akhir := strings.TrimSpace(parts[1])
			// Validate both parts look like years (4 digits each)
			if len(awal) == 4 && len(akhir) == 4 && isAllDigits(awal) && isAllDigits(akhir) {
				return entity.TahunAjaran{
					Awal:  awal,
					Akhir: akhir,
				}
			}
		}
	}

	return entity.TahunAjaran{Awal: ta}
}

// isAllDigits checks if a string contains only digit characters.
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// parseHeaderFields extracts label-value pairs from plain text lines.
// Lines are expected in "Label : Value" format.
// Returns a map of label -> value for the requested labels.
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

// PenerbitanInfo is a generic publication info struct shared by KRS and KHS parsers.
type PenerbitanInfo struct {
	Tempat  string
	Tanggal string
}

// parsePenerbitanFromLines extracts publication info from text lines.
// Handles both formats:
//   - "Subang, 06 Agustus 2026"
//   - "Dikeluarkan di Subang, 06 Agustus 2026"
//
// Output timezone is always WIB (+07:00) since Indonesian academic documents
// are in Western Indonesia timezone.
//
// Returns empty PenerbitanInfo if not found.
func parsePenerbitanFromLines(lines []string) PenerbitanInfo {
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

		// Try to parse the date, then add WIB timezone
		for _, format := range dateFormats {
			if t, err := time.Parse(format, dateStr); err == nil {
				// Add WIB offset (+07:00) since the date has no timezone info.
				// LoadLocation can return (nil, err) on Windows when zoneinfo
				// is missing — t.In(nil) would panic ("missing Location").
				// Fix: embed tzdata (import _ "time/tzdata" above) + fallback
				// to FixedZone so extract never 500s on any Windows host.
				wib, err := time.LoadLocation("Asia/Jakarta")
				if err != nil || wib == nil {
					wib = time.FixedZone("WIB", 7*3600)
				}
				t = t.In(wib)
				return PenerbitanInfo{
					Tempat:  tempat,
					Tanggal: t.Format(dateOutputFormat),
				}
			}
		}
	}

	return PenerbitanInfo{}
}

// toEntityPenerbitan converts parser PenerbitanInfo to domain entity.Penerbitan.
func toEntityPenerbitan(p PenerbitanInfo) entity.Penerbitan {
	return entity.Penerbitan{
		Tempat:  p.Tempat,
		Tanggal: p.Tanggal,
	}
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
