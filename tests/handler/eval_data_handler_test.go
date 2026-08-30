package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// mockExtractionService implements extractionServiceInterface for testing.
type mockExtractionService struct {
	krsFunc func(npm string) ([]byte, error)
	khsFunc func(npm string, tahunAjaran string, semester string) ([]byte, error)
}

func (m *mockExtractionService) GetKRSExtraction(npm string) ([]byte, error) {
	if m.krsFunc != nil {
		return m.krsFunc(npm)
	}
	return nil, errors.New("not implemented")
}

func (m *mockExtractionService) GetKHSExtraction(npm string, tahunAjaran string, semester string) ([]byte, error) {
	if m.khsFunc != nil {
		return m.khsFunc(npm, tahunAjaran, semester)
	}
	return nil, errors.New("not implemented")
}

// TestEvalDataHandler_KRS_NoNPM verifies the KRS page renders without error when no NPM is provided.
func TestEvalDataHandler_KRS_NoNPM(t *testing.T) {
	svc := &mockExtractionService{}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/krs", h.KRSPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/krs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Data KRS") {
		t.Error("body should contain 'Data KRS' title")
	}
	if strings.Contains(body.String(), "Data Tidak Tersedia") {
		t.Error("body should NOT contain error when no NPM provided")
	}
}

// TestEvalDataHandler_KRS_WithData verifies the KRS page renders data when available.
func TestEvalDataHandler_KRS_WithData(t *testing.T) {
	svc := &mockExtractionService{
		krsFunc: func(npm string) ([]byte, error) {
			data := `{"krs":{"mahasiswa":{"nama":"Test Mahasiswa","npm":"` + npm + `","program_studi":"Teknik Informatika"},"periode":{"semester":"8","tahun_ajaran":{"awal":"2024","akhir":"2025"}},"mata_kuliah":[{"no":1,"kode":"IF101","nama":"Algoritma","sks":3,"kelas":"A"}]}}`
			return []byte(data), nil
		},
	}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/krs", h.KRSPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/krs?npm=2211700006", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Test Mahasiswa") {
		t.Errorf("body should contain student name; got: %s", body.String())
	}
	if !strings.Contains(body.String(), "Algoritma") {
		t.Errorf("body should contain course name; got: %s", body.String())
	}
}

// TestEvalDataHandler_KRS_NotFound verifies graceful error when data is not found.
func TestEvalDataHandler_KRS_NotFound(t *testing.T) {
	svc := &mockExtractionService{
		krsFunc: func(npm string) ([]byte, error) {
			return nil, errors.New("no krs extraction for npm " + npm)
		},
	}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/krs", h.KRSPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/krs?npm=9999999999", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Data Tidak Tersedia") {
		t.Errorf("body should contain 'Data Tidak Tersedia' error message; got: %s", body.String())
	}
	if strings.Contains(body.String(), "no krs extraction") {
		t.Error("body should NOT expose raw error to user")
	}
}

// TestEvalDataHandler_KHS_NoNPM verifies the KHS page renders without error when no params provided.
func TestEvalDataHandler_KHS_NoNPM(t *testing.T) {
	svc := &mockExtractionService{}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/khs", h.KHSPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/khs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Data KHS") {
		t.Error("body should contain 'Data KHS' title")
	}
}

// TestEvalDataHandler_KHS_WithData verifies the KHS page renders data when available.
func TestEvalDataHandler_KHS_WithData(t *testing.T) {
	svc := &mockExtractionService{
		khsFunc: func(npm string, tahunAjaran string, semester string) ([]byte, error) {
			data := `{"khs":{"mahasiswa":{"nama":"Test Mahasiswa KHS","npm":"` + npm + `","program_studi":"Sistem Informasi"},"periode":{"semester":"` + semester + `","tahun_ajaran":{"awal":"2024","akhir":"2025"}},"mata_kuliah":[{"no":1,"kode":"IF201","nama":"Basis Data","sks":3,"nilai":"A","mutu":4}],"rekapitulasi":{"total_sks":3,"total_mutu":12,"ipk":4.00}}}`
			return []byte(data), nil
		},
	}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/khs", h.KHSPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/khs?npm=2211700006&tahun_ajaran=2024/2025&semester=GENAP", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Test Mahasiswa KHS") {
		t.Errorf("body should contain student name; got: %s", body.String())
	}
	if !strings.Contains(body.String(), "Basis Data") {
		t.Errorf("body should contain course name; got: %s", body.String())
	}
	if !strings.Contains(body.String(), "4") {
		t.Errorf("body should contain IPK; got: %s", body.String())
	}
}

// TestEvalDataHandler_KHS_NotFound verifies graceful error when KHS data is not found.
func TestEvalDataHandler_KHS_NotFound(t *testing.T) {
	svc := &mockExtractionService{
		khsFunc: func(npm string, tahunAjaran string, semester string) ([]byte, error) {
			return nil, errors.New("no khs extraction for npm " + npm)
		},
	}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/khs", h.KHSPage)

	req := httptest.NewRequest(http.MethodGet, "/eval/khs?npm=9999999999&tahun_ajaran=2024/2025&semester=GENAP", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Data Tidak Tersedia") {
		t.Errorf("body should contain 'Data Tidak Tersedia' error message; got: %s", body.String())
	}
	if strings.Contains(body.String(), "no khs extraction") {
		t.Error("body should NOT expose raw error to user")
	}
}

// TestEvalDataHandler_KHS_PartialParams verifies error when only some params provided.
func TestEvalDataHandler_KHS_PartialParams(t *testing.T) {
	svc := &mockExtractionService{}
	h, err := handler.NewEvalDataHandler(svc)
	if err != nil {
		t.Fatalf("NewEvalDataHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/khs", h.KHSPage)

	// Only NPM, missing tahun and semester
	req := httptest.NewRequest(http.MethodGet, "/eval/khs?npm=2211700006", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Data Tidak Tersedia") {
		t.Errorf("body should contain error for partial params; got: %s", body.String())
	}
}
