package extractor

import (
	"fmt"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// ParseKRS extracts structured KRS data from a PDF file.
func ParseKRS(path string, npm string) (*entity.KRSExtraction, error) {
	rows, err := ReadPDFWithPosition(path)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	result := &entity.KRSExtraction{}
	result.KRS.Mahasiswa.NPM = npm

	// Extract all lines for section detection
	lines := RowsToLines(rows)

	// Parse sections
	parseKRSHeader(lines, result)
	parseKRSMataKuliah(rows, lines, result)
	parseKRSPenerbitan(lines, result)
	parseKRSPersetujuan(lines, result)

	// Set metadata
	result.Metadata.ExtractedAt = time.Now()
	result.Metadata.SourceFile = path

	return result, nil
}

// parseKRSHeader extracts student info and period from header section.
func parseKRSHeader(lines []string, result *entity.KRSExtraction) {
	for i, line := range lines {
		// Look for student name pattern: "Nama : XXXX"
		if strings.Contains(line, "Nama") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KRS.Mahasiswa.Nama = strings.TrimSpace(parts[1])
			}
		}

		// Look for NPM pattern: "NPM : XXXX"
		if strings.Contains(line, "NPM") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				npm := strings.TrimSpace(parts[1])
				if npm != "" {
					result.KRS.Mahasiswa.NPM = npm
				}
			}
		}

		// Look for program studi: "Program Studi : XXXX"
		if strings.Contains(line, "Program Studi") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KRS.Mahasiswa.ProgramStudi = strings.TrimSpace(parts[1])
			}
		}

		// Look for tahun ajaran: "Tahun Ajaran : 2025/2026"
		if strings.Contains(line, "Tahun Ajaran") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KRS.Periode.TahunAjaran = strings.TrimSpace(parts[1])
			}
		}

		// Look for semester: "Semester : GENAP"
		if i > 0 && strings.Contains(line, "Semester") && strings.Contains(line, ":") {
			// Avoid matching "Semester" in table headers
			if !strings.Contains(lines[i-1], "Mata Kuliah") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					result.KRS.Periode.Semester = strings.TrimSpace(parts[1])
				}
			}
		}
	}
}

// parseKRSMataKuliah extracts course table from KRS.
func parseKRSMataKuliah(rows []PDFRow, lines []string, result *entity.KRSExtraction) {
	// Find table start marker
	tableStart := -1
	for i, line := range lines {
		if strings.Contains(line, "Kode") && strings.Contains(line, "Mata Kuliah") {
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
				if sks := parseIntSafe(words[j].Text); sks > 0 && sks <= 12 {
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
	days := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}
	for _, day := range days {
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
	// Indonesian month names
	indonesianMonths := map[string]string{
		"Januari": "January", "Februari": "February", "Maret": "March",
		"April": "April", "Mei": "May", "Juni": "June",
		"Juli": "July", "Agustus": "August", "September": "September",
		"Oktober": "October", "November": "November", "Desember": "December",
	}

	for _, line := range lines {
		// Look for "Dikeluarkan di" pattern
		if strings.Contains(line, "Dikeluarkan di") || strings.Contains(line, "dikeluarkan di") {
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
				dateFormats := []string{"2 January 2006", "02 January 2006", "2 January 2006", "January 2, 2006"}
				for _, format := range dateFormats {
					if t, err := time.Parse(format, dateStr); err == nil {
						result.KRS.Penerbitan.Tanggal = t.Format("2006-01-02")
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
		if strings.Contains(line, "Ketua Program Studi") {
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
