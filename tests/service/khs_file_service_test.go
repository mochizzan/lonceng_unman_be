package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockBrowserSession struct{}

func (m *mockBrowserSession) Navigate(url string) error      { return nil }
func (m *mockBrowserSession) Eval(js string) (string, error) { return "{}", nil }
func (m *mockBrowserSession) ElementAttribute(sel, attr string) (string, error) {
	return "", nil
}
func (m *mockBrowserSession) ElementExists(sel string) (bool, error) { return true, nil }
func (m *mockBrowserSession) ElementHref(sel string) (string, error) { return "", nil }
func (m *mockBrowserSession) DownloadPDF(url, save string) (string, int, error) {
	return "", 0, nil
}

func (m *mockBrowserSession) DownloadImage(url, save string) (string, int, error) {
	return "", 0, nil
}
func (m *mockBrowserSession) Close() error { return nil }

type mockSessionManager struct {
	createFunc func(npm, password string) (port.BrowserSession, error)
	closeFunc  func(npm string) error
}

func (m *mockSessionManager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	if m.createFunc != nil {
		return m.createFunc(npm, password)
	}
	return &mockBrowserSession{}, nil
}

func (m *mockSessionManager) Close(npm string) error {
	if m.closeFunc != nil {
		return m.closeFunc(npm)
	}
	return nil
}
func (m *mockSessionManager) CloseAll()                  {}
func (m *mockSessionManager) MarkStale(npm string) bool  { return false }
func (m *mockSessionManager) Invalidate(npm string) bool { return m.MarkStale(npm) }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		App: config.AppConfig{
			DownloadDir: t.TempDir(),
		},
	}
}

func newTestService(t *testing.T, sessions port.SessionManager) service.LMSDocumentService {
	t.Helper()
	return service.NewLMSDocumentService(newTestConfig(t), sessions)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestKHSFileService(t *testing.T) {
	validReq := entity.KHSDownloadRequest{
		NPM:         "2211700006",
		Password:    "test123",
		TahunAjaran: "2022/2023",
		Semester:    "GANJIL",
	}

	t.Run("file exists returns path and size", func(t *testing.T) {
		sessions := &mockSessionManager{}

		// Create the expected PDF file
		downloadDir := t.TempDir()
		cfg := &config.Config{
			App: config.AppConfig{
				DownloadDir: downloadDir,
			},
		}
		svc := service.NewLMSDocumentService(cfg, sessions)

		khsDir := filepath.Join(downloadDir, "2211700006", "khs")
		if err := os.MkdirAll(khsDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		pdfPath := filepath.Join(khsDir, entity.KHSFilename("2022/2023", "GANJIL"))
		content := []byte("fake PDF content for testing")
		if err := os.WriteFile(pdfPath, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		path, size, err := svc.DownloadKHSFile(validReq)
		if err != nil {
			t.Fatalf("DownloadKHSFile() error = %v", err)
		}
		if path != pdfPath {
			t.Errorf("path = %q, want %q", path, pdfPath)
		}
		if size != int64(len(content)) {
			t.Errorf("size = %d, want %d", size, len(content))
		}
	})

	t.Run("file missing returns NotFound", func(t *testing.T) {
		downloadDir := t.TempDir()
		cfg := &config.Config{
			App: config.AppConfig{
				DownloadDir: downloadDir,
			},
		}
		sessions := &mockSessionManager{}
		svc := service.NewLMSDocumentService(cfg, sessions)

		// Create directory but not the file
		khsDir := filepath.Join(downloadDir, "2211700006", "khs")
		if err := os.MkdirAll(khsDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		_, _, err := svc.DownloadKHSFile(validReq)
		if err == nil {
			t.Fatal("DownloadKHSFile() should return error when file missing")
		}
		appErr, ok := err.(*apperror.AppError)
		if !ok {
			t.Fatalf("error type = %T, want *apperror.AppError", err)
		}
		if appErr.StatusCode != 404 {
			t.Errorf("status = %d, want 404", appErr.StatusCode)
		}
	})

	t.Run("invalid semester returns BadRequest", func(t *testing.T) {
		sessions := &mockSessionManager{}
		svc := newTestService(t, sessions)

		req := validReq
		req.Semester = "INVALID"
		_, _, err := svc.DownloadKHSFile(req)
		if err == nil {
			t.Fatal("DownloadKHSFile() should return error for invalid semester")
		}
		appErr, ok := err.(*apperror.AppError)
		if !ok {
			t.Fatalf("error type = %T, want *apperror.AppError", err)
		}
		if appErr.StatusCode != 400 {
			t.Errorf("status = %d, want 400", appErr.StatusCode)
		}
	})

	t.Run("session creation failure returns error", func(t *testing.T) {
		sessions := &mockSessionManager{
			createFunc: func(npm, password string) (port.BrowserSession, error) {
				return nil, fmt.Errorf("login failed")
			},
		}
		svc := newTestService(t, sessions)

		_, _, err := svc.DownloadKHSFile(validReq)
		if err == nil {
			t.Fatal("DownloadKHSFile() should return error when session creation fails")
		}
		// Error is wrapped, not AppError — will be handled as 500 at handler level
		if err.Error() != "get session: login failed" {
			t.Errorf("error = %q, want %q", err.Error(), "get session: login failed")
		}
	})

	t.Run("lowercase semester is normalized", func(t *testing.T) {
		downloadDir := t.TempDir()
		cfg := &config.Config{
			App: config.AppConfig{
				DownloadDir: downloadDir,
			},
		}
		sessions := &mockSessionManager{}
		svc := service.NewLMSDocumentService(cfg, sessions)

		khsDir := filepath.Join(downloadDir, "2211700006", "khs")
		if err := os.MkdirAll(khsDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		pdfPath := filepath.Join(khsDir, entity.KHSFilename("2022/2023", "GANJIL"))
		content := []byte("fake PDF content")
		if err := os.WriteFile(pdfPath, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		req := validReq
		req.Semester = "ganjil" // lowercase
		path, size, err := svc.DownloadKHSFile(req)
		if err != nil {
			t.Fatalf("DownloadKHSFile() error = %v", err)
		}
		if path != pdfPath {
			t.Errorf("path = %q, want %q", path, pdfPath)
		}
		if size != int64(len(content)) {
			t.Errorf("size = %d, want %d", size, len(content))
		}
	})
}
