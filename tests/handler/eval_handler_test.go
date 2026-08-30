package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// mockEvalService implements evalServiceInterface for testing.
type mockEvalService struct {
	indexFunc       func() (entity.UnifiedEval, error)
	studentFunc     func(npm string) (entity.StudentEval, error)
	loadGTFunc      func(npm, docType, filename string) ([]byte, error)
	studentListFunc func() ([]entity.StudentEntry, error)
}

func (m *mockEvalService) Index() (entity.UnifiedEval, error) {
	if m.indexFunc != nil {
		return m.indexFunc()
	}
	return entity.UnifiedEval{}, errors.New("not implemented")
}

func (m *mockEvalService) Student(npm string) (entity.StudentEval, error) {
	if m.studentFunc != nil {
		return m.studentFunc(npm)
	}
	return entity.StudentEval{}, errors.New("not implemented")
}

func (m *mockEvalService) LoadGT(npm, docType, filename string) ([]byte, error) {
	if m.loadGTFunc != nil {
		return m.loadGTFunc(npm, docType, filename)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEvalService) StudentList() ([]entity.StudentEntry, error) {
	if m.studentListFunc != nil {
		return m.studentListFunc()
	}
	return nil, errors.New("not implemented")
}

// TestStudentPage_BasicPagination verifies basic pagination works.
func TestStudentPage_BasicPagination(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			entries := make([]entity.StudentEntry, 0, 60)
			for i := 1; i <= 60; i++ {
				entries = append(entries, entity.StudentEntry{
					NPM:  "2020100" + string(rune('0'+i%10)),
					Name: "Mahasiswa " + string(rune('A'+i%26)),
				})
			}
			return entries, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// Page 1, default per_page=25
	req := httptest.NewRequest(http.MethodGet, "/eval/student", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	if !strings.Contains(body.String(), "Data Mahasiswa") {
		t.Error("body should contain 'Data Mahasiswa' title")
	}
}

// TestStudentPage_SearchByNPM verifies search filters students by NPM.
func TestStudentPage_SearchByNPM(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			return []entity.StudentEntry{
				{NPM: "2020100001", Name: "Mochamad Izzan"},
				{NPM: "2020100002", Name: "Budi Santoso"},
				{NPM: "2021200001", Name: "Izzan Firdaus"},
				{NPM: "2021200002", Name: "Andi Wijaya"},
			}, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// Search for "20201" should return 2 students: 2020100001 and 2020100002
	req := httptest.NewRequest(http.MethodGet, "/eval/student?q=20201", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)

	// Should contain matching NPMs
	if !strings.Contains(body.String(), "2020100001") {
		t.Error("body should contain NPM 2020100001")
	}
	if !strings.Contains(body.String(), "2020100002") {
		t.Error("body should contain NPM 2020100002")
	}
	// Should NOT contain non-matching NPMs
	if strings.Contains(body.String(), "2021200001") {
		t.Error("body should NOT contain NPM 2021200001")
	}
	if strings.Contains(body.String(), "2021200002") {
		t.Error("body should NOT contain NPM 2021200002")
	}
}

// TestStudentPage_SearchByName verifies search filters students by name (case-insensitive).
func TestStudentPage_SearchByName(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			return []entity.StudentEntry{
				{NPM: "2020100001", Name: "Mochamad Izzan Firasyansyah"},
				{NPM: "2020100002", Name: "Budi Santoso"},
				{NPM: "2021200001", Name: "Izzan Firdaus"},
				{NPM: "2021200002", Name: "Andi Wijaya"},
			}, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// Search for "izzan" (lowercase) should match "Izzan" in names
	req := httptest.NewRequest(http.MethodGet, "/eval/student?q=izzan", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)

	// Should contain matching names
	if !strings.Contains(body.String(), "Mochamad Izzan Firasyansyah") {
		t.Error("body should contain 'Mochamad Izzan Firasyansyah'")
	}
	if !strings.Contains(body.String(), "Izzan Firdaus") {
		t.Error("body should contain 'Izzan Firdaus'")
	}
	// Should NOT contain non-matching names
	if strings.Contains(body.String(), "Budi Santoso") {
		t.Error("body should NOT contain 'Budi Santoso'")
	}
	if strings.Contains(body.String(), "Andi Wijaya") {
		t.Error("body should NOT contain 'Andi Wijaya'")
	}
}

// TestStudentPage_SearchEmptyQuery verifies all students shown when query is empty.
func TestStudentPage_SearchEmptyQuery(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			return []entity.StudentEntry{
				{NPM: "2020100001", Name: "Mochamad Izzan"},
				{NPM: "2020100002", Name: "Budi Santoso"},
				{NPM: "2021200001", Name: "Izzan Firdaus"},
			}, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// No query param — should show all
	req := httptest.NewRequest(http.MethodGet, "/eval/student", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)

	if !strings.Contains(body.String(), "2020100001") {
		t.Error("body should contain all students when no query")
	}
	if !strings.Contains(body.String(), "2020100002") {
		t.Error("body should contain all students when no query")
	}
	if !strings.Contains(body.String(), "2021200001") {
		t.Error("body should contain all students when no query")
	}
}

// TestStudentPage_SearchNoMatch verifies empty state when no students match.
func TestStudentPage_SearchNoMatch(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			return []entity.StudentEntry{
				{NPM: "2020100001", Name: "Mochamad Izzan"},
				{NPM: "2020100002", Name: "Budi Santoso"},
			}, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// Search for non-existent query
	req := httptest.NewRequest(http.MethodGet, "/eval/student?q=zzzzzzz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)

	if !strings.Contains(body.String(), "Tidak ada mahasiswa ditemukan") {
		t.Error("body should show empty state when no students match")
	}
}

// TestStudentPage_PaginationPreservesQuery verifies query param is preserved in pagination links.
func TestStudentPage_PaginationPreservesQuery(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			entries := make([]entity.StudentEntry, 0, 30)
			for i := 1; i <= 30; i++ {
				entries = append(entries, entity.StudentEntry{
					NPM:  "2020100" + string(rune('0'+i%10)),
					Name: "Mochamad Izzan " + string(rune('A'+i%26)),
				})
			}
			return entries, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// Search with pagination — should preserve query in pagination links
	req := httptest.NewRequest(http.MethodGet, "/eval/student?q=mochamad&page=1&per_page=25", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)

	// Pagination links should preserve the query parameter
	if !strings.Contains(body.String(), "q=mochamad") {
		t.Error("pagination links should preserve query parameter 'q=mochamad'")
	}
}

// TestStudentPage_SearchWithSpaces verifies search handles spaces in query (URL-encoded +).
func TestStudentPage_SearchWithSpaces(t *testing.T) {
	svc := &mockEvalService{
		studentListFunc: func() ([]entity.StudentEntry, error) {
			return []entity.StudentEntry{
				{NPM: "2020100001", Name: "Mochamad Izzan Firasyansyah"},
				{NPM: "2020100002", Name: "Budi Santoso"},
			}, nil
		},
	}
	h, err := handler.NewEvalHandler(svc, "", "", "", nil)
	if err != nil {
		t.Fatalf("NewEvalHandler: %v", err)
	}

	app := fiber.New()
	app.Get("/eval/student", h.StudentPage)

	// URL-encoded "mochamad+izzan+firasyansyah" should decode to "mochamad izzan firasyansyah"
	req := httptest.NewRequest(http.MethodGet, "/eval/student?q=mochamad+izzan+firasyansyah", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)

	if !strings.Contains(body.String(), "Mochamad Izzan Firasyansyah") {
		t.Error("body should match student with full name")
	}
}
