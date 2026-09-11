package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
	"lonceng_unman_be/internal/infrastructure/evalstore"
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

	// Per-request cached store: each file read from disk at most once.
	cs := evalstore.NewCachedStore(s.store)

	var allRows []entity.UnifiedTableRow
	var krsTP, krsFN, krsFP, krsTN int
	var khsTP, khsFN, khsFP, khsTN int

	for _, npm := range npms {
		docs, err := cs.ListDocs(npm)
		if err != nil {
			continue
		}

		// Get student info using partial decode (only extract "nama" field).
		name := s.fastNameLookupWithStore(cs, npm)

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
				metrics, err := s.evaluateDocWithStore(cs, npm, doc.DocType, doc.File)
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

	// Per-request cached store: each file read from disk at most once.
	cs := evalstore.NewCachedStore(s.store)

	docs, err := cs.ListDocs(npm)
	if err != nil {
		return result, err
	}

	// Get student info using partial decode (only extract "nama" field).
	name, programStudi := s.studentInfoWithStore(cs, npm)
	result.Name = name
	result.ProgramStudi = programStudi

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
			metrics, err := s.evaluateDocWithStore(cs, npm, doc.DocType, doc.File)
			if err == nil {
				docEval.Metrics = metrics
				docEval.Heatmap = buildHeatmapFromMetrics(metrics)
				docEval.CompareRows = s.buildCompareRowsWithStore(cs, npm, doc.DocType, doc.File)

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

// StudentEntry is a lightweight struct for the student list page.
// It contains only the data needed for the list view — no metrics.
type StudentEntry = entity.StudentEntry

// StudentList returns all NPMs with their names WITHOUT expensive metrics computation.
// Uses partial JSON decode to extract only the "nama" field from GT.
func (s *EvalService) StudentList() ([]StudentEntry, error) {
	npms, err := s.store.ListNPMs()
	if err != nil {
		return nil, err
	}
	entries := make([]StudentEntry, 0, len(npms))
	for _, npm := range npms {
		name := s.FastNameLookup(npm)
		entries = append(entries, StudentEntry{NPM: npm, Name: name})
	}
	return entries, nil
}

// fastNameLookupWithStore does a partial JSON decode to extract only the "nama" field
// using the provided store (which may be cached). Avoids full evaluateDoc().
func (s *EvalService) fastNameLookupWithStore(st port.EvalStore, npm string) string {
	docs, _ := st.ListDocs(npm)
	for _, doc := range docs {
		if !doc.HasGT {
			continue
		}
		gtData, err := st.LoadGT(npm, doc.DocType, doc.File)
		if err != nil {
			continue
		}
		// Partial decode — only extract "khs.mahasiswa.nama" or "krs.mahasiswa.nama"
		var partial struct {
			KHS struct {
				Mahasiswa struct {
					Nama string `json:"nama"`
				} `json:"mahasiswa"`
			} `json:"khs"`
			KRS struct {
				Mahasiswa struct {
					Nama string `json:"nama"`
				} `json:"mahasiswa"`
			} `json:"krs"`
		}
		if err := json.Unmarshal(gtData, &partial); err == nil {
			if partial.KHS.Mahasiswa.Nama != "" {
				return partial.KHS.Mahasiswa.Nama
			}
			if partial.KRS.Mahasiswa.Nama != "" {
				return partial.KRS.Mahasiswa.Nama
			}
		}
	}
	return ""
}

// studentInfoWithStore extracts nama and program_studi using partial decode.
func (s *EvalService) studentInfoWithStore(st port.EvalStore, npm string) (name, programStudi string) {
	docs, _ := st.ListDocs(npm)
	for _, doc := range docs {
		if !doc.HasGT {
			continue
		}
		gtData, err := st.LoadGT(npm, doc.DocType, doc.File)
		if err != nil {
			continue
		}
		// Partial decode — extract nama and program_studi
		var partial struct {
			KHS struct {
				Mahasiswa struct {
					Nama         string `json:"nama"`
					ProgramStudi string `json:"program_studi"`
				} `json:"mahasiswa"`
			} `json:"khs"`
			KRS struct {
				Mahasiswa struct {
					Nama         string `json:"nama"`
					ProgramStudi string `json:"program_studi"`
				} `json:"mahasiswa"`
			} `json:"krs"`
		}
		if err := json.Unmarshal(gtData, &partial); err == nil {
			if partial.KHS.Mahasiswa.Nama != "" {
				return partial.KHS.Mahasiswa.Nama, partial.KHS.Mahasiswa.ProgramStudi
			}
			if partial.KRS.Mahasiswa.Nama != "" {
				return partial.KRS.Mahasiswa.Nama, partial.KRS.Mahasiswa.ProgramStudi
			}
		}
	}
	return "", ""
}

// evaluateDocWithStore evaluates a single document pair using the provided store.
// FIX 2026-09-10: course-level was bug (TP konstan 8, FN/FP per-matkul +6/+8).
// Now field-level: each course field contributes TP/FN/FP individually via CompareFields.
func (s *EvalService) evaluateDocWithStore(st port.EvalStore, npm, docType, filename string) (entity.Metrics, error) {
	gtData, err := st.LoadGT(npm, docType, filename)
	if err != nil {
		return entity.Metrics{}, err
	}
	extractData, err := st.LoadExtract(npm, docType, filename)
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

		// Field-level KHS courses: 6 fields per course (kode,nama,dosen,sks,nilai,mutu)
		predMap := make(map[int]entity.KHSMataKuliah, len(extract.KHS.MataKuliah))
		for _, c := range extract.KHS.MataKuliah {
			predMap[c.No] = c
		}
		gtNos := make(map[int]bool, len(gt.KHS.MataKuliah))
		for _, gtMK := range gt.KHS.MataKuliah {
			gtNos[gtMK.No] = true
			predMK, exists := predMap[gtMK.No]
			if !exists {
				// whole course missing in pred -> 6 fields FN
				fn += 6
				continue
			}
			// 6 field comparisons per matched course
			fields := []struct {
				name    string
				gtVal   string
				predVal string
				isKode  bool
				isInt   bool
			}{
				{"kode", gtMK.Kode, predMK.Kode, true, false},
				{"nama", gtMK.Nama, predMK.Nama, false, false},
				{"dosen", gtMK.Dosen, predMK.Dosen, false, false},
				{"sks", strconv.Itoa(gtMK.SKS), strconv.Itoa(predMK.SKS), false, true},
				{"nilai", gtMK.Nilai, predMK.Nilai, false, false},
				{"mutu", strconv.Itoa(gtMK.Mutu), strconv.Itoa(predMK.Mutu), false, true},
			}
			for _, f := range fields {
				result := CompareFields(f.gtVal, f.predVal, f.isKode, f.isInt, false, false)
				t2, f2, p2, tn2 := countResult(result.Status)
				tp += t2
				fn += f2
				fp += p2
				tn += tn2
			}
		}
		// extra courses in pred only -> 6 fields FP each
		for _, predMK := range extract.KHS.MataKuliah {
			if !gtNos[predMK.No] {
				fp += 6
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

		// Field-level KRS courses: 8 fields per course (kode,nama,dosen,sks,kelas,jadwal.hari,waktu_mulai,waktu_selesai)
		predMap := make(map[int]entity.KRSMataKuliah, len(extract.KRS.MataKuliah))
		for _, c := range extract.KRS.MataKuliah {
			predMap[c.No] = c
		}
		gtNos := make(map[int]bool, len(gt.KRS.MataKuliah))
		for _, gtMK := range gt.KRS.MataKuliah {
			gtNos[gtMK.No] = true
			predMK, exists := predMap[gtMK.No]
			if !exists {
				fn += 8
				continue
			}
			fields := []struct {
				gtVal, predVal        string
				isKode, isInt, isTime bool
			}{
				{gtMK.Kode, predMK.Kode, true, false, false},
				{gtMK.Nama, predMK.Nama, false, false, false},
				{gtMK.Dosen, predMK.Dosen, false, false, false},
				{strconv.Itoa(gtMK.SKS), strconv.Itoa(predMK.SKS), false, true, false},
				{gtMK.Kelas, predMK.Kelas, false, false, false},
				{gtMK.Jadwal.Hari, predMK.Jadwal.Hari, false, false, false},
				{gtMK.Jadwal.WaktuMulai, predMK.Jadwal.WaktuMulai, false, false, true},
				{gtMK.Jadwal.WaktuSelesai, predMK.Jadwal.WaktuSelesai, false, false, true},
			}
			for _, f := range fields {
				result := CompareFields(f.gtVal, f.predVal, f.isKode, f.isInt, false, f.isTime)
				t2, f2, p2, tn2 := countResult(result.Status)
				tp += t2
				fn += f2
				fp += p2
				tn += tn2
			}
		}
		for _, predMK := range extract.KRS.MataKuliah {
			if !gtNos[predMK.No] {
				fp += 8
			}
		}
	}

	return computeMetrics(tp, fn, fp, tn), nil
}

// buildCompareRowsWithStore builds compare rows using the provided store.
func (s *EvalService) buildCompareRowsWithStore(st port.EvalStore, npm, docType, filename string) []entity.CompareRow {
	gtData, err := st.LoadGT(npm, docType, filename)
	if err != nil {
		return []entity.CompareRow{}
	}
	extractData, err := st.LoadExtract(npm, docType, filename)
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

// FastNameLookup does a partial JSON decode to extract only the "nama" field.
// Avoids full evaluateDoc() which reads both GT + extract and compares all fields.
// Exported for testing purposes.
func (s *EvalService) FastNameLookup(npm string) string {
	return s.fastNameLookupWithStore(s.store, npm)
}
