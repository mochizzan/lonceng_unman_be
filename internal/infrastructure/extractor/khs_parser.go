package extractor

import (
	"fmt"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// ParseKHS extracts structured KHS data from a PDF file.
func ParseKHS(path string, npm string, tahunAjaran string, semester string) (*entity.KHSExtraction, error) {
	rows, err := ReadPDFWithPosition(path)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	result := &entity.KHSExtraction{}
	result.KHS.Mahasiswa.NPM = npm
	result.KHS.Periode.TahunAjaran = tahunAjaran
	result.KHS.Periode.Semester = semester

	// Extract all lines for section detection
	lines := RowsToLines(rows)

	// Parse sections
	parseKHSHeader(lines, result)
	parseKHSMataKuliah(rows, lines, result)
	parseKHSRekapitulasi(lines, result)
	parseKHSPenerbitan(lines, result)
	parseKHSPersetujuan(lines, result)

	// Set metadata
	result.Metadata.ExtractedAt = time.Now()
	result.Metadata.SourceFile = path

	return result, nil
}

// parseKHSHeader extracts student info from header section.
func parseKHSHeader(lines []string, result *entity.KHSExtraction) {
	for i, line := range lines {
		normalized := NormalizeLabel(line)

		// Look for student name pattern: "Nama : XXXX" or "Nama" + next line ": XXXX"
		if strings.Contains(normalized, "Nama") {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					result.KHS.Mahasiswa.Nama = strings.TrimSpace(parts[1])
				}
			} else if val := FindNextValueLine(lines, i); val != "" {
				result.KHS.Mahasiswa.Nama = val
			}
		}

		// Look for NPM pattern: "NPM : XXXX" or "N P M" + next line ": XXXX"
		if strings.Contains(normalized, "NPM") {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					npm := strings.TrimSpace(parts[1])
					if npm != "" {
						result.KHS.Mahasiswa.NPM = npm
					}
				}
			} else if val := FindNextValueLine(lines, i); val != "" {
				result.KHS.Mahasiswa.NPM = val
			}
		}

		// Look for program studi: "Program Studi : XXXX" or "Program Studi" + next line ": XXXX"
		if strings.Contains(normalized, "Program Studi") {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					result.KHS.Mahasiswa.ProgramStudi = strings.TrimSpace(parts[1])
				}
			} else if val := FindNextValueLine(lines, i); val != "" {
				result.KHS.Mahasiswa.ProgramStudi = val
			}
		}
	}
}

// parseKHSMataKuliah extracts course table from KHS.
func parseKHSMataKuliah(rows []PDFRow, lines []string, result *entity.KHSExtraction) {
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

	// Find table end (look for "Rekapitulasi" or "Total")
	tableEnd := len(lines)
	for i := tableStart; i < len(lines); i++ {
		if strings.Contains(lines[i], "Rekapitulasi") || strings.Contains(lines[i], "Total") {
			tableEnd = i
			break
		}
	}

	// Parse table rows
	var courses []entity.KHSMataKuliah
	courseNo := 1

	for i := tableStart; i < tableEnd && i < len(rows); i++ {
		row := rows[i]
		if len(row.Words) < 5 {
			continue // skip non-table rows
		}

		line := RowToLine(row)

		// Skip header-like rows
		if strings.Contains(line, "No") && strings.Contains(line, "Kode") {
			continue
		}

		// Parse course data
		course := entity.KHSMataKuliah{
			No: courseNo,
		}

		words := row.Words
		if len(words) >= 6 {
			course.Kode = words[1].Text
			course.Nama = words[2].Text

			// Try to find nilai (grade) and mutu
			for j := 3; j < len(words); j++ {
				nilai := strings.ToUpper(words[j].Text)
				if nilai == "A" || nilai == "B" || nilai == "C" || nilai == "D" || nilai == "E" {
					course.Nilai = nilai
					// Mutu is usually next to nilai
					if j+1 < len(words) {
						course.Mutu = parseIntSafe(words[j+1].Text)
					}
					break
				}
			}

			// Get dosen and SKS
			for j := 3; j < len(words); j++ {
				if sks := parseIntSafe(words[j].Text); sks > 0 && sks <= 12 {
					course.SKS = sks
					break
				}
			}

			// Get dosen from remaining words
			if len(words) > 5 {
				parts := make([]string, 0, len(words)-3)
				for _, w := range words[3:] {
					parts = append(parts, w.Text)
				}
				course.Dosen = strings.Join(parts, " ")
			}
		}

		if course.Kode != "" {
			courses = append(courses, course)
			courseNo++
		}
	}

	result.KHS.MataKuliah = courses
}

// parseKHSRekapitulasi extracts summary stats from KHS.
func parseKHSRekapitulasi(lines []string, result *entity.KHSExtraction) {
	for _, line := range lines {
		// Look for total SKS: "Total SKS : 23"
		if strings.Contains(line, "Total SKS") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.TotalSKS = parseIntSafe(parts[1])
			}
		}

		// Look for total mutu: "Total Mutu : 84"
		if strings.Contains(line, "Total Mutu") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result.KHS.Rekapitulasi.TotalMutu = parseIntSafe(parts[1])
			}
		}

		// Look for IPK: "IPK : 3.65"
		if strings.Contains(line, "IPK") && strings.Contains(line, ":") {
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
				result.KHS.Penerbitan.Tempat = strings.TrimSpace(parts[0])
				dateStr := strings.TrimSpace(parts[1])

				// Try Indonesian date format first
				for indo, eng := range indonesianMonths {
					if strings.Contains(dateStr, indo) {
						dateStr = strings.Replace(dateStr, indo, eng, 1)
						break
					}
				}

				// Try multiple date formats
				dateFormats := []string{"2 January 2006", "02 January 2006", "January 2, 2006"}
				for _, format := range dateFormats {
					if t, err := time.Parse(format, dateStr); err == nil {
						result.KHS.Penerbitan.Tanggal = t.Format("2006-01-02")
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
		if strings.Contains(line, "Dekan") && strings.Contains(line, "Fakultas") {
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
