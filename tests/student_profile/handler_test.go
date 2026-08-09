package student_profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/interfaces/http/handler"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// --- mock service ---

type mockStudentProfileService struct {
	scrapeFunc   func(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error)
	getFunc      func(req entity.StudentProfileRequest) (*entity.StudentProfile, error)
	getPhotoFunc func(req entity.StudentProfileRequest) ([]byte, string, error)
}

func (m *mockStudentProfileService) Scrape(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error) {
	if m.scrapeFunc != nil {
		return m.scrapeFunc(req)
	}
	return &entity.StudentProfileResult{
		NPM:     req.NPM,
		Message: "Student profile downloaded",
	}, nil
}

func (m *mockStudentProfileService) Get(req entity.StudentProfileRequest) (*entity.StudentProfile, error) {
	if m.getFunc != nil {
		return m.getFunc(req)
	}
	return &entity.StudentProfile{
		PersonalData: entity.PersonalData{
			NIM: req.NPM,
		},
	}, nil
}

func (m *mockStudentProfileService) GetPhoto(req entity.StudentProfileRequest) ([]byte, string, error) {
	if m.getPhotoFunc != nil {
		return m.getPhotoFunc(req)
	}
	return nil, "", nil
}

// --- helpers ---

func newTestApp(svc *mockStudentProfileService) *fiber.App {
	h := handler.NewStudentProfileHandler(svc)
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})
	app.Post("/api/v1/lms/student-profile", h.Scrape)
	app.Post("/api/v1/lms/student-profile/data", h.Get)
	return app
}

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// --- Scrape tests ---

func TestHandler_Scrape_Success(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "2211700006",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	var envelope map[string]any
	json.NewDecoder(resp.Body).Decode(&envelope)
	if envelope["status"] != "success" {
		t.Errorf("status field = %q, want %q", envelope["status"], "success")
	}
}

func TestScrape_InvalidJSONBody(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_EmptyNPM(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (empty NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_InvalidNPMCharacters(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "221170000A",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (non-digit NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_NPMTooShort(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "1234567",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (short NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_NPMTooLong(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "2211700006123",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (long NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_EmptyPassword(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "2211700006",
		Password: "",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (empty password should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_ServiceError(t *testing.T) {
	svc := &mockStudentProfileService{
		scrapeFunc: func(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error) {
			return nil, fmt.Errorf("lms connection timeout")
		},
	}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "2211700006",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	// Service error is wrapped as Internal → 500
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("status = %d, want %d (service error should be 500)", resp.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestScrape_NPMBoundary8Chars(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "12345678",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (8-char NPM should be valid)", resp.StatusCode, fiber.StatusOK)
	}
}

func TestScrape_NPMBoundary12Chars(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "123456789012",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (12-char NPM should be valid)", resp.StatusCode, fiber.StatusOK)
	}
}

func TestScrape_ResponseEnvelope(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	npm := "2211700006"
	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      npm,
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	var envelope response.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Status != "success" {
		t.Errorf("status = %q, want %q", envelope.Status, "success")
	}
	if envelope.Message == "" {
		t.Error("message should not be empty")
	}
}

// --- Get tests ---

func TestHandler_Get_Success(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "2211700006",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	var envelope response.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Status != "success" {
		t.Errorf("status field = %q, want %q", envelope.Status, "success")
	}
}

func TestGet_InvalidJSONBody(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data",
		strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (invalid JSON should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGet_EmptyNPM(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGet_InvalidNPMCharacters(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "ABCDEFGH",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (non-digit NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGet_ProfileNotFound(t *testing.T) {
	svc := &mockStudentProfileService{
		getFunc: func(req entity.StudentProfileRequest) (*entity.StudentProfile, error) {
			return nil, errors.New("student profile not found")
		},
	}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "2211700006",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	// Get wraps errors as NotFound → 404
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d (not found should be 404)", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestGet_ResponseContainsProfileData(t *testing.T) {
	expectedNIM := "2211700006"
	expectedName := "Test Student"

	svc := &mockStudentProfileService{
		getFunc: func(req entity.StudentProfileRequest) (*entity.StudentProfile, error) {
			return &entity.StudentProfile{
				PersonalData: entity.PersonalData{
					NIM:           expectedNIM,
					NamaMahasiswa: expectedName,
				},
			}, nil
		},
	}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      expectedNIM,
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	var envelope response.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Decode nested data into StudentProfile
	dataBytes, _ := json.Marshal(envelope.Data)
	var profile entity.StudentProfile
	if err := json.Unmarshal(dataBytes, &profile); err != nil {
		t.Fatalf("decode profile data: %v", err)
	}

	if profile.PersonalData.NIM != expectedNIM {
		t.Errorf("NIM = %q, want %q", profile.PersonalData.NIM, expectedNIM)
	}
	if profile.PersonalData.NamaMahasiswa != expectedName {
		t.Errorf("NamaMahasiswa = %q, want %q", profile.PersonalData.NamaMahasiswa, expectedName)
	}
}

func TestGet_NPMBoundary8Chars(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "12345678",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (8-char NPM should be valid)", resp.StatusCode, fiber.StatusOK)
	}
}

func TestGet_NPMBoundary12Chars(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "123456789012",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d (12-char NPM should be valid)", resp.StatusCode, fiber.StatusOK)
	}
}

func TestGet_NPMBoundary7CharsInvalid(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "1234567",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (7-char NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGet_NPMBoundary13CharsInvalid(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "1234567890123",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (13-char NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

// --- Scrape/NPM boundary tests ---

func TestScrape_NPMBoundary7CharsInvalid(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "1234567",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (7-char NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_NPMBoundary13CharsInvalid(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	body := jsonBody(t, entity.StudentProfileRequest{
		NPM:      "1234567890123",
		Password: "test123",
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (13-char NPM should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestScrape_MissingBody(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	// Empty body → JSON bind error → 400
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (missing body should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGet_MissingBody(t *testing.T) {
	svc := &mockStudentProfileService{}
	app := newTestApp(svc)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/student-profile/data", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d (missing body should be 400)", resp.StatusCode, fiber.StatusBadRequest)
	}
}
