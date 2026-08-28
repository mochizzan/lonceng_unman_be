package service

import (
	"fmt"
	"strconv"

	"lonceng_unman_be/internal/domain/entity"
)

// FieldResult holds the comparison result for a single field instance.
type FieldResult struct {
	Status string // "tp", "fp", "fn", "tn", "wrong"
}

// CompareFields compares two field values and returns the confusion status.
func CompareFields(gtVal, predVal string, isKode bool, isInt bool, isIPK bool, isTime bool) FieldResult {
	gtEmpty := isEmpty(gtVal, isInt || isIPK)
	predEmpty := isEmpty(predVal, isInt || isIPK)

	if gtEmpty && predEmpty {
		return FieldResult{Status: "tn"}
	}
	if gtEmpty && !predEmpty {
		return FieldResult{Status: "fp"}
	}
	if !gtEmpty && predEmpty {
		return FieldResult{Status: "fn"}
	}

	// Both filled - compare
	var equal bool
	if isIPK {
		equal = ipkEqual(parseFloat(gtVal), parseFloat(predVal))
	} else if isInt {
		equal = parseInt(gtVal) == parseInt(predVal)
	} else if isTime {
		equal = normalizeTime(gtVal) == normalizeTime(predVal)
	} else if isKode {
		equal = normalizeKode(gtVal) == normalizeKode(predVal)
	} else {
		equal = normalizeString(gtVal) == normalizeString(predVal)
	}

	if equal {
		return FieldResult{Status: "tp"}
	}
	return FieldResult{Status: "wrong"}
}

// CourseResult holds course-level matching result.
type CourseResult struct {
	No       int
	Matched  bool
	Fields   []string // mismatched field names
	IsGTOnly bool     // true if course is in GT only (FN)
}

// MatchCourses matches courses by `no` and compares fields.
func MatchCourses(gtCourses, predCourses []entity.KRSMataKuliah) []CourseResult {
	// Build pred map by no
	predMap := make(map[int]entity.KRSMataKuliah)
	for _, c := range predCourses {
		predMap[c.No] = c
	}

	var results []CourseResult
	for _, gt := range gtCourses {
		pred, exists := predMap[gt.No]
		if !exists {
			results = append(results, CourseResult{No: gt.No, Matched: false, IsGTOnly: true})
			continue
		}
		// Compare fields
		var mismatches []string
		if result := CompareFields(gt.Kode, pred.Kode, true, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "kode")
		}
		if result := CompareFields(gt.Nama, pred.Nama, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "nama")
		}
		if result := CompareFields(gt.Dosen, pred.Dosen, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "dosen")
		}
		if result := CompareFields(strconv.Itoa(gt.SKS), strconv.Itoa(pred.SKS), false, true, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "sks")
		}
		if result := CompareFields(gt.Kelas, pred.Kelas, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "kelas")
		}
		// KRS schedule fields
		if result := CompareFields(gt.Jadwal.Hari, pred.Jadwal.Hari, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "jadwal.hari")
		}
		if result := CompareFields(gt.Jadwal.WaktuMulai, pred.Jadwal.WaktuMulai, false, false, false, true); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "jadwal.waktu_mulai")
		}
		if result := CompareFields(gt.Jadwal.WaktuSelesai, pred.Jadwal.WaktuSelesai, false, false, false, true); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "jadwal.waktu_selesai")
		}

		results = append(results, CourseResult{
			No:      gt.No,
			Matched: len(mismatches) == 0,
			Fields:  mismatches,
		})
	}

	// Add FP courses (in pred but not in gt)
	gtNos := make(map[int]bool)
	for _, c := range gtCourses {
		gtNos[c.No] = true
	}
	for _, pred := range predCourses {
		if !gtNos[pred.No] {
			results = append(results, CourseResult{No: pred.No, Matched: false})
		}
	}

	return results
}

// BuildCompareRows builds per-field compare rows for the detail page.
func BuildCompareRows(gt, extract *entity.KRSExtraction) []entity.CompareRow {
	var rows []entity.CompareRow

	// Header fields
	rows = append(rows, entity.CompareRow{
		Field:  "mahasiswa.nama",
		GT:     gt.KRS.Mahasiswa.Nama,
		Pred:   extract.KRS.Mahasiswa.Nama,
		Status: compareStatus(gt.KRS.Mahasiswa.Nama, extract.KRS.Mahasiswa.Nama, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "mahasiswa.program_studi",
		GT:     gt.KRS.Mahasiswa.ProgramStudi,
		Pred:   extract.KRS.Mahasiswa.ProgramStudi,
		Status: compareStatus(gt.KRS.Mahasiswa.ProgramStudi, extract.KRS.Mahasiswa.ProgramStudi, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "periode.semester",
		GT:     gt.KRS.Periode.Semester,
		Pred:   extract.KRS.Periode.Semester,
		Status: compareStatus(gt.KRS.Periode.Semester, extract.KRS.Periode.Semester, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "periode.tahun_ajaran.awal",
		GT:     gt.KRS.Periode.TahunAjaran.Awal,
		Pred:   extract.KRS.Periode.TahunAjaran.Awal,
		Status: compareStatus(gt.KRS.Periode.TahunAjaran.Awal, extract.KRS.Periode.TahunAjaran.Awal, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "periode.tahun_ajaran.akhir",
		GT:     gt.KRS.Periode.TahunAjaran.Akhir,
		Pred:   extract.KRS.Periode.TahunAjaran.Akhir,
		Status: compareStatus(gt.KRS.Periode.TahunAjaran.Akhir, extract.KRS.Periode.TahunAjaran.Akhir, false, false, false, false),
	})

	// Course fields - match by no
	predMap := make(map[int]entity.KRSMataKuliah)
	for _, c := range extract.KRS.MataKuliah {
		predMap[c.No] = c
	}

	for _, gtMK := range gt.KRS.MataKuliah {
		predMK, exists := predMap[gtMK.No]
		if !exists {
			rows = append(
				rows,
				entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].kode", GT: gtMK.Kode, Pred: "", Status: "missing"},
				entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].nama", GT: gtMK.Nama, Pred: "", Status: "missing"},
			)
			continue
		}
		rows = append(
			rows,
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].kode", GT: gtMK.Kode, Pred: predMK.Kode, Status: compareStatus(gtMK.Kode, predMK.Kode, true, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].nama", GT: gtMK.Nama, Pred: predMK.Nama, Status: compareStatus(gtMK.Nama, predMK.Nama, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].dosen", GT: gtMK.Dosen, Pred: predMK.Dosen, Status: compareStatus(gtMK.Dosen, predMK.Dosen, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].sks", GT: strconv.Itoa(gtMK.SKS), Pred: strconv.Itoa(predMK.SKS), Status: compareStatus(strconv.Itoa(gtMK.SKS), strconv.Itoa(predMK.SKS), false, true, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].kelas", GT: gtMK.Kelas, Pred: predMK.Kelas, Status: compareStatus(gtMK.Kelas, predMK.Kelas, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].jadwal.hari", GT: gtMK.Jadwal.Hari, Pred: predMK.Jadwal.Hari, Status: compareStatus(gtMK.Jadwal.Hari, predMK.Jadwal.Hari, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].jadwal.waktu_mulai", GT: gtMK.Jadwal.WaktuMulai, Pred: predMK.Jadwal.WaktuMulai, Status: compareStatus(gtMK.Jadwal.WaktuMulai, predMK.Jadwal.WaktuMulai, false, false, false, true)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].jadwal.waktu_selesai", GT: gtMK.Jadwal.WaktuSelesai, Pred: predMK.Jadwal.WaktuSelesai, Status: compareStatus(gtMK.Jadwal.WaktuSelesai, predMK.Jadwal.WaktuSelesai, false, false, false, true)},
		)
	}

	return rows
}

// compareStatus returns the status string for a field comparison.
func compareStatus(gtVal, predVal string, isKode, isInt, isIPK, isTime bool) string {
	result := CompareFields(gtVal, predVal, isKode, isInt, isIPK, isTime)
	switch result.Status {
	case "tp":
		return "correct"
	case "tn":
		return "correct"
	case "wrong":
		return "wrong"
	case "fn":
		return "missing"
	case "fp":
		return "extra"
	}
	return "correct"
}

// MatchKHSCourses matches KHS courses by `no` and compares fields.
func MatchKHSCourses(gtCourses, predCourses []entity.KHSMataKuliah) []CourseResult {
	// Build pred map by no
	predMap := make(map[int]entity.KHSMataKuliah)
	for _, c := range predCourses {
		predMap[c.No] = c
	}

	var results []CourseResult
	for _, gt := range gtCourses {
		pred, exists := predMap[gt.No]
		if !exists {
			results = append(results, CourseResult{No: gt.No, Matched: false, IsGTOnly: true})
			continue
		}
		// Compare fields
		var mismatches []string
		if result := CompareFields(gt.Kode, pred.Kode, true, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "kode")
		}
		if result := CompareFields(gt.Nama, pred.Nama, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "nama")
		}
		if result := CompareFields(gt.Dosen, pred.Dosen, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "dosen")
		}
		if result := CompareFields(strconv.Itoa(gt.SKS), strconv.Itoa(pred.SKS), false, true, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "sks")
		}
		if result := CompareFields(gt.Nilai, pred.Nilai, false, false, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "nilai")
		}
		if result := CompareFields(strconv.Itoa(gt.Mutu), strconv.Itoa(pred.Mutu), false, true, false, false); result.Status != "tp" && result.Status != "tn" {
			mismatches = append(mismatches, "mutu")
		}

		results = append(results, CourseResult{
			No:      gt.No,
			Matched: len(mismatches) == 0,
			Fields:  mismatches,
		})
	}

	// Add FP courses (in pred but not in gt)
	gtNos := make(map[int]bool)
	for _, c := range gtCourses {
		gtNos[c.No] = true
	}
	for _, pred := range predCourses {
		if !gtNos[pred.No] {
			results = append(results, CourseResult{No: pred.No, Matched: false})
		}
	}

	return results
}

// BuildCompareRowsKHS builds per-field compare rows for KHS detail page.
func BuildCompareRowsKHS(gt, extract *entity.KHSExtraction) []entity.CompareRow {
	var rows []entity.CompareRow

	// Header fields
	rows = append(rows, entity.CompareRow{
		Field:  "mahasiswa.nama",
		GT:     gt.KHS.Mahasiswa.Nama,
		Pred:   extract.KHS.Mahasiswa.Nama,
		Status: compareStatus(gt.KHS.Mahasiswa.Nama, extract.KHS.Mahasiswa.Nama, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "mahasiswa.program_studi",
		GT:     gt.KHS.Mahasiswa.ProgramStudi,
		Pred:   extract.KHS.Mahasiswa.ProgramStudi,
		Status: compareStatus(gt.KHS.Mahasiswa.ProgramStudi, extract.KHS.Mahasiswa.ProgramStudi, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "periode.semester",
		GT:     gt.KHS.Periode.Semester,
		Pred:   extract.KHS.Periode.Semester,
		Status: compareStatus(gt.KHS.Periode.Semester, extract.KHS.Periode.Semester, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "periode.tahun_ajaran.awal",
		GT:     gt.KHS.Periode.TahunAjaran.Awal,
		Pred:   extract.KHS.Periode.TahunAjaran.Awal,
		Status: compareStatus(gt.KHS.Periode.TahunAjaran.Awal, extract.KHS.Periode.TahunAjaran.Awal, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "periode.tahun_ajaran.akhir",
		GT:     gt.KHS.Periode.TahunAjaran.Akhir,
		Pred:   extract.KHS.Periode.TahunAjaran.Akhir,
		Status: compareStatus(gt.KHS.Periode.TahunAjaran.Akhir, extract.KHS.Periode.TahunAjaran.Akhir, false, false, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "rekapitulasi.total_sks",
		GT:     strconv.Itoa(gt.KHS.Rekapitulasi.TotalSKS),
		Pred:   strconv.Itoa(extract.KHS.Rekapitulasi.TotalSKS),
		Status: compareStatus(strconv.Itoa(gt.KHS.Rekapitulasi.TotalSKS), strconv.Itoa(extract.KHS.Rekapitulasi.TotalSKS), false, true, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "rekapitulasi.total_mutu",
		GT:     strconv.Itoa(gt.KHS.Rekapitulasi.TotalMutu),
		Pred:   strconv.Itoa(extract.KHS.Rekapitulasi.TotalMutu),
		Status: compareStatus(strconv.Itoa(gt.KHS.Rekapitulasi.TotalMutu), strconv.Itoa(extract.KHS.Rekapitulasi.TotalMutu), false, true, false, false),
	})
	rows = append(rows, entity.CompareRow{
		Field:  "rekapitulasi.ipk",
		GT:     fmt.Sprintf("%.2f", gt.KHS.Rekapitulasi.IPK),
		Pred:   fmt.Sprintf("%.2f", extract.KHS.Rekapitulasi.IPK),
		Status: compareStatus(fmt.Sprintf("%.2f", gt.KHS.Rekapitulasi.IPK), fmt.Sprintf("%.2f", extract.KHS.Rekapitulasi.IPK), false, false, true, false),
	})

	// Course fields - match by no
	predMap := make(map[int]entity.KHSMataKuliah)
	for _, c := range extract.KHS.MataKuliah {
		predMap[c.No] = c
	}

	for _, gtMK := range gt.KHS.MataKuliah {
		predMK, exists := predMap[gtMK.No]
		if !exists {
			rows = append(
				rows,
				entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].kode", GT: gtMK.Kode, Pred: "", Status: "missing"},
				entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].nama", GT: gtMK.Nama, Pred: "", Status: "missing"},
			)
			continue
		}
		rows = append(
			rows,
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].kode", GT: gtMK.Kode, Pred: predMK.Kode, Status: compareStatus(gtMK.Kode, predMK.Kode, true, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].nama", GT: gtMK.Nama, Pred: predMK.Nama, Status: compareStatus(gtMK.Nama, predMK.Nama, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].dosen", GT: gtMK.Dosen, Pred: predMK.Dosen, Status: compareStatus(gtMK.Dosen, predMK.Dosen, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].sks", GT: strconv.Itoa(gtMK.SKS), Pred: strconv.Itoa(predMK.SKS), Status: compareStatus(strconv.Itoa(gtMK.SKS), strconv.Itoa(predMK.SKS), false, true, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].nilai", GT: gtMK.Nilai, Pred: predMK.Nilai, Status: compareStatus(gtMK.Nilai, predMK.Nilai, false, false, false, false)},
			entity.CompareRow{Field: "mata_kuliah[" + strconv.Itoa(gtMK.No) + "].mutu", GT: strconv.Itoa(gtMK.Mutu), Pred: strconv.Itoa(predMK.Mutu), Status: compareStatus(strconv.Itoa(gtMK.Mutu), strconv.Itoa(predMK.Mutu), false, true, false, false)},
		)
	}

	return rows
}
