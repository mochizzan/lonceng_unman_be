package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// EvalService implements port.EvalService.
type EvalService struct {
	store port.EvalStore
}

// NewEvalService creates a new eval service.
func NewEvalService(store port.EvalStore) *EvalService {
	return &EvalService{store: store}
}

// Index returns unified evaluation data with single table.
func (s *EvalService) Index() (entity.UnifiedEval, error) {
	npms, err := s.store.ListNPMs()
	if err != nil {
		return entity.UnifiedEval{}, err
	}

	var allRows []entity.UnifiedTableRow
	var krsTP, krsFN, krsFP, krsTN int
	var khsTP, khsFN, khsFP, khsTN int

	for _, npm := range npms {
		docs, err := s.store.ListDocs(npm)
		if err != nil {
			continue
		}

		// Get student info
		name := ""
		for _, doc := range docs {
			if doc.HasGT {
				gtData, err := s.store.LoadGT(npm, doc.DocType, doc.File)
				if err == nil {
					if doc.DocType == "khs" {
						var gt entity.KHSExtraction
						if err := json.Unmarshal(gtData, &gt); err == nil {
							name = gt.KHS.Mahasiswa.Nama
							break
						}
					} else {
						var gt entity.KRSExtraction
						if err := json.Unmarshal(gtData, &gt); err == nil {
							name = gt.KRS.Mahasiswa.Nama
							break
						}
					}
				}
			}
		}

		for _, doc := range docs {
			row := entity.IndexRow{
				NPM:        npm,
				Name:       name,
				DocType:    doc.DocType,
				File:       doc.File,
				Paired:     doc.Paired,
				HasGT:      doc.HasGT,
				HasExtract: doc.HasExtract,
			}

			if doc.DocType == "krs" {
				row.Semester = parseSemester(doc.File)
			} else {
				row.TahunAjaran = parseTahunAjaran(doc.File)
			}

			// Show ALL documents: Paired, HasGT only, or HasExtract only
			unifiedRow := entity.UnifiedTableRow{
				NPM:         npm,
				Name:        name,
				DocType:     doc.DocType,
				Category:    docTypeToCategory(doc.DocType),
				Semester:    row.Semester,
				TahunAjaran: row.TahunAjaran,
				File:        doc.File,
				Paired:      doc.Paired,
				HasGT:       doc.HasGT,
				HasExtract:  doc.HasExtract,
				HasRaw:      doc.HasRaw,
			}

			// Determine status, badge class, and action
			switch {
			case doc.Paired:
				unifiedRow.Status = "Paired"
				unifiedRow.StatusClass = "badge-paired"
				unifiedRow.Action = "Edit GT"
			case doc.HasGT && !doc.HasExtract:
				unifiedRow.Status = "Butuh Extract"
				unifiedRow.StatusClass = "badge-need-extract"
				unifiedRow.Action = "Edit GT"
			case !doc.HasGT && doc.HasExtract:
				unifiedRow.Status = "Butuh GT"
				unifiedRow.StatusClass = "badge-need-gt"
				unifiedRow.Action = "Buat GT"
			default:
				unifiedRow.Status = "Belum Ada"
				unifiedRow.StatusClass = "badge-unused"
				unifiedRow.Action = "—"
			}

			// Compute metrics only for paired docs
			if doc.Paired {
				metrics, err := s.evaluateDoc(npm, doc.DocType, doc.File)
				if err == nil {
					row.Metrics = metrics
					unifiedRow.Metrics = metrics
					unifiedRow.TP = metrics.Confusion.TP
					unifiedRow.FN = metrics.Confusion.FN
					unifiedRow.FP = metrics.Confusion.FP
					unifiedRow.Precision = metrics.Precision
					unifiedRow.Recall = metrics.Recall
					unifiedRow.F1 = metrics.F1

					if doc.DocType == "krs" {
						krsTP += metrics.Confusion.TP
						krsFN += metrics.Confusion.FN
						krsFP += metrics.Confusion.FP
						krsTN += metrics.Confusion.TN
					} else {
						khsTP += metrics.Confusion.TP
						khsFN += metrics.Confusion.FN
						khsFP += metrics.Confusion.FP
						khsTN += metrics.Confusion.TN
					}
				}
			}

			allRows = append(allRows, unifiedRow)
		}
	}

	// Sort rows
	sort.Slice(allRows, func(i, j int) bool {
		if allRows[i].NPM != allRows[j].NPM {
			return allRows[i].NPM < allRows[j].NPM
		}
		return allRows[i].File < allRows[j].File
	})

	return entity.UnifiedEval{
		TotalNPMs:   len(npms),
		UnifiedRows: allRows,
		KRSMetrics:  computeMetrics(krsTP, krsFN, krsFP, krsTN),
		KHSMetrics:  computeMetrics(khsTP, khsFN, khsFP, khsTN),
		RawPDFURL:   "/eval/static/pdf-preview.html",
	}, nil
}

// Student returns the full evaluation for a single NPM.
func (s *EvalService) Student(npm string) (entity.StudentEval, error) {
	result := entity.StudentEval{NPM: npm}

	docs, err := s.store.ListDocs(npm)
	if err != nil {
		return result, err
	}

	// Get student info
	for _, doc := range docs {
		if doc.HasGT {
			gtData, err := s.store.LoadGT(npm, doc.DocType, doc.File)
			if err == nil {
				if doc.DocType == "khs" {
					var gt entity.KHSExtraction
					if err := json.Unmarshal(gtData, &gt); err == nil {
						result.Name = gt.KHS.Mahasiswa.Nama
						result.ProgramStudi = gt.KHS.Mahasiswa.ProgramStudi
						break
					}
				} else {
					var gt entity.KRSExtraction
					if err := json.Unmarshal(gtData, &gt); err == nil {
						result.Name = gt.KRS.Mahasiswa.Nama
						result.ProgramStudi = gt.KRS.Mahasiswa.ProgramStudi
						break
					}
				}
			}
		}
	}

	var krsTP, krsFN, krsFP, krsTN int
	var khsTP, khsFN, khsFP, khsTN int

	for _, doc := range docs {
		docEval := entity.DocEval{
			File:       doc.File,
			Paired:     doc.Paired,
			HasGT:      doc.HasGT,
			HasExtract: doc.HasExtract,
			HasRaw:     doc.HasRaw,
		}

		if doc.DocType == "krs" {
			docEval.Semester = parseSemester(doc.File)
		} else {
			docEval.TahunAjaran = parseTahunAjaran(doc.File)
		}

		if doc.Paired {
			metrics, err := s.evaluateDoc(npm, doc.DocType, doc.File)
			if err == nil {
				docEval.Metrics = metrics
				docEval.Heatmap = buildHeatmapFromMetrics(metrics)
				docEval.CompareRows = s.buildCompareRows(npm, doc.DocType, doc.File)

				if doc.DocType == "krs" {
					krsTP += metrics.Confusion.TP
					krsFN += metrics.Confusion.FN
					krsFP += metrics.Confusion.FP
					krsTN += metrics.Confusion.TN
				} else {
					khsTP += metrics.Confusion.TP
					khsFN += metrics.Confusion.FN
					khsFP += metrics.Confusion.FP
					khsTN += metrics.Confusion.TN
				}
			}
		}

		if doc.DocType == "krs" {
			result.KRSDocs = append(result.KRSDocs, docEval)
		} else {
			result.KHSDocs = append(result.KHSDocs, docEval)
		}
	}

	result.KRSMetrics = computeMetrics(krsTP, krsFN, krsFP, krsTN)
	result.KHSMetrics = computeMetrics(khsTP, khsFN, khsFP, khsTN)

	return result, nil
}

// LoadGT exposes store.LoadGT for handler use (PDF preview loading).
func (s *EvalService) LoadGT(npm, docType, filename string) ([]byte, error) {
	return s.store.LoadGT(npm, docType, filename)
}

// LoadExtract exposes store.LoadExtract for handler use.
func (s *EvalService) LoadExtract(npm, docType, filename string) ([]byte, error) {
	return s.store.LoadExtract(npm, docType, filename)
}

// evaluateDoc evaluates a single document pair.
func (s *EvalService) evaluateDoc(npm, docType, filename string) (entity.Metrics, error) {
	gtData, err := s.store.LoadGT(npm, docType, filename)
	if err != nil {
		return entity.Metrics{}, err
	}
	extractData, err := s.store.LoadExtract(npm, docType, filename)
	if err != nil {
		return entity.Metrics{}, err
	}

	var tp, fn, fp, tn int
	heatmapAgg := make(map[string]*entity.FieldHeatmapRow)

	if docType == "khs" {
		var gt, extract entity.KHSExtraction
		if err := json.Unmarshal(gtData, &gt); err != nil {
			return entity.Metrics{}, err
		}
		if err := json.Unmarshal(extractData, &extract); err != nil {
			return entity.Metrics{}, err
		}

		t, f, p, tn0 := s.compareKHSHeaders(&gt, &extract, heatmapAgg)
		tp += t
		fn += f
		fp += p
		tn += tn0

		courseResults := MatchKHSCourses(gt.KHS.MataKuliah, extract.KHS.MataKuliah)
		for _, cr := range courseResults {
			if !cr.Matched {
				if cr.IsGTOnly {
					fn += 6
				} else {
					fp += 6
				}
			}
		}
	} else {
		var gt, extract entity.KRSExtraction
		if err := json.Unmarshal(gtData, &gt); err != nil {
			return entity.Metrics{}, err
		}
		if err := json.Unmarshal(extractData, &extract); err != nil {
			return entity.Metrics{}, err
		}

		t, f, p, tn0 := s.compareHeaders(&gt, &extract, heatmapAgg)
		tp += t
		fn += f
		fp += p
		tn += tn0

		courseResults := MatchCourses(gt.KRS.MataKuliah, extract.KRS.MataKuliah)
		for _, cr := range courseResults {
			if !cr.Matched {
				if cr.IsGTOnly {
					fn += 8
				} else {
					fp += 8
				}
			}
		}
	}

	return computeMetrics(tp, fn, fp, tn), nil
}

// compareKHSHeaders compares KHS header fields between GT and extract.
func (s *EvalService) compareKHSHeaders(gt, extract *entity.KHSExtraction, heatmapAgg map[string]*entity.FieldHeatmapRow) (tp, fn, fp, tn int) {
	comparisons := []struct {
		name string
		gt   string
		pred string
	}{
		{"mahasiswa.nama", gt.KHS.Mahasiswa.Nama, extract.KHS.Mahasiswa.Nama},
		{"mahasiswa.program_studi", gt.KHS.Mahasiswa.ProgramStudi, extract.KHS.Mahasiswa.ProgramStudi},
		{"periode.semester", gt.KHS.Periode.Semester, extract.KHS.Periode.Semester},
		{"periode.tahun_ajaran.awal", gt.KHS.Periode.TahunAjaran.Awal, extract.KHS.Periode.TahunAjaran.Awal},
		{"periode.tahun_ajaran.akhir", gt.KHS.Periode.TahunAjaran.Akhir, extract.KHS.Periode.TahunAjaran.Akhir},
		{"rekapitulasi.total_sks", strconv.Itoa(gt.KHS.Rekapitulasi.TotalSKS), strconv.Itoa(extract.KHS.Rekapitulasi.TotalSKS)},
		{"rekapitulasi.total_mutu", strconv.Itoa(gt.KHS.Rekapitulasi.TotalMutu), strconv.Itoa(extract.KHS.Rekapitulasi.TotalMutu)},
		{"rekapitulasi.ipk", fmt.Sprintf("%.2f", gt.KHS.Rekapitulasi.IPK), fmt.Sprintf("%.2f", extract.KHS.Rekapitulasi.IPK)},
	}

	for _, c := range comparisons {
		isIPK := c.name == "rekapitulasi.ipk"
		isInt := c.name == "rekapitulasi.total_sks" || c.name == "rekapitulasi.total_mutu"
		result := CompareFields(c.gt, c.pred, false, isInt, isIPK, false)
		t, f, p, tn0 := countResult(result.Status)
		tp += t
		fn += f
		fp += p
		tn += tn0
		updateHeatmap(heatmapAgg, c.name, result.Status)
	}

	return tp, fn, fp, tn
}

// compareHeaders compares header fields between GT and extract.
func (s *EvalService) compareHeaders(gt, extract *entity.KRSExtraction, heatmapAgg map[string]*entity.FieldHeatmapRow) (tp, fn, fp, tn int) {
	comparisons := []struct {
		name string
		gt   string
		pred string
	}{
		{"mahasiswa.nama", gt.KRS.Mahasiswa.Nama, extract.KRS.Mahasiswa.Nama},
		{"mahasiswa.program_studi", gt.KRS.Mahasiswa.ProgramStudi, extract.KRS.Mahasiswa.ProgramStudi},
		{"periode.semester", gt.KRS.Periode.Semester, extract.KRS.Periode.Semester},
		{"periode.tahun_ajaran.awal", gt.KRS.Periode.TahunAjaran.Awal, extract.KRS.Periode.TahunAjaran.Awal},
		{"periode.tahun_ajaran.akhir", gt.KRS.Periode.TahunAjaran.Akhir, extract.KRS.Periode.TahunAjaran.Akhir},
	}

	for _, c := range comparisons {
		result := CompareFields(c.gt, c.pred, false, false, false, false)
		t, f, p, tn0 := countResult(result.Status)
		tp += t
		fn += f
		fp += p
		tn += tn0
		updateHeatmap(heatmapAgg, c.name, result.Status)
	}

	return tp, fn, fp, tn
}

// buildCompareRows builds the compare rows for the detail page.
func (s *EvalService) buildCompareRows(npm, docType, filename string) []entity.CompareRow {
	gtData, err := s.store.LoadGT(npm, docType, filename)
	if err != nil {
		return []entity.CompareRow{}
	}
	extractData, err := s.store.LoadExtract(npm, docType, filename)
	if err != nil {
		return []entity.CompareRow{}
	}

	if docType == "khs" {
		var gt, extract entity.KHSExtraction
		if err := json.Unmarshal(gtData, &gt); err != nil {
			return []entity.CompareRow{}
		}
		if err := json.Unmarshal(extractData, &extract); err != nil {
			return []entity.CompareRow{}
		}
		return BuildCompareRowsKHS(&gt, &extract)
	}

	var gt, extract entity.KRSExtraction
	if err := json.Unmarshal(gtData, &gt); err != nil {
		return []entity.CompareRow{}
	}
	if err := json.Unmarshal(extractData, &extract); err != nil {
		return []entity.CompareRow{}
	}
	return BuildCompareRows(&gt, &extract)
}

// buildHeatmapFromMetrics builds heatmap from metrics.
func buildHeatmapFromMetrics(m entity.Metrics) []entity.FieldHeatmapRow {
	return []entity.FieldHeatmapRow{}
}

// countResult converts FieldResult status to confusion counts.
func countResult(status string) (tp, fn, fp, tn int) {
	switch status {
	case "tp":
		return 1, 0, 0, 0
	case "fn":
		return 0, 1, 0, 0
	case "fp":
		return 0, 0, 1, 0
	case "tn":
		return 0, 0, 0, 1
	case "wrong":
		// A mismatched field (both GT and pred filled but not equal) is counted
		// as a false-negative only: the GT value was missed by the prediction.
		// Counting it as both FN+FP would inflate denominators and produce
		// non-standard precision/recall/F1 scores.
		return 0, 1, 0, 0
	}
	return 0, 0, 0, 0
}

// updateHeatmap updates the heatmap aggregation.
func updateHeatmap(heatmapAgg map[string]*entity.FieldHeatmapRow, field, status string) {
	if _, ok := heatmapAgg[field]; !ok {
		heatmapAgg[field] = &entity.FieldHeatmapRow{Field: field}
	}
	row := heatmapAgg[field]
	switch status {
	case "tp":
		row.Correct++
	case "wrong":
		row.Wrong++
	case "fn":
		row.Missing++
	case "fp":
		row.Extra++
	}
}

// computeMetrics computes precision, recall, F1, accuracy.
func computeMetrics(tp, fn, fp, tn int) entity.Metrics {
	m := entity.Metrics{
		Confusion: entity.ConfusionCounts{TP: tp, FN: fn, FP: fp, TN: tn},
	}

	if tp+fp > 0 {
		m.Precision = float64(tp) / float64(tp+fp)
	}
	if tp+fn > 0 {
		m.Recall = float64(tp) / float64(tp+fn)
	}
	if m.Precision+m.Recall > 0 {
		m.F1 = 2 * m.Precision * m.Recall / (m.Precision + m.Recall)
	}
	if tp+tn+fp+fn > 0 {
		m.Accuracy = float64(tp+tn) / float64(tp+tn+fp+fn)
	}

	return m
}

// parseSemester extracts semester number from filename like "semester_8.json"
func parseSemester(filename string) string {
	base := strings.TrimSuffix(filename, ".json")
	parts := strings.Split(base, "_")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return base
}

// parseTahunAjaran extracts tahun ajaran from filename like "2025_2026_GENAP.json"
func parseTahunAjaran(filename string) string {
	base := strings.TrimSuffix(filename, ".json")
	parts := strings.Split(base, "_")
	if len(parts) >= 3 {
		return parts[0] + "/" + parts[1] + " " + parts[2]
	}
	return base
}

// docTypeToCategory converts internal doc type to display category.
func docTypeToCategory(docType string) string {
	if docType == "krs" {
		return "KRS"
	}
	return "KHS"
}
