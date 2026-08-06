package extractor

import (
	"fmt"
	"os"
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
// It uses ReadPDF (plain text) for header fields and table data to preserve word spacing,
// and ReadPDFWithPosition as fallback for table data that needs column positions.
func ParseKRS(path string, npm string) (*entity.KRSExtraction, error) {
	// Validate file exists before parsing
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pdf file not found: %s", path)
		}
		return nil, fmt.Errorf("stat pdf: %w", err)
	}

	result := &entity.KRSExtraction{}
	result.KRS.Mahasiswa.NPM = npm

	// Use plain text for header fields and table data (preserves spaces)
	plainText, err := ReadPDF(path)
	if err == nil {
		parsePlainTextHeaderKRS(plainText, result)
		parsePlainTextTableKRS(plainText, result)
	}

	// Use position-based extraction as fallback if plain text table parsing failed
	if len(result.KRS.MataKuliah) == 0 {
		rows, posErr := ReadPDFWithPosition(path)
		if posErr == nil {
			lines := RowsToLines(rows)
			parseKRSMataKuliah(rows, lines, result)
		}
	}

	// Parse penerbitan and persetujuan from position-based extraction
	rows, err := ReadPDFWithPosition(path)
	if err == nil {
		lines := RowsToLines(rows)
		parseKRSPenerbitan(lines, result)
		parseKRSPersetujuan(lines, result)
	}

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

// parsePlainTextTableKRS extracts the course table from plain text output.
// The plain text format has each column value on a separate line:
//
// No.
// Kode
// Mata Kuliah
// SKS
// KELAS
// DOSEN
// JADWAL
// 1
// SI40306
// Tugas Akhir/Skripsi
// 6
// SI-8A
// TIM DOSEN FAKULTAS TEKNIK
// Sabtu
// 08:00 s/d 09:40
//
// This produces correctly spaced text unlike position-based extraction.
func parsePlainTextTableKRS(text string, result *entity.KRSExtraction) {
	lines := strings.Split(text, "\n")

	// Find the header row: look for consecutive lines matching column names
	headerIdx := -1
	for i := 0; i < len(lines)-6; i++ {
		if strings.TrimSpace(lines[i]) == "No." &&
			strings.TrimSpace(lines[i+1]) == "Kode" &&
			strings.TrimSpace(lines[i+2]) == "Mata Kuliah" {
			headerIdx = i
			break
		}
	}

	if headerIdx == -1 {
		return
	}

	// Find the table end: look for "Total"
	tableEnd := len(lines)
	for i := headerIdx + 7; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "Total" {
			tableEnd = i
			break
		}
	}

	// Each course takes 7 lines (No, Kode, Mata Kuliah, SKS, KELAS, DOSEN)
	// plus optionally 2 lines for schedule (day, time)
	// Parse course by course
	var courses []entity.KRSMataKuliah
	courseNo := 1

	dataStart := headerIdx + 7 // skip 7 header lines
	for i := dataStart; i < tableEnd; {
		// Read 7 lines for one course
		if i+6 >= tableEnd {
			break
		}

		// Skip empty lines
		for i < tableEnd && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i+6 >= tableEnd {
			break
		}

		kode := strings.TrimSpace(lines[i+1])
		nama := strings.TrimSpace(lines[i+2])
		sksStr := strings.TrimSpace(lines[i+3])
		kelas := strings.TrimSpace(lines[i+4])
		dosen := strings.TrimSpace(lines[i+5])

		// Validate: Kode should be a course code (contains digits)
		if kode == "" || len(kode) < 3 {
			i++
			continue
		}
		hasDigit := false
		for _, r := range kode {
			if r >= '0' && r <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			i++
			continue
		}

		// Parse SKS
		sks := parseIntSafe(sksStr)

		course := entity.KRSMataKuliah{
			No:    courseNo,
			Kode:  kode,
			Nama:  nama,
			SKS:   sks,
			Kelas: kelas,
			Dosen: dosen,
		}

		i += 6 // advance past the 7 course lines (0-6)

		// Check for schedule (day + time) in next lines
		if i < tableEnd {
			dayLine := strings.TrimSpace(lines[i])
			if isIndonesianDay(dayLine) {
				course.Jadwal.Hari = extractDayName(dayLine)
				i++
				if i < tableEnd {
					timeLine := strings.TrimSpace(lines[i])
					if strings.Contains(timeLine, ":") {
						course.Jadwal = parseJadwal(dayLine + " " + timeLine)
						i++
					}
				}
			}
		}

		courses = append(courses, course)
		courseNo++
	}

	result.KRS.MataKuliah = courses

	// Calculate total SKS
	total := 0
	for _, c := range courses {
		total += c.SKS
	}
	result.KRS.TotalSKS = total
}

// extractDayName extracts the Indonesian day name from a line.
func extractDayName(line string) string {
	for _, day := range indonesianDays {
		if strings.Contains(line, day) {
			return day
		}
	}
	return ""
}

// krsColumnNames defines the expected column headers for KRS table extraction.
var krsColumnNames = []string{"No", "Kode", "Mata Kuliah", "SKS", "KELAS", "DOSEN", "JADWAL"}

// parseKRSMataKuliah extracts course table from KRS.
// Uses column-position-based extraction: identifies column boundaries from the header row,
// then scans ALL rows for course data (not dependent on row order).
// This correctly handles PDFs where the library returns individual characters
// and where rows may be in non-standard order (e.g., bottom-to-top).
func parseKRSMataKuliah(rows []PDFRow, lines []string, result *entity.KRSExtraction) {
	// Find the header row to identify column boundaries
	var headerRow *PDFRow
	for i, row := range rows {
		line := RowToLine(row)
		normalized := NormalizeLabel(line)
		if strings.Contains(normalized, "Kode") && strings.Contains(normalized, "Mata Kuliah") {
			headerRow = &rows[i]
			break
		}
	}

	if headerRow == nil {
		return
	}

	// Identify column X boundaries from the header row
	boundaries := FindColumnPositions(*headerRow, krsColumnNames)
	if boundaries == nil {
		return
	}

	// Scan ALL rows for course data (not dependent on row order).
	// A row is a course row if it has a valid Kode (alphanumeric course code).
	var courses []entity.KRSMataKuliah
	courseNo := 1
	seenKodes := make(map[string]bool) // deduplicate courses

	for i, row := range rows {
		if len(row.Words) == 0 {
			continue
		}

		line := RowToLine(row)

		// Skip header-like rows
		if strings.Contains(line, "No") && strings.Contains(line, "Kode") {
			continue
		}

		// Skip non-data rows
		if len(row.Words) < 3 {
			continue
		}

		// Skip Total/Jumlah rows
		if strings.Contains(line, "Total") || strings.Contains(line, "Jumlah") {
			continue
		}

		// Extract text for each column using X-position boundaries
		cols := ExtractColumnsFromRow(row, boundaries)

		// Skip rows where Kode is empty or too short (not a course row)
		kode := strings.TrimSpace(cols["Kode"])
		if kode == "" || len(kode) < 3 {
			continue
		}

		// Skip non-alphanumeric course codes (must contain at least one digit)
		hasDigit := false
		for _, r := range kode {
			if r >= '0' && r <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			continue
		}

		// Deduplicate (same code + class = same course)
		kelas := strings.TrimSpace(cols["KELAS"])
		key := kode + ":" + kelas
		if seenKodes[key] {
			continue
		}
		seenKodes[key] = true

		// Parse SKS (must be a valid integer)
		sks := parseIntSafe(cols["SKS"])

		course := entity.KRSMataKuliah{
			No:    courseNo,
			Kode:  kode,
			Nama:  strings.TrimSpace(cols["Mata Kuliah"]),
			SKS:   sks,
			Kelas: kelas,
			Dosen: strings.TrimSpace(cols["DOSEN"]),
		}

		// Extract schedule from the Jadwal column
		jadwalText := strings.TrimSpace(cols["JADWAL"])
		if jadwalText != "" {
			course.Jadwal = parseJadwal(jadwalText)
		} else {
			// Search nearby rows for schedule (day + time might be on separate rows)
			course.Jadwal = findScheduleNearRow(rows, i, boundaries)
		}

		courses = append(courses, course)
		courseNo++
	}

	result.KRS.MataKuliah = courses

	// Calculate total SKS
	total := 0
	for _, c := range courses {
		total += c.SKS
	}
	result.KRS.TotalSKS = total
}

// findScheduleNearRow searches rows near the given index for schedule information.
// Schedule data might be split across multiple rows (day on one row, time on another).
func findScheduleNearRow(rows []PDFRow, courseIdx int, boundaries []ColumnBoundary) entity.KRSJadwal {
	// Search in a window of ±3 rows around the course row
	start := courseIdx - 3
	if start < 0 {
		start = 0
	}
	end := courseIdx + 4
	if end > len(rows) {
		end = len(rows)
	}

	var dayLine string
	var timeLine string

	for i := start; i < end; i++ {
		if i == courseIdx {
			continue
		}
		line := RowToLine(rows[i])
		if isIndonesianDay(line) {
			dayLine = line
		}
		// Check for time pattern HH:MM
		if strings.Contains(line, ":") && len(line) >= 5 {
			for _, part := range strings.Fields(line) {
				if isValidTime(part) {
					timeLine = line
					break
				}
			}
		}
	}

	if dayLine != "" || timeLine != "" {
		combined := dayLine
		if timeLine != "" {
			if combined != "" {
				combined += " "
			}
			combined += timeLine
		}
		return parseJadwal(combined)
	}

	return entity.KRSJadwal{}
}

// isIndonesianDay checks if a line contains an Indonesian day name.
func isIndonesianDay(line string) bool {
	for _, day := range indonesianDays {
		if strings.Contains(line, day) {
			return true
		}
	}
	return false
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
