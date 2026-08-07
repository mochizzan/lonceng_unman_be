package extractor

import (
	"fmt"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// ParseKHS extracts structured KHS data from a PDF file.
// It uses ReadPDF (plain text) for header fields to preserve word spacing,
// and ReadPDFWithPosition for table data that needs column positions.
func ParseKHS(path string, npm string, tahunAjaran string, semester string) (*entity.KHSExtraction, error) {
	result := &entity.KHSExtraction{}
	result.KHS.Mahasiswa.NPM = npm
	result.KHS.Periode.Semester = semester
	result.KHS.Periode.TahunAjaran = splitTahunAjaran(tahunAjaran)

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
	result.KHS.Penerbitan = toEntityPenerbitan(parsePenerbitanFromLines(lines))
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

// parseKHSMataKuliah extracts course table from KHS.
// KHS PDF layout has multi-row course entries:
//
//	Row before: "Kepemimpinan" (course name, standalone)
//	Course row: "1 KK21250323 3 A 12" (No, Kode, SKS, Nilai, Mutu)
//	Row after:  "Haris Nizhomul Haq, S.Kom" (dosen name, standalone)
//
// Strategy: find course data rows by numeric pattern, then look
// nearby rows for nama (before) and dosen (after) using Y-position proximity.
func parseKHSMataKuliah(rows []PDFRow, lines []string, result *entity.KHSExtraction) {
	// Find the header row to identify column boundaries
	var headerRow *PDFRow
	var headerIdx int
	for i, row := range rows {
		line := RowToLine(row)
		normalized := NormalizeLabel(line)
		if strings.Contains(normalized, "Kode") && strings.Contains(normalized, "Mata Kuliah") {
			headerRow = &rows[i]
			headerIdx = i
			break
		}
	}

	if headerRow == nil {
		return
	}

	// Identify column X boundaries from the header row
	// KHS columns: No, Kode, SKS, Nilai, Mutu (Mata Kuliah and Dosen are NOT in columns)
	khsDataColumns := []string{"No", "Kode", "SKS", "Nilai", "Mutu"}
	boundaries := FindColumnPositions(*headerRow, khsDataColumns)
	if boundaries == nil {
		return
	}

	// First pass: find all course data rows
	type courseEntry struct {
		rowIdx int
		row    PDFRow
		kode   string
		no     int
		sks    int
		nilai  string
		mutu   int
	}

	var courseEntries []courseEntry
	seenKodes := make(map[string]bool)

	for i, row := range rows {
		if i <= headerIdx {
			continue
		}
		if len(row.Words) == 0 {
			continue
		}

		line := RowToLine(row)

		// Skip Total/IPK rows
		if strings.Contains(line, "Total") || strings.Contains(line, "IPK") {
			continue
		}

		// Course data rows MUST start with a digit (the No. column)
		// e.g., "1 KK21250323 3 A 12" — but NOT "Kewirausahaan 1"
		trimmedLine := strings.TrimSpace(line)
		if len(trimmedLine) == 0 || trimmedLine[0] < '1' || trimmedLine[0] > '9' {
			continue
		}

		// Extract columns
		cols := ExtractColumnsFromRow(row, boundaries)

		// A course row must have a valid Kode
		kode := strings.TrimSpace(cols["Kode"])
		if kode == "" || len(kode) < 3 {
			continue
		}

		// Must contain at least one digit AND one letter (course codes are alphanumeric like KK21250323)
		hasDigit := false
		hasLetter := false
		for _, r := range kode {
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				hasLetter = true
			}
		}
		if !hasDigit || !hasLetter {
			continue
		}

		// Deduplicate
		if seenKodes[kode] {
			continue
		}
		seenKodes[kode] = true

		no := parseIntSafe(cols["No"])
		sks := parseIntSafe(cols["SKS"])
		nilai := strings.ToUpper(strings.TrimSpace(cols["Nilai"]))
		mutu := parseIntSafe(cols["Mutu"])

		courseEntries = append(courseEntries, courseEntry{
			rowIdx: i,
			row:    row,
			kode:   kode,
			no:     no,
			sks:    sks,
			nilai:  nilai,
			mutu:   mutu,
		})
	}

	// Second pass: for each course entry, find nama (row before) and dosen (row after)
	var courses []entity.KHSMataKuliah
	for idx, entry := range courseEntries {
		nama := ""
		dosen := ""

		// Search rows BEFORE for course name (closest non-empty row that looks like a name)
		for j := entry.rowIdx - 1; j > headerIdx && j >= entry.rowIdx-3; j-- {
			line := strings.TrimSpace(RowToLine(rows[j]))
			if line == "" {
				continue
			}
			// Skip rows that look like course data (start with digit)
			if len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
				continue
			}
			nama = line
			break
		}

		// Search rows AFTER for dosen name (closest non-empty row)
		endSearch := len(rows)
		if idx+1 < len(courseEntries) {
			endSearch = courseEntries[idx+1].rowIdx
		}
		for j := entry.rowIdx + 1; j < endSearch && j <= entry.rowIdx+3; j++ {
			line := strings.TrimSpace(RowToLine(rows[j]))
			if line == "" {
				continue
			}
			// Skip rows that look like course data (start with digit)
			if len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
				continue
			}
			dosen = line
			break
		}

		courses = append(courses, entity.KHSMataKuliah{
			No:    entry.no,
			Kode:  entry.kode,
			Nama:  nama,
			Dosen: dosen,
			SKS:   entry.sks,
			Nilai: entry.nilai,
			Mutu:  entry.mutu,
		})
	}

	result.KHS.MataKuliah = courses
}

// parseKHSRekapitulasi extracts summary stats from KHS.
// Handles two formats:
//   - "Total SKS : 23" (with colon, labeled)
//   - "Total 23 84" (space-separated, unlabeled: Total <SKS> <Mutu>)
//   - "IPK 3.65" (space-separated)
func parseKHSRekapitulasi(lines []string, result *entity.KHSExtraction) {
	for _, line := range lines {
		// NormalizeLabel does NOT lowercase, so we lowercase manually
		normalized := strings.ToLower(NormalizeLabel(line))

		// Format 1: "Total SKS : 23" (with colon)
		if strings.Contains(normalized, "total sks") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.TotalSKS = parseIntSafe(parts[1])
			}
		}

		// Format 2: "Total 23 84" (space-separated, no colon)
		if strings.HasPrefix(normalized, "total ") && !strings.Contains(line, ":") {
			parts := strings.Fields(normalized)
			if len(parts) >= 3 {
				result.KHS.Rekapitulasi.TotalSKS = parseIntSafe(parts[1])
				result.KHS.Rekapitulasi.TotalMutu = parseIntSafe(parts[2])
			}
		}

		// Format 1: "Total Mutu : 84" (with colon)
		if strings.Contains(normalized, "total mutu") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.TotalMutu = parseIntSafe(parts[1])
			}
		}

		// Format: "IPK 3.65" or "IPK : 3.65"
		if strings.Contains(normalized, "ipk") {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					result.KHS.Rekapitulasi.IPK = parseFloatSafe(parts[1])
				}
			} else {
				parts := strings.Fields(normalized)
				for i, p := range parts {
					if p == "ipk" && i+1 < len(parts) {
						result.KHS.Rekapitulasi.IPK = parseFloatSafe(parts[i+1])
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
