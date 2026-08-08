package extractor

import (
	"testing"

	"lonceng_unman_be/internal/domain/entity"
)

// makeWord creates a PDFWord with the given text and X position.
func makeWord(text string, x float64) PDFWord {
	return PDFWord{Text: text, X: x, Y: 0}
}

// makeRow creates a PDFRow with the given Y position and words.
func makeRow(y float64, words []PDFWord) PDFRow {
	return PDFRow{Y: y, Words: words}
}

// buildKHSRows builds a minimal set of PDFRows that simulate a KHS table
// layout parseKHSMataKuliah can process.
//
// Structure:
//   - Header row at Y=0: "No", "Kode", "Mata Kuliah", "SKS", "Nilai", "Mutu"
//   - N data rows with course info at specified Y positions
//   - Row before each data row: course name
//   - Row after each data row: dosen name
//
// X positions per column (must match what FindColumnPositions expects):
//
//	No=10, Kode=60, MataKuliah=170 (words close together so they group),
//	SKS=150, Nilai=280, Mutu=410
//
// Wait — we need "Kode" and "Mata Kuliah" to be separate X groups
// because FindColumnPositions expects column names to be in distinct X groups.
// "Mata" and "Kuliah" should share the same X group.
func buildKHSRows(courses []struct {
	no    int
	kode  string
	nama  string
	dosen string
	sks   int
	nilai string
	mutu  int
	y     float64
},
) []PDFRow {
	var rows []PDFRow

	// Header row at Y=0
	// "No" "Kode" "MataKuliah" "SKS" "Nilai" "Mutu"
	// Words placed so that "Mata" and "Kuliah" group together (same rounded X)
	// and all other columns are separate groups.
	headerWords := []PDFWord{
		{Text: "No", X: 10, Y: 0},
		{Text: "Kode", X: 60, Y: 0},
		{Text: "Mata", X: 120, Y: 0},     // groups with Kuliah at ~120.5
		{Text: "Kuliah", X: 120.5, Y: 0}, // rounds to same X as Mata
		{Text: "SKS", X: 175, Y: 0},
		{Text: "Nilai", X: 225, Y: 0},
		{Text: "Mutu", X: 275, Y: 0},
	}
	rows = append(rows, makeRow(0, headerWords))

	// For each course, add 3 rows: nama (before), data (at course y), dosen (after)
	for _, c := range courses {
		// Nama row (course name, above)
		namaY := c.y - 2.0
		namaWords := []PDFWord{
			{Text: c.nama, X: 10, Y: namaY},
		}
		rows = append(rows, makeRow(namaY, namaWords))

		// Data row (the actual course line)
		dataWords := []PDFWord{
			{Text: intToStr(c.no), X: 10, Y: c.y},
			{Text: c.kode, X: 60, Y: c.y},
			{Text: intToStr(c.sks), X: 175, Y: c.y},
			{Text: c.nilai, X: 225, Y: c.y},
			{Text: intToStr(c.mutu), X: 275, Y: c.y},
		}
		rows = append(rows, makeRow(c.y, dataWords))

		// Dosen row (lecturer name, below)
		dosenY := c.y + 2.0
		dosenWords := []PDFWord{
			{Text: c.dosen, X: 10, Y: dosenY},
		}
		rows = append(rows, makeRow(dosenY, dosenWords))
	}

	return rows
}

// intToStr converts an int to string without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToStr(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// TestParseKHSMataKuliah_DuplicateCourseCodes verifies that courses sharing
// the same Kode but different No values are BOTH extracted.
//
// This is the core regression test for the dedup fix:
//
//	Before fix: dedup key was Kode → second course with same Kode was skipped
//	After fix: dedup key is No → both courses extracted (different No values)
func TestParseKHSMataKuliah_DuplicateCourseCodes(t *testing.T) {
	courses := []struct {
		no    int
		kode  string
		nama  string
		dosen string
		sks   int
		nilai string
		mutu  int
		y     float64
	}{
		{no: 1, kode: "KK21250323", nama: "Kepemimpinan", dosen: "Dr. Ahmad", sks: 3, nilai: "A", mutu: 12, y: 10},
		{no: 2, kode: "KK21250323", nama: "Manajemen Proyek", dosen: "Dr. Budi", sks: 3, nilai: "B+", mutu: 10, y: 16},
	}

	rows := buildKHSRows(courses)
	lines := RowsToLines(rows)
	result := &entity.KHSExtraction{}

	parseKHSMataKuliah(rows, lines, result)

	if len(result.KHS.MataKuliah) != 2 {
		t.Fatalf("expected 2 courses (same Kode, different No), got %d: %+v",
			len(result.KHS.MataKuliah), result.KHS.MataKuliah)
	}

	// Verify both courses are present
	kodes := map[string]bool{}
	for _, mk := range result.KHS.MataKuliah {
		kodes[mk.Kode] = true
		if mk.Kode != "KK21250323" {
			t.Errorf("unexpected kode %q, want KK21250323", mk.Kode)
		}
	}
	if !kodes["KK21250323"] {
		t.Error("course with kode KK21250323 not found")
	}
}

// TestParseKHSMataKuliah_DuplicateNo verifies that courses with
// duplicate No values have the second one skipped.
func TestParseKHSMataKuliah_DuplicateNo(t *testing.T) {
	courses := []struct {
		no    int
		kode  string
		nama  string
		dosen string
		sks   int
		nilai string
		mutu  int
		y     float64
	}{
		{no: 1, kode: "KK21250323", nama: "Kepemimpinan", dosen: "Dr. Ahmad", sks: 3, nilai: "A", mutu: 12, y: 10},
		{no: 1, kode: "SI4030001", nama: "Manajemen Proyek", dosen: "Dr. Budi", sks: 3, nilai: "B+", mutu: 10, y: 16},
	}

	rows := buildKHSRows(courses)
	lines := RowsToLines(rows)
	result := &entity.KHSExtraction{}

	parseKHSMataKuliah(rows, lines, result)

	if len(result.KHS.MataKuliah) != 1 {
		t.Fatalf("expected 1 course (duplicate No=1 skipped), got %d: %+v",
			len(result.KHS.MataKuliah), result.KHS.MataKuliah)
	}

	if result.KHS.MataKuliah[0].No != 1 {
		t.Errorf("expected No=1, got No=%d", result.KHS.MataKuliah[0].No)
	}
	if result.KHS.MataKuliah[0].Kode != "KK21250323" {
		t.Errorf("expected kode KK21250323, got %q", result.KHS.MataKuliah[0].Kode)
	}
}

// TestParseKHSMataKuliah_UniqueCodes verifies that courses with unique
// codes and unique No values are all extracted (regression: existing behavior).
func TestParseKHSMataKuliah_UniqueCodes(t *testing.T) {
	courses := []struct {
		no    int
		kode  string
		nama  string
		dosen string
		sks   int
		nilai string
		mutu  int
		y     float64
	}{
		{no: 1, kode: "KK21250323", nama: "Kepemimpinan", dosen: "Dr. Ahmad", sks: 3, nilai: "A", mutu: 12, y: 10},
		{no: 2, kode: "SI4030001", nama: "Manajemen Proyek", dosen: "Dr. Budi", sks: 3, nilai: "B+", mutu: 10, y: 16},
	}

	rows := buildKHSRows(courses)
	lines := RowsToLines(rows)
	result := &entity.KHSExtraction{}

	parseKHSMataKuliah(rows, lines, result)

	if len(result.KHS.MataKuliah) != 2 {
		t.Fatalf("expected 2 courses (unique codes), got %d: %+v",
			len(result.KHS.MataKuliah), result.KHS.MataKuliah)
	}

	// Verify both are present with correct codes
	codes := map[string]bool{}
	for _, mk := range result.KHS.MataKuliah {
		codes[mk.Kode] = true
	}
	if !codes["KK21250323"] {
		t.Error("course KK21250323 not found")
	}
	if !codes["SI4030001"] {
		t.Error("course SI4030001 not found")
	}
}

// TestParseKHSMataKuliah_NoCourses verifies that an empty table returns
// no courses and does not panic.
func TestParseKHSMataKuliah_NoCourses(t *testing.T) {
	// Only a header row, no data
	headerWords := []PDFWord{
		{Text: "No", X: 10, Y: 0},
		{Text: "Kode", X: 60, Y: 0},
		{Text: "Mata", X: 120, Y: 0},
		{Text: "Kuliah", X: 120.5, Y: 0},
		{Text: "SKS", X: 175, Y: 0},
		{Text: "Nilai", X: 225, Y: 0},
		{Text: "Mutu", X: 275, Y: 0},
	}
	rows := []PDFRow{makeRow(0, headerWords)}
	lines := RowsToLines(rows)
	result := &entity.KHSExtraction{}

	parseKHSMataKuliah(rows, lines, result)

	if len(result.KHS.MataKuliah) != 0 {
		t.Fatalf("expected 0 courses for empty table, got %d", len(result.KHS.MataKuliah))
	}
}

// TestParseKHSMataKuliah_CourseDetails verifies that nama and dosen
// are correctly extracted from surrounding rows.
func TestParseKHSMataKuliah_CourseDetails(t *testing.T) {
	courses := []struct {
		no    int
		kode  string
		nama  string
		dosen string
		sks   int
		nilai string
		mutu  int
		y     float64
	}{
		{no: 1, kode: "KK21250323", nama: "Kepemimpinan", dosen: "Dr. Ahmad, S.Kom", sks: 3, nilai: "A", mutu: 12, y: 10},
	}

	rows := buildKHSRows(courses)
	lines := RowsToLines(rows)
	result := &entity.KHSExtraction{}

	parseKHSMataKuliah(rows, lines, result)

	if len(result.KHS.MataKuliah) != 1 {
		t.Fatalf("expected 1 course, got %d", len(result.KHS.MataKuliah))
	}

	mk := result.KHS.MataKuliah[0]
	if mk.Nama != "Kepemimpinan" {
		t.Errorf("expected nama 'Kepemimpinan', got %q", mk.Nama)
	}
	if mk.Dosen != "Dr. Ahmad, S.Kom" {
		t.Errorf("expected dosen 'Dr. Ahmad, S.Kom', got %q", mk.Dosen)
	}
	if mk.SKS != 3 {
		t.Errorf("expected SKS=3, got %d", mk.SKS)
	}
	if mk.Nilai != "A" {
		t.Errorf("expected Nilai='A', got %q", mk.Nilai)
	}
	if mk.Mutu != 12 {
		t.Errorf("expected Mutu=12, got %d", mk.Mutu)
	}
}
