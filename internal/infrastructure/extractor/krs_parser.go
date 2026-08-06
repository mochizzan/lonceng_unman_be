package extractor

import (
	"fmt"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// indonesianMonths maps Indonesian month names to English for date parsing.
var indonesianMonths = map[string]string{
	"Januari": "January", "Februari": "February", "Maret": "March",
	"April": "April", "Mei": "May", "Juni": "June",
	"Juli": "July", "Agustus": "August", "September": "September",
	"Oktober": "October", "November": "November", "Desember": "December",
}

// dateFormats lists Go time.Parse formats to try for Indonesian dates.
var dateFormats = []string{"2 January 2006", "02 January 2006", "January 2, 2006"}

// dateOutputFormat is the ISO date format used for output.
const dateOutputFormat = "2006-01-02"

// indonesianDays lists Indonesian day names for schedule parsing.
var indonesianDays = []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}

// MaxSKS is the maximum plausible SKS credit value.
const MaxSKS = 12

// ParseKRS extracts structured KRS data from a PDF file.
// It uses ReadPDF (plain text) for header fields to preserve word spacing,
// and ReadPDFWithPosition for table data that needs column positions.
func ParseKRS(path string, npm string) (*entity.KRSExtraction, error) {
	result := &entity.KRSExtraction{}
	result.KRS.Mahasiswa.NPM = npm

	// Use plain text for header fields (preserves spaces)
	plainText, err := ReadPDF(path)
	if err == nil {
		parsePlainTextHeaderKRS(plainText, result)
	}

	// Use position-based extraction for table data
	rows, err := ReadPDFWithPosition(path)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	lines := RowsToLines(rows)

	// Parse table and other sections
	parseKRSMataKuliah(rows, lines, result)
	parseKRSPenerbitan(lines, result)
	parseKRSPersetujuan(lines, result)

	// Set metadata
	result.Metadata.ExtractedAt = time.Now()
	result.Metadata.SourceFile = path

	return result, nil
}

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

// isLowerLetter checks if a rune is a lowercase Latin letter.
func isLowerLetter(r rune) bool {
	return r >= 'a' && r <= 'z'
}

// parsePlainTextHeaderKRS extracts header fields from plain text output.
// The plain text format has label, colon, and value on separate lines:
//
//	Nama
//	:
//	MOCHAMAD IZZAN FIRASYANSYAH
func parsePlainTextHeaderKRS(text string, result *entity.KRSExtraction) {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for Nama field
		if trimmed == "Nama" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KRS.Mahasiswa.Nama = val
			}
		}

		// Look for NPM field (handles both "NPM" and "N P M")
		if trimmed == "NPM" || trimmed == "N P M" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KRS.Mahasiswa.NPM = val
			}
		}

		// Look for Program Studi field
		if trimmed == "Program Studi" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KRS.Mahasiswa.ProgramStudi = val
			}
		}

		// Look for Tahun Ajaran field
		if trimmed == "Tahun Ajaran" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KRS.Periode.TahunAjaran = val
			}
		}

		// Look for Semester field
		if trimmed == "Semester" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KRS.Periode.Semester = val
			}
		}
	}
}

// extractNextColonValue looks for ":" in the next few lines and returns the value after it.
// Handles formats:
//   - "Label\n:\nValue" (colon on its own line)
//   - "Label: Value" (colon on same line as label)
//   - "Label\n: Value" (colon on next line with value)
func extractNextColonValue(lines []string, startIdx int) string {
	// First check if the current line has colon with value
	currentLine := strings.TrimSpace(lines[startIdx])
	if strings.Contains(currentLine, ":") {
		parts := strings.SplitN(currentLine, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Look in next few lines for colon
	for j := startIdx + 1; j < len(lines) && j < startIdx+4; j++ {
		trimmed := strings.TrimSpace(lines[j])

		// Case 1: Line is just ":" → value is on the next line
		if trimmed == ":" {
			if j+1 < len(lines) {
				return strings.TrimSpace(lines[j+1])
			}
			return ""
		}

		// Case 2: Line starts with ":" → value is after the colon
		if strings.HasPrefix(trimmed, ":") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, ":"))
		}
	}

	return ""
}

// IsUpperLetter checks if a rune is an uppercase Latin letter.
func IsUpperLetter(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// FindNextValueLine looks for a value starting with ":" in the next few lines.
// Returns the trimmed value after ":", or empty string if not found.
func FindNextValueLine(lines []string, startIdx int) string {
	for j := startIdx + 1; j < len(lines) && j < startIdx+3; j++ {
		nextLine := strings.TrimSpace(lines[j])
		if strings.HasPrefix(nextLine, ":") {
			return strings.TrimSpace(strings.TrimPrefix(nextLine, ":"))
		}
	}
	return ""
}

// parseKRSMataKuliah extracts course table from KRS.
func parseKRSMataKuliah(rows []PDFRow, lines []string, result *entity.KRSExtraction) {
	// Find table start marker
	tableStart := -1
	for i, line := range lines {
		normalized := NormalizeLabel(line)
		if strings.Contains(normalized, "Kode") && strings.Contains(normalized, "Mata Kuliah") {
			tableStart = i + 1
			break
		}
	}

	if tableStart == -1 {
		return
	}

	// Find table end (look for "Total" or empty rows)
	tableEnd := len(lines)
	for i := tableStart; i < len(lines); i++ {
		if strings.Contains(lines[i], "Total") || strings.Contains(lines[i], "Jumlah") {
			tableEnd = i
			break
		}
	}

	// Parse table rows
	var courses []entity.KRSMataKuliah
	courseNo := 1

	for i := tableStart; i < tableEnd && i < len(rows); i++ {
		row := rows[i]
		if len(row.Words) < 5 {
			continue // skip non-table rows
		}

		// Try to identify columns by position
		// Typical KRS table: No | Kode | Nama | SKS | Kelas | Dosen | Jadwal
		line := RowToLine(row)

		// Skip header-like rows
		if strings.Contains(line, "No") && strings.Contains(line, "Kode") {
			continue
		}

		// Parse course data
		course := entity.KRSMataKuliah{
			No: courseNo,
		}

		// Extract fields based on position patterns
		words := row.Words
		if len(words) >= 6 {
			course.Kode = words[1].Text
			course.Nama = words[2].Text

			// Try to parse SKS
			for j := 3; j < len(words); j++ {
				if sks := parseIntSafe(words[j].Text); sks > 0 && sks <= MaxSKS {
					course.SKS = sks
					break
				}
			}

			// Get class and dosen from remaining words
			if len(words) > 4 {
				course.Kelas = words[4].Text
			}
			if len(words) > 5 {
				dosenParts := make([]string, len(words)-5)
				for k, w := range words[5:] {
					dosenParts[k] = w.Text
				}
				course.Dosen = strings.Join(dosenParts, " ")
			}
		}

		// Extract schedule from next row if available
		if i+1 < len(rows) {
			nextLine := RowToLine(rows[i+1])
			if strings.Contains(nextLine, "Sabtu") || strings.Contains(nextLine, "Senin") ||
				strings.Contains(nextLine, "Selasa") || strings.Contains(nextLine, "Rabu") ||
				strings.Contains(nextLine, "Kamis") || strings.Contains(nextLine, "Jumat") {
				course.Jadwal = parseJadwal(nextLine)
				i++ // skip schedule row
			}
		}

		if course.Kode != "" {
			courses = append(courses, course)
			courseNo++
		}
	}

	result.KRS.MataKuliah = courses

	// Calculate total SKS
	total := 0
	for _, c := range courses {
		total += c.SKS
	}
	result.KRS.TotalSKS = total
}

// parseJadwal extracts schedule info from a line.
// Handles formats: "Sabtu 08:00-09:40", "Sabtu 08:00 - 09:40", "08:00-09:40"
func parseJadwal(line string) entity.KRSJadwal {
	jadwal := entity.KRSJadwal{}

	// Extract day
	for _, day := range indonesianDays {
		if strings.Contains(line, day) {
			jadwal.Hari = day
			break
		}
	}

	// Extract time pattern: "HH:MM" (with or without spaces around dash)
	// First, normalize the line by removing spaces around dashes
	normalized := strings.ReplaceAll(line, " - ", "-")
	normalized = strings.ReplaceAll(normalized, "- ", "-")
	normalized = strings.ReplaceAll(normalized, " -", "-")

	// Split by dash to get start and end times
	timeParts := strings.Split(normalized, "-")

	for _, part := range timeParts {
		part = strings.TrimSpace(part)
		// Match time pattern HH:MM
		if len(part) >= 5 && part[2] == ':' {
			timeStr := part[:5]
			if isValidTime(timeStr) {
				if jadwal.WaktuMulai == "" {
					jadwal.WaktuMulai = timeStr
				} else {
					jadwal.WaktuSelesai = timeStr
				}
			}
		}
	}

	return jadwal
}

// isValidTime validates time format HH:MM.
func isValidTime(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	hour := parseIntSafe(s[:2])
	min := parseIntSafe(s[3:])
	return hour >= 0 && hour <= 23 && min >= 0 && min <= 59
}

// parseKRSPenerbitan extracts publication info from KRS.
// Handles Indonesian date formats: "6 Agustus 2026", "06 Agustus 2026"
func parseKRSPenerbitan(lines []string, result *entity.KRSExtraction) {
	for _, line := range lines {
		normalized := NormalizeLabel(line)
		// Look for "Dikeluarkan di" pattern
		if strings.Contains(normalized, "Dikeluarkan di") || strings.Contains(normalized, "dikeluarkan di") {
			parts := strings.SplitN(line, ",", 2)
			if len(parts) == 2 {
				result.KRS.Penerbitan.Tempat = strings.TrimSpace(parts[0])
				dateStr := strings.TrimSpace(parts[1])

				// Try Indonesian date format first
				for indo, eng := range indonesianMonths {
					if strings.Contains(dateStr, indo) {
						dateStr = strings.Replace(dateStr, indo, eng, 1)
						break
					}
				}

				// Try multiple date formats
				for _, format := range dateFormats {
					if t, err := time.Parse(format, dateStr); err == nil {
						result.KRS.Penerbitan.Tanggal = t.Format(dateOutputFormat)
						break
					}
				}
			}
		}
	}
}

// parseKRSPersetujuan extracts approval section from KRS.
func parseKRSPersetujuan(lines []string, result *entity.KRSExtraction) {
	for i, line := range lines {
		normalized := NormalizeLabel(line)
		if strings.Contains(normalized, "Ketua Program Studi") {
			result.KRS.Persetujuan.KetuaProgramStudi.Jabatan = line
			// Look for name in next lines
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.TrimSpace(lines[j]) != "" &&
					!strings.Contains(lines[j], "NIDN") &&
					!strings.Contains(lines[j], "(") {
					name := strings.TrimSpace(lines[j])
					result.KRS.Persetujuan.KetuaProgramStudi.Nama = &name
					break
				}
			}
		}
	}
}
