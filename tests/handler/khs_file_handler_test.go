package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// --- mock service ---

type mockLMSDocumentService struct {
	downloadKHSFileFunc func(req entity.KHSDownloadRequest) (string, int64, error)
}

func (m *mockLMSDocumentService) DownloadKRS(req entity.KRSDownloadRequest) (*entity.KRSDownloadResult, error) {
	return nil, nil
}

func (m *mockLMSDocumentService) GetKHSSemesters(req entity.KHSSemestersRequest) (*entity.KHSSemestersResult, error) {
	return nil, nil
}

func (m *mockLMSDocumentService) DownloadKHS(req entity.KHSDownloadRequest) (*entity.KHSDownloadResult, error) {
	return nil, nil
}

func (m *mockLMSDocumentService) DownloadKHSFile(req entity.KHSDownloadRequest) (string, int64, error) {
	if m.downloadKHSFileFunc != nil {
		return m.downloadKHSFileFunc(req)
	}
	return "", 0, nil
}

// --- helpers ---

func newTestApp(svc *mockLMSDocumentService) *fiber.App {
	h := handler.NewDocumentHandler(svc)
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})
	app.Post("/api/v1/lms/khs/file", h.DownloadKHSFile)
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

// --- tests ---

func TestKHSFileHandler(t *testing.T) {
	validReq := entity.KHSDownloadRequest{
		NPM:         "2211700006",
		Password:    "test123",
		TahunAjaran: "2022/2023",
		Semester:    "GANJIL",
	}

	t.Run("valid request with existing PDF returns 200 with PDF headers", func(t *testing.T) {
		// Create a real temp file for SendFile to serve
		tmpFile, err := os.CreateTemp("", "test-*.pdf")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		content := []byte("fake PDF content for testing")
		tmpFile.Write(content)
		tmpFile.Close()

		svc := &mockLMSDocumentService{
			downloadKHSFileFunc: func(req entity.KHSDownloadRequest) (string, int64, error) {
				return tmpFile.Name(), int64(len(content)), nil
			},
		}
		app := newTestApp(svc)

		body := jsonBody(t, validReq)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/khs/file", body)
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/pdf")
		}
		cd := resp.Header.Get("Content-Disposition")
		if cd == "" || !bytes.Contains([]byte(cd), []byte("attachment")) {
			t.Errorf("Content-Disposition = %q, want attachment with filename", cd)
		}
		cl := resp.Header.Get("Content-Length")
		if cl != fmt.Sprintf("%d", len(content)) {
			t.Errorf("Content-Length = %q, want %q", cl, fmt.Sprintf("%d", len(content)))
		}
		if ar := resp.Header.Get("Accept-Ranges"); ar != "bytes" {
			t.Errorf("Accept-Ranges = %q, want %q", ar, "bytes")
		}
	})

	t.Run("valid request with non-existing PDF returns 404", func(t *testing.T) {
		svc := &mockLMSDocumentService{
			downloadKHSFileFunc: func(req entity.KHSDownloadRequest) (string, int64, error) {
				return "", 0, apperror.NotFound("KHS PDF not found for the specified year and semester", nil)
			},
		}
		app := newTestApp(svc)

		body := jsonBody(t, validReq)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/khs/file", body)
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != fiber.StatusNotFound {
			t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
		}
	})

	t.Run("session creation failure returns 500", func(t *testing.T) {
		svc := &mockLMSDocumentService{
			downloadKHSFileFunc: func(req entity.KHSDownloadRequest) (string, int64, error) {
				return "", 0, fmt.Errorf("get session: login failed")
			},
		}
		app := newTestApp(svc)

		body := jsonBody(t, validReq)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/khs/file", body)
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusInternalServerError)
		}
	})

	t.Run("missing fields return 400", func(t *testing.T) {
		tests := []struct {
			name string
			body any
		}{
			{"empty body", nil},
			{"empty npm", entity.KHSDownloadRequest{NPM: "", Password: "test123", TahunAjaran: "2022/2023", Semester: "GANJIL"}},
			{"empty password", entity.KHSDownloadRequest{NPM: "2211700006", Password: "", TahunAjaran: "2022/2023", Semester: "GANJIL"}},
			{"empty tahunAjaran", entity.KHSDownloadRequest{NPM: "2211700006", Password: "test123", TahunAjaran: "", Semester: "GANJIL"}},
			{"empty semester", entity.KHSDownloadRequest{NPM: "2211700006", Password: "test123", TahunAjaran: "2022/2023", Semester: ""}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockLMSDocumentService{}
				app := newTestApp(svc)

				var body *bytes.Reader
				if tt.body == nil {
					body = bytes.NewReader([]byte("{}"))
				} else {
					body = jsonBody(t, tt.body)
				}
				req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/khs/file", body)
				req.Header.Set("Content-Type", "application/json")

				resp, err := app.Test(req)
				if err != nil {
					t.Fatalf("app.Test: %v", err)
				}
				if resp.StatusCode != fiber.StatusBadRequest {
					t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
				}
			})
		}
	})

	t.Run("invalid semester value returns 400", func(t *testing.T) {
		svc := &mockLMSDocumentService{
			downloadKHSFileFunc: func(req entity.KHSDownloadRequest) (string, int64, error) {
				return "", 0, apperror.BadRequest("semester must be GANJIL or GENAP")
			},
		}
		app := newTestApp(svc)

		body := jsonBody(t, entity.KHSDownloadRequest{
			NPM:         "2211700006",
			Password:    "test123",
			TahunAjaran: "2022/2023",
			Semester:    "INVALID",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/khs/file", body)
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
		}
	})

	t.Run("invalid NPM returns 400", func(t *testing.T) {
		tests := []struct {
			name string
			npm  string
		}{
			{"non-digit", "221170000A"},
			{"too short", "1234567"},
			{"too long", "2211700006123"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockLMSDocumentService{}
				app := newTestApp(svc)

				body := jsonBody(t, entity.KHSDownloadRequest{
					NPM:         tt.npm,
					Password:    "test123",
					TahunAjaran: "2022/2023",
					Semester:    "GANJIL",
				})
				req, _ := http.NewRequest(http.MethodPost, "/api/v1/lms/khs/file", body)
				req.Header.Set("Content-Type", "application/json")

				resp, err := app.Test(req)
				if err != nil {
					t.Fatalf("app.Test: %v", err)
				}
				if resp.StatusCode != fiber.StatusBadRequest {
					t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
				}
			})
		}
	})
}
