package extractor

import (
	"fmt"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// indonesianMonthsKHS maps Indonesian month names to English for date parsing.
// Kept as separate var from krs_parser to avoid redeclaration in same package.
var indonesianMonthsKHS = map[string]string{
	"Januari": "January", "Februari": "February", "Maret": "March",
	"April": "April", "Mei": "May", "Juni": "June",
	"Juli": "July", "Agustus": "August", "September": "September",
	"Oktober": "October", "November": "November", "Desember": "December",
}

// dateFormatsKHS lists Go time.Parse formats to try for Indonesian dates.
var dateFormatsKHS = []string{"2 January 2006", "02 January 2006", "January 2, 2006"}

// dateOutputFormatKHS is the ISO date format used for output.
const dateOutputFormatKHS = "2006-01-02"

// MaxSKSKHS is the maximum plausible SKS credit value.
const MaxSKSKHS = 12

// ParseKHS extracts structured KHS data from a PDF file.
// It uses ReadPDF (plain text) for header fields to preserve word spacing,
// and ReadPDFWithPosition for table data that needs column positions.
func ParseKHS(path string, npm string, tahunAjaran string, semester string) (*entity.KHSExtraction, error) {
	result := &entity.KHSExtraction{}
	result.KHS.Mahasiswa.NPM = npm
	result.KHS.Periode.TahunAjaran = tahunAjaran
	result.KHS.Periode.Semester = semester

	// Use plain text for header fields (preserves spaces)
	plainText, err := ReadPDF(path)
	if err == nil {
		parsePlainTextHeaderKHS(plainText, result)
	}

	// Use position-based extraction for table data
	rows, err := ReadPDFWithPosition(path)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	lines := RowsToLines(rows)

	// Parse table and other sections
	parseKHSMataKuliah(rows, lines, result)
	parseKHSRekapitulasi(lines, result)
	parseKHSPenerbitan(lines, result)
	parseKHSPersetujuan(lines, result)

	// Set metadata
	result.Metadata.ExtractedAt = time.Now()
	result.Metadata.SourceFile = path

	return result, nil
}

// parsePlainTextHeaderKHS extracts header fields from plain text output.
// The plain text format has label, colon, and value on separate lines.
func parsePlainTextHeaderKHS(text string, result *entity.KHSExtraction) {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for Nama field
		if trimmed == "Nama" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KHS.Mahasiswa.Nama = val
			}
		}

		// Look for NPM field (handles both "NPM" and "N P M")
		if trimmed == "NPM" || trimmed == "N P M" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KHS.Mahasiswa.NPM = val
			}
		}

		// Look for Program Studi field
		if trimmed == "Program Studi" {
			if val := extractNextColonValue(lines, i); val != "" {
				result.KHS.Mahasiswa.ProgramStudi = val
			}
		}
	}
}

// khsColumnNames defines the expected column headers for KHS table extraction.
// KHS table: No | Kode | Mata Kuliah | Dosen | SKS | Nilai | Mutu
var khsColumnNames = []string{"No", "Kode", "Mata Kuliah", "Dosen", "SKS", "Nilai", "Mutu"}

// parseKHSMataKuliah extracts course table from KHS.
// Uses column-position-based extraction: identifies column boundaries from the header row,
// then scans ALL rows for course data (not dependent on row order).
// This correctly handles PDFs where the library returns individual characters
// and where rows may be in non-standard order (e.g., bottom-to-top).
func parseKHSMataKuliah(rows []PDFRow, lines []string, result *entity.KHSExtraction) {
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
	boundaries := FindColumnPositions(*headerRow, khsColumnNames)
	if boundaries == nil {
		return
	}

	// Scan ALL rows for course data (not dependent on row order).
	// A row is a course row if it has a valid Kode (alphanumeric course code).
	var courses []entity.KHSMataKuliah
	courseNo := 1
	seenKodes := make(map[string]bool) // deduplicate courses

	for _, row := range rows {
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

		// Skip Rekapitulasi/Total rows
		if strings.Contains(line, "Rekapitulasi") || strings.Contains(line, "Total") {
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

		// Deduplicate
		if seenKodes[kode] {
			continue
		}
		seenKodes[kode] = true

		// Parse SKS and Nilai
		sks := parseIntSafe(cols["SKS"])
		nilai := strings.TrimSpace(cols["Nilai"])
		if nilai != "" {
			nilai = strings.ToUpper(nilai)
		}
		mutu := parseIntSafe(cols["Mutu"])

		course := entity.KHSMataKuliah{
			No:    courseNo,
			Kode:  kode,
			Nama:  strings.TrimSpace(cols["Mata Kuliah"]),
			Dosen: strings.TrimSpace(cols["Dosen"]),
			SKS:   sks,
			Nilai: nilai,
			Mutu:  mutu,
		}

		courses = append(courses, course)
		courseNo++
	}

	result.KHS.MataKuliah = courses
}

// parseKHSRekapitulasi extracts summary stats from KHS.
func parseKHSRekapitulasi(lines []string, result *entity.KHSExtraction) {
	for _, line := range lines {
		normalized := NormalizeLabel(line)

		// Look for total SKS: "Total SKS : 23"
		if strings.Contains(normalized, "Total SKS") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.TotalSKS = parseIntSafe(parts[1])
			}
		}

		// Look for total mutu: "Total Mutu : 84"
		if strings.Contains(normalized, "Total Mutu") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.TotalMutu = parseIntSafe(parts[1])
			}
		}

		// Look for IPK: "IPK : 3.65"
		if strings.Contains(normalized, "IPK") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.IPK = parseFloatSafe(parts[1])
			}
		}
	}
}

// parseKHSPenerbitan extracts publication info from KHS.
// Handles Indonesian date formats: "6 Agustus 2026", "06 Agustus 2026"
func parseKHSPenerbitan(lines []string, result *entity.KHSExtraction) {
	for _, line := range lines {
		normalized := NormalizeLabel(line)
		// Look for "Dikeluarkan di" pattern
		if strings.Contains(normalized, "Dikeluarkan di") || strings.Contains(normalized, "dikeluarkan di") {
			parts := strings.SplitN(line, ",", 2)
			if len(parts) == 2 {
				result.KHS.Penerbitan.Tempat = strings.TrimSpace(parts[0])
				dateStr := strings.TrimSpace(parts[1])

				// Try Indonesian date format first
				for indo, eng := range indonesianMonthsKHS {
					if strings.Contains(dateStr, indo) {
						dateStr = strings.Replace(dateStr, indo, eng, 1)
						break
					}
				}

				// Try multiple date formats
				for _, format := range dateFormatsKHS {
					if t, err := time.Parse(format, dateStr); err == nil {
						result.KHS.Penerbitan.Tanggal = t.Format(dateOutputFormatKHS)
						break
					}
				}
			}
		}
	}
}

// parseKHSPersetujuan extracts approval section from KHS.
func parseKHSPersetujuan(lines []string, result *entity.KHSExtraction) {
	for i, line := range lines {
		normalized := NormalizeLabel(line)
		if strings.Contains(normalized, "Dekan") && strings.Contains(normalized, "Fakultas") {
			result.KHS.Persetujuan.Dekan.Jabatan = line
			// Look for name in next lines
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.TrimSpace(lines[j]) != "" &&
					!strings.Contains(lines[j], "NIDN") &&
					!strings.Contains(lines[j], "(") {
					name := strings.TrimSpace(lines[j])
					result.KHS.Persetujuan.Dekan.Nama = name
					break
				}
			}
			// Look for NIDN
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], "NIDN") {
					parts := strings.SplitN(lines[j], ":", 2)
					if len(parts) == 2 {
						result.KHS.Persetujuan.Dekan.NIDN = strings.TrimSpace(parts[1])
					}
					break
				}
			}
		}
	}
}
