package extractor

import (
	"fmt"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// indonesianDays lists Indonesian day names for schedule parsing.
var indonesianDays = []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}

// ParseKRS extracts structured KRS data from a PDF file.
// It uses ReadPDF (plain text) for header fields and table data to preserve word spacing,
// and ReadPDFWithPosition as fallback for table data that needs column positions.
func ParseKRS(path string, npm string) (*entity.KRSExtraction, error) {
	result := &entity.KRSExtraction{}
	result.KRS.Mahasiswa.NPM = npm

	// Use plain text for header fields (handles gopdf horizontal format)
	plainText, err := ReadPDF(path)
	if err == nil {
		lines := strings.Split(plainText, "\n")
		parsePlainTextHeaderKRS(lines, result)
	}

	// Use position-based extraction for table data (works reliably with gopdf TextSpans)
	rows, posErr := ReadPDFWithPosition(path)
	if posErr != nil {
		// If plain text also failed, surface the position error
		if err != nil {
			return nil, fmt.Errorf("read pdf: %w", posErr)
		}
		// Header may still be usable; continue without table data
	} else {
		lines := RowsToLines(rows)
		parseKRSMataKuliah(rows, lines, result)
		result.KRS.Penerbitan = toEntityPenerbitan(parsePenerbitanFromLines(lines))
		parseKRSPersetujuan(lines, result)
	}

	// Set metadata
	result.Metadata.ExtractedAt = time.Now()
	result.Metadata.SourceFile = path

	return result, nil
}

// parsePlainTextHeaderKRS extracts header fields from plain text output.
// Uses shared parseHeaderFields helper for consistent label matching.
func parsePlainTextHeaderKRS(lines []string, result *entity.KRSExtraction) {
	labels := []string{"N P M", "Nama", "Program Studi", "Tahun Ajaran", "Semester"}
	fields := parseHeaderFields(lines, labels)
	result.KRS.Mahasiswa.NPM = fields["N P M"]
	result.KRS.Mahasiswa.Nama = fields["Nama"]
	result.KRS.Mahasiswa.ProgramStudi = fields["Program Studi"]
	result.KRS.Periode.Semester = fields["Semester"]
	result.KRS.Periode.TahunAjaran = splitTahunAjaran(fields["Tahun Ajaran"])
}

// krsColumnNames defines the expected column headers for KRS table extraction.
var krsColumnNames = []string{"No", "Kode", "Mata Kuliah", "SKS", "KELAS", "DOSEN", "JADWAL"}

// parseKRSMataKuliah extracts course table from KRS.
// Uses column-position-based extraction: identifies column boundaries from the header row,
// then scans ALL rows for course data (not dependent on row order).
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
// Uses column boundaries to only look at the JADWAL column's X range,
// preventing cross-contamination from other columns or courses.
func findScheduleNearRow(rows []PDFRow, courseIdx int, boundaries []ColumnBoundary) entity.KRSJadwal {
	// Find JADWAL column boundaries
	var jadwalStart, jadwalEnd float64
	for _, b := range boundaries {
		if b.Name == "JADWAL" {
			jadwalStart = b.Start
			jadwalEnd = b.End
			break
		}
	}
	if jadwalEnd == 0 {
		return entity.KRSJadwal{}
	}

	// Search in a window of ±2 rows around the course row
	start := courseIdx - 2
	if start < 0 {
		start = 0
	}
	end := courseIdx + 3
	if end > len(rows) {
		end = len(rows)
	}

	var dayLine string
	var timeLine string

	for i := start; i < end; i++ {
		if i == courseIdx {
			continue
		}

		// Only consider words within the JADWAL column's X range
		var jadwalWords []PDFWord
		for _, w := range rows[i].Words {
			if w.X >= jadwalStart && w.X < jadwalEnd {
				jadwalWords = append(jadwalWords, w)
			}
		}
		if len(jadwalWords) == 0 {
			continue
		}

		// Reconstruct the line from jadwal-column words only
		line := ""
		for _, w := range jadwalWords {
			if line != "" {
				line += " "
			}
			line += w.Text
		}
		line = strings.TrimSpace(line)

		if isIndonesianDay(line) {
			dayLine = line
		}
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
// Handles formats: "Sabtu 08:00-09:40", "Sabtu 08:00 - 09:40", "08:00-09:40",
// "Sabtu 08:00 s/d 09:40", "08:00 s/d 09:40"
func parseJadwal(line string) entity.KRSJadwal {
	jadwal := entity.KRSJadwal{}

	// Extract day
	for _, day := range indonesianDays {
		if strings.Contains(line, day) {
			jadwal.Hari = day
			break
		}
	}

	// Extract all HH:MM patterns from the line using sequential scanning
	var times []string
	for i := 0; i+4 <= len(line); i++ {
		if line[i+2] == ':' && isValidTime(line[i:i+5]) {
			times = append(times, line[i:i+5])
			i += 4 // skip past this time
		}
	}

	if len(times) >= 2 {
		jadwal.WaktuMulai = times[0] + ":00"
		jadwal.WaktuSelesai = times[1] + ":00"
	} else if len(times) == 1 {
		jadwal.WaktuMulai = times[0] + ":00"
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

// parseKRSPersetujuan extracts approval section from KRS.
// PDF layout (each label on its own row):
//
//	Mahasiswa
//	MOCHAMAD IZZAN FIRASYANSYAH
//	Ketua Prgram Studi Sistem Informasi
//	..............................................
//	NIDN......................................
func parseKRSPersetujuan(lines []string, result *entity.KRSExtraction) {
	for i, line := range lines {
		normalized := NormalizeLabel(line)
		trimmed := strings.TrimSpace(line)

		// Extract mahasiswa name
		// Case 1: Combined line "Mahasiswa Ketua Prgram Studi..." (old PDF format)
		if strings.Contains(line, "Mahasiswa") && strings.Contains(line, "Ketua") {
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				nextLine = extractNameBeforeDots(nextLine)
				if nextLine != "" {
					result.KRS.Persetujuan.Mahasiswa.Nama = nextLine
				}
			}
			continue
		}
		// Case 2: "Mahasiswa" on its own line (new PDF format)
		if normalized == "mahasiswa" {
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				nextLine = extractNameBeforeDots(nextLine)
				if nextLine != "" {
					result.KRS.Persetujuan.Mahasiswa.Nama = nextLine
				}
			}
			continue
		}

		// Extract ketua program studi jabatan
		if strings.Contains(normalized, "Ketua Program Studi") ||
			strings.Contains(normalized, "Ketua Prgram Studi") {
			result.KRS.Persetujuan.KetuaProgramStudi.Jabatan = line
			continue
		}

		// Extract ketua program studi name (skip dots and NIDN lines)
		if result.KRS.Persetujuan.KetuaProgramStudi.Jabatan != "" &&
			result.KRS.Persetujuan.KetuaProgramStudi.Nama == nil {
			// Skip empty lines, dots-only lines, and NIDN lines
			if trimmed == "" || isDotsOnly(trimmed) || strings.Contains(line, "NIDN") {
				continue
			}
			name := extractNameBeforeDots(trimmed)
			if name != "" {
				result.KRS.Persetujuan.KetuaProgramStudi.Nama = &name
			}
		}

		// Extract NIDN (only if it has an actual value after the dots)
		if strings.Contains(line, "NIDN") {
			nidn := strings.TrimSpace(strings.ReplaceAll(line, "NIDN", ""))
			nidn = strings.TrimLeft(nidn, ".")
			nidn = strings.TrimSpace(nidn)
			if nidn != "" {
				result.KRS.Persetujuan.KetuaProgramStudi.NIDN = &nidn
			}
		}
	}
}

// isDotsOnly checks if a string contains only dots and whitespace.
// Used to identify signature placeholder lines in PDF.
func isDotsOnly(s string) bool {
	for _, r := range s {
		if r != '.' && r != ' ' && r != '\t' {
			return false
		}
	}
	return len(s) > 0
}
