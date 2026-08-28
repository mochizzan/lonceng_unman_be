package service

import (
	"testing"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/domain/entity"
)

func TestMatchCourses_TwoSI40306_DifferentNo(t *testing.T) {
	gt := []entity.KRSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8A", Dosen: "Dosen A", Jadwal: entity.KRSJadwal{Hari: "Senin", WaktuMulai: "08:00", WaktuSelesai: "10:00"}},
		{No: 2, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8B", Dosen: "Dosen B", Jadwal: entity.KRSJadwal{Hari: "Selasa", WaktuMulai: "10:00", WaktuSelesai: "12:00"}},
	}
	pred := []entity.KRSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8A", Dosen: "Dosen A", Jadwal: entity.KRSJadwal{Hari: "Senin", WaktuMulai: "08:00", WaktuSelesai: "10:00"}},
		{No: 2, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8B", Dosen: "Dosen B", Jadwal: entity.KRSJadwal{Hari: "Selasa", WaktuMulai: "10:00", WaktuSelesai: "12:00"}},
	}

	results := service.MatchCourses(gt, pred)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Matched {
			t.Errorf("expected no %d to be matched", r.No)
		}
	}
}

func TestMatchCourses_SameNo_DifferentKelas(t *testing.T) {
	gt := []entity.KRSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8A", Dosen: "Dosen A", Jadwal: entity.KRSJadwal{Hari: "Senin", WaktuMulai: "08:00", WaktuSelesai: "10:00"}},
	}
	pred := []entity.KRSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8B", Dosen: "Dosen A", Jadwal: entity.KRSJadwal{Hari: "Senin", WaktuMulai: "08:00", WaktuSelesai: "10:00"}},
	}

	results := service.MatchCourses(gt, pred)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Matched {
		t.Errorf("expected no 1 to be matched (kelas differs)")
	}
}

func TestMatchCourses_NoOnlyInGT(t *testing.T) {
	gt := []entity.KRSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Kelas: "SI-8A", Dosen: "Dosen A", Jadwal: entity.KRSJadwal{Hari: "Senin", WaktuMulai: "08:00", WaktuSelesai: "10:00"}},
	}
	pred := []entity.KRSMataKuliah{}

	results := service.MatchCourses(gt, pred)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Matched {
		t.Errorf("expected no 1 to be unmatched")
	}
}

func TestCompareFields_Substitution(t *testing.T) {
	result := service.CompareFields("Basis Data", "Basisdata", false, false, false, false)
	if result.Status != "wrong" {
		t.Errorf("expected wrong, got %s", result.Status)
	}
}

func TestCompareFields_BlankBlank(t *testing.T) {
	result := service.CompareFields("", "", false, false, false, false)
	if result.Status != "tn" {
		t.Errorf("expected tn, got %s", result.Status)
	}
}

func TestCompareFields_IPK(t *testing.T) {
	result := service.CompareFields("3.50", "3.501", false, false, true, false)
	if result.Status != "tp" {
		t.Errorf("expected tp, got %s", result.Status)
	}
}

func TestCompareFields_Time(t *testing.T) {
	result := service.CompareFields("08:00:00", "08:00", false, false, false, true)
	if result.Status != "tp" {
		t.Errorf("expected tp, got %s", result.Status)
	}
}

func TestMatchKHSCourses_TwoIdenticalCourses_AllMatched(t *testing.T) {
	gt := []entity.KHSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
		{No: 2, Kode: "SI40307", Nama: "Pemrograman", SKS: 4, Nilai: "B+", Mutu: 14, Dosen: "Dosen B"},
	}
	pred := []entity.KHSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
		{No: 2, Kode: "SI40307", Nama: "Pemrograman", SKS: 4, Nilai: "B+", Mutu: 14, Dosen: "Dosen B"},
	}

	results := service.MatchKHSCourses(gt, pred)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Matched {
			t.Errorf("expected no %d to be matched", r.No)
		}
	}
}

func TestMatchKHSCourses_DifferentNilai_NotMatched(t *testing.T) {
	gt := []entity.KHSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
	}
	pred := []entity.KHSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "B+", Mutu: 10, Dosen: "Dosen A"},
	}

	results := service.MatchKHSCourses(gt, pred)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Matched {
		t.Errorf("expected no 1 to be unmatched (nilai differs)")
	}
}

func TestMatchKHSCourses_NoOnlyInGT_CountsAsFN(t *testing.T) {
	gt := []entity.KHSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
	}
	pred := []entity.KHSMataKuliah{}

	results := service.MatchKHSCourses(gt, pred)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Matched {
		t.Errorf("expected no 1 to be unmatched")
	}
	if !results[0].IsGTOnly {
		t.Errorf("expected course to be marked as GT-only (FN)")
	}
}

func TestMatchKHSCourses_NoOnlyInPred_CountsAsFP(t *testing.T) {
	gt := []entity.KHSMataKuliah{}
	pred := []entity.KHSMataKuliah{
		{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
	}

	results := service.MatchKHSCourses(gt, pred)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Matched {
		t.Errorf("expected no 1 to be unmatched")
	}
	if results[0].IsGTOnly {
		t.Errorf("expected course to be marked as pred-only (FP)")
	}
}

func TestBuildCompareRowsKHS_ProducesCorrectStatus(t *testing.T) {
	gt := &entity.KHSExtraction{
		KHS: struct {
			Mahasiswa    entity.Mahasiswa       `json:"mahasiswa"`
			Periode      entity.Periode         `json:"periode"`
			MataKuliah   []entity.KHSMataKuliah `json:"mata_kuliah"`
			Rekapitulasi entity.KHSRekapitulasi `json:"rekapitulasi"`
			Penerbitan   entity.Penerbitan      `json:"penerbitan"`
			Persetujuan  entity.KHSPersetujuan  `json:"persetujuan"`
		}{
			Mahasiswa: entity.Mahasiswa{Nama: "John Doe", NPM: "2211700006", ProgramStudi: "Sistem Informasi"},
			Periode: entity.Periode{
				TahunAjaran: entity.TahunAjaran{Awal: "2022", Akhir: "2023"},
				Semester:    "GANJIL",
			},
			MataKuliah: []entity.KHSMataKuliah{
				{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
			},
			Rekapitulasi: entity.KHSRekapitulasi{TotalSKS: 24, TotalMutu: 90, IPK: 3.75},
		},
	}
	extract := &entity.KHSExtraction{
		KHS: struct {
			Mahasiswa    entity.Mahasiswa       `json:"mahasiswa"`
			Periode      entity.Periode         `json:"periode"`
			MataKuliah   []entity.KHSMataKuliah `json:"mata_kuliah"`
			Rekapitulasi entity.KHSRekapitulasi `json:"rekapitulasi"`
			Penerbitan   entity.Penerbitan      `json:"penerbitan"`
			Persetujuan  entity.KHSPersetujuan  `json:"persetujuan"`
		}{
			Mahasiswa: entity.Mahasiswa{Nama: "John Doe", NPM: "2211700006", ProgramStudi: "Sistem Informasi"},
			Periode: entity.Periode{
				TahunAjaran: entity.TahunAjaran{Awal: "2022", Akhir: "2023"},
				Semester:    "GANJIL",
			},
			MataKuliah: []entity.KHSMataKuliah{
				{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
			},
			Rekapitulasi: entity.KHSRekapitulasi{TotalSKS: 24, TotalMutu: 90, IPK: 3.75},
		},
	}

	rows := service.BuildCompareRowsKHS(gt, extract)
	if len(rows) == 0 {
		t.Fatalf("expected compare rows, got empty")
	}

	for _, row := range rows {
		if row.Status != "correct" {
			t.Errorf("expected 'correct' status for field %s, got %s", row.Field, row.Status)
		}
	}
}

func TestBuildCompareRowsKHS_WrongNilai_ProducesWrongStatus(t *testing.T) {
	gt := &entity.KHSExtraction{
		KHS: struct {
			Mahasiswa    entity.Mahasiswa       `json:"mahasiswa"`
			Periode      entity.Periode         `json:"periode"`
			MataKuliah   []entity.KHSMataKuliah `json:"mata_kuliah"`
			Rekapitulasi entity.KHSRekapitulasi `json:"rekapitulasi"`
			Penerbitan   entity.Penerbitan      `json:"penerbitan"`
			Persetujuan  entity.KHSPersetujuan  `json:"persetujuan"`
		}{
			Mahasiswa: entity.Mahasiswa{Nama: "John Doe", NPM: "2211700006", ProgramStudi: "Sistem Informasi"},
			Periode: entity.Periode{
				TahunAjaran: entity.TahunAjaran{Awal: "2022", Akhir: "2023"},
				Semester:    "GANJIL",
			},
			MataKuliah: []entity.KHSMataKuliah{
				{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "A", Mutu: 12, Dosen: "Dosen A"},
			},
			Rekapitulasi: entity.KHSRekapitulasi{TotalSKS: 24, TotalMutu: 90, IPK: 3.75},
		},
	}
	extract := &entity.KHSExtraction{
		KHS: struct {
			Mahasiswa    entity.Mahasiswa       `json:"mahasiswa"`
			Periode      entity.Periode         `json:"periode"`
			MataKuliah   []entity.KHSMataKuliah `json:"mata_kuliah"`
			Rekapitulasi entity.KHSRekapitulasi `json:"rekapitulasi"`
			Penerbitan   entity.Penerbitan      `json:"penerbitan"`
			Persetujuan  entity.KHSPersetujuan  `json:"persetujuan"`
		}{
			Mahasiswa: entity.Mahasiswa{Nama: "John Doe", NPM: "2211700006", ProgramStudi: "Sistem Informasi"},
			Periode: entity.Periode{
				TahunAjaran: entity.TahunAjaran{Awal: "2022", Akhir: "2023"},
				Semester:    "GANJIL",
			},
			MataKuliah: []entity.KHSMataKuliah{
				{No: 1, Kode: "SI40306", Nama: "Basis Data", SKS: 3, Nilai: "B+", Mutu: 10, Dosen: "Dosen A"},
			},
			Rekapitulasi: entity.KHSRekapitulasi{TotalSKS: 24, TotalMutu: 90, IPK: 3.75},
		},
	}

	rows := service.BuildCompareRowsKHS(gt, extract)

	foundWrong := false
	for _, row := range rows {
		if row.Field == "mata_kuliah[1].nilai" || row.Field == "mata_kuliah[1].mutu" {
			if row.Status != "wrong" {
				t.Errorf("expected 'wrong' status for field %s, got %s", row.Field, row.Status)
			}
			foundWrong = true
		}
	}
	if !foundWrong {
		t.Errorf("expected to find wrong status for nilai/mutu fields")
	}
}
