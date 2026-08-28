package eval_test

import (
	"bytes"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// newTestAppWithPDF wires the same routes as newTestApp but lets the caller
// pass a non-empty pdfDir so the preview loader is actually exercised.
func newTestAppWithPDF(svc *mockEvalService, pdfDir string) *fiber.App {
	evalHandler, err := handler.NewEvalHandler(svc, "", "", pdfDir, nil)
	if err != nil {
		panic(err)
	}
	evalGTHandler := handler.NewEvalGTHandler(svc)
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})
	app.Get("/eval/:npm/krs/:file/edit", evalHandler.EditKRS)
	app.Get("/eval/:npm/khs/:file/edit", evalHandler.EditKHS)
	app.Post("/api/v1/eval/:npm/krs/:file", evalGTHandler.SaveKRS)
	app.Post("/api/v1/eval/:npm/khs/:file", evalGTHandler.SaveKHS)
	return app
}

func getEditBody(t *testing.T, app *fiber.App, url string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
	body := bytes.Buffer{}
	_, _ = body.ReadFrom(resp.Body)
	return body.String()
}

func TestEditKRS_PreviewDiagnostic_PDFDirEmpty(t *testing.T) {
	svc := &mockEvalService{}
	app := newTestAppWithPDF(svc, "")

	body := getEditBody(t, app, "/eval/2211700006/krs/semester_8.json/edit")

	// Should embed the diagnostic block with a known reason code.
	if !strings.Contains(body, "Pratinjau PDF tidak tersedia.") {
		t.Error("body should contain the unavailable heading")
	}
	if !strings.Contains(body, "DOWNLOAD_DIR") {
		t.Error("body should mention DOWNLOAD_DIR env var")
	}
	if !strings.Contains(body, "pdf_dir_unset") {
		t.Error("body should expose reason_code pdf_dir_unset")
	}
}

func TestEditKRS_PreviewDiagnostic_NotFound(t *testing.T) {
	tmp := t.TempDir()
	svc := &mockEvalService{}
	app := newTestAppWithPDF(svc, tmp)

	body := getEditBody(t, app, "/eval/2211700006/krs/semester_8.json/edit")

	if !strings.Contains(body, "tidak ditemukan") {
		t.Error("body should mention 'tidak ditemukan' when PDF is absent")
	}
	if !strings.Contains(body, "pdf_not_found") {
		t.Error("body should expose reason_code pdf_not_found")
	}
	// Both candidate paths should be listed in the debug details.
	if !strings.Contains(body, filepath.Join(tmp, "2211700006", "krs", "semester_8.pdf")) {
		t.Error("body should list candidate (A) path")
	}
}

func TestEditKHS_PreviewDiagnostic_LoadedFromJSONFilename(t *testing.T) {
	tmp := t.TempDir()
	// Simulate the LMS download layout: npm/khs/2025_2026_GENAP.pdf
	khsDir := filepath.Join(tmp, "2211700006", "khs")
	if err := os.MkdirAll(khsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pdfBytes := []byte("%PDF-1.4 fake content for test")
	if err := os.WriteFile(filepath.Join(khsDir, "2025_2026_GENAP.pdf"), pdfBytes, 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	svc := &mockEvalService{}
	app := newTestAppWithPDF(svc, tmp)

	body := getEditBody(t, app, "/eval/2211700006/khs/2025_2026_GENAP.json/edit")

	// The iframe should embed the base64-encoded PDF.
	wantB64 := base64.StdEncoding.EncodeToString(pdfBytes)
	if !strings.Contains(body, "data:application/pdf;base64,"+wantB64) {
		t.Error("body should embed base64 PDF from disk")
	}
	if strings.Contains(body, "Pratinjau PDF tidak tersedia.") {
		t.Error("body should NOT show the unavailable block when PDF is present")
	}
}

func TestEditKRS_PreviewDiagnostic_OversizedFile(t *testing.T) {
	tmp := t.TempDir()
	krsDir := filepath.Join(tmp, "2211700006", "krs")
	if err := os.MkdirAll(krsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create a sparse file slightly above the 32 MiB cap by writing a header
	// then seeking. We use os.Create + Truncate.
	f, err := os.Create(filepath.Join(krsDir, "semester_8.pdf"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.Write([]byte("%PDF-1.4 ")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Truncate(33 * 1024 * 1024); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_ = f.Close()

	// Capture slog output to confirm the warn is emitted.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	svc := &mockEvalService{}
	app := newTestAppWithPDF(svc, tmp)
	body := getEditBody(t, app, "/eval/2211700006/krs/semester_8.json/edit")

	if !strings.Contains(body, "melebihi batas pratinjau") {
		t.Error("body should explain file is too large for preview")
	}
	if !strings.Contains(body, "pdf_too_large") {
		t.Error("body should expose reason_code pdf_too_large")
	}
	if !strings.Contains(buf.String(), "pdf preview skipped: file too large") {
		t.Errorf("slog should warn about oversized file; got: %s", buf.String())
	}
}

func TestEditKRS_PreviewDiagnostic_DirectPDFFilename(t *testing.T) {
	// When the URL parameter already ends in .pdf (e.g. a deep link), the
	// loader should still find the file via candidate (A) = the literal path.
	tmp := t.TempDir()
	krsDir := filepath.Join(tmp, "2211700006", "krs")
	if err := os.MkdirAll(krsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pdfBytes := []byte("%PDF-1.4 test")
	if err := os.WriteFile(filepath.Join(krsDir, "semester_8.pdf"), pdfBytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	svc := &mockEvalService{}
	app := newTestAppWithPDF(svc, tmp)
	body := getEditBody(t, app, "/eval/2211700006/krs/semester_8.pdf/edit")

	wantB64 := base64.StdEncoding.EncodeToString(pdfBytes)
	if !strings.Contains(body, "data:application/pdf;base64,"+wantB64) {
		t.Error("body should embed PDF even when URL parameter is .pdf")
	}
}

// silence unused-import warning for entity in case some toolchains drop a usage.
// (no additional imports needed beyond handler + fibererror for these tests)
