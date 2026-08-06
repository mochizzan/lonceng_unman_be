package extractor

import (
	"fmt"
	"os"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// ParseKHS extracts structured KHS data from a PDF file.
// It uses ReadPDF (plain text) for header fields to preserve word spacing,
// and ReadPDFWithPosition for table data that needs column positions.
func ParseKHS(path string, npm string, tahunAjaran string, semester string) (*entity.KHSExtraction, error) {
	// Validate file exists before parsing
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pdf file not found: %s", path)
		}
		return nil, fmt.Errorf("stat pdf: %w", err)
	}

	result := &entity.KHSExtraction{}
	result.KHS.Mahasiswa.NPM = npm
	result.KHS.Periode.Semester = semester

	// Split tahun ajaran "2025/2026" into awal/akhir
	if tahunAjaran != "" {
		parts := strings.SplitN(tahunAjaran, "/", 2)
		if len(parts) == 2 {
			result.KHS.Periode.TahunAjaran.Awal = strings.TrimSpace(parts[0])
			result.KHS.Periode.TahunAjaran.Akhir = strings.TrimSpace(parts[1])
		} else {
			result.KHS.Periode.TahunAjaran.Awal = tahunAjaran
		}
	}

	// Use plain text for header fields (preserves spaces)
	plainText, err := ReadPDF(path)
	if err == nil {
		lines := strings.Split(plainText, "\n")
		parsePlainTextHeaderKHS(lines, result)
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
// It delegates to parseHeaderFields which handles both horizontal
// ("Label : Value") and vertical (label/colon/value) formats.
func parsePlainTextHeaderKHS(lines []string, result *entity.KHSExtraction) {
	labels := []string{"N P M", "Nama", "Program Studi"}
	fields := parseHeaderFields(lines, labels)
	result.KHS.Mahasiswa.NPM = fields["N P M"]
	result.KHS.Mahasiswa.Nama = fields["Nama"]
	result.KHS.Mahasiswa.ProgramStudi = fields["Program Studi"]
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
// Delegates to parsePenerbitanFromLines which handles both formats:
// "Subang, 06 Agustus 2026" and "Dikeluarkan di Subang, 06 Agustus 2026"
func parseKHSPenerbitan(lines []string, result *entity.KHSExtraction) {
	p := parsePenerbitanFromLines(lines)
	result.KHS.Penerbitan.Tempat = p.Tempat
	result.KHS.Penerbitan.Tanggal = p.Tanggal
}

// parseKHSPersetujuan extracts approval section from KHS.
func parseKHSPersetujuan(lines []string, result *entity.KHSExtraction) {
	for i, line := range lines {
		normalized := NormalizeLabel(line)
		if strings.Contains(normalized, "Dekan") && strings.Contains(normalized, "Fakultas") {
			result.KHS.Persetujuan.Dekan.Jabatan = line
			result.KHS.Persetujuan.Dekan.Nama = extractNextNonEmptyAfterLabel(lines, i, func(s string) bool {
				return true
			})
			// Extract NIDN
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
