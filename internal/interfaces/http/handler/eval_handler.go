package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/evalhtml"

	"github.com/gofiber/fiber/v3"
)

// maxPDFPreviewSize caps the raw PDF size that we will base64-embed in the
// edit page. Larger PDFs would blow up the HTML payload. The cap is
// intentionally generous (32 MiB) — most LMS PDFs are well under 1 MiB.
const maxPDFPreviewSize = 32 * 1024 * 1024

// EvalHandler handles HTML eval pages.
type EvalHandler struct {
	evalSvc interface {
		Index() (entity.UnifiedEval, error)
		Student(npm string) (entity.StudentEval, error)
		LoadGT(npm, docType, filename string) ([]byte, error)
	}
	evalDir   string
	pdfDir    string
	templates *template.Template
}

// NewEvalHandler creates a new eval handler.
func NewEvalHandler(evalSvc interface {
	Index() (entity.UnifiedEval, error)
	Student(npm string) (entity.StudentEval, error)
	LoadGT(npm, docType, filename string) ([]byte, error)
}, evalDir, pdfDir string,
) (*EvalHandler, error) {
	tmpl, err := template.ParseFS(evalhtml.FS, "*.html")
	if err != nil {
		return nil, err
	}
	return &EvalHandler{
		evalSvc:   evalSvc,
		evalDir:   evalDir,
		pdfDir:    pdfDir,
		templates: tmpl,
	}, nil
}

// Index handles GET /eval - uses unified table.
func (h *EvalHandler) Index(c fiber.Ctx) error {
	data, err := h.evalSvc.Index()
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "index.html", data)
}

// Student handles GET /eval/:npm.
func (h *EvalHandler) Student(c fiber.Ctx) error {
	npm := c.Params("npm")
	result, err := h.evalSvc.Student(npm)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "detail.html", result)
}

// EditKRS handles GET /eval/:npm/krs/:file/edit.
func (h *EvalHandler) EditKRS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")
	gtData, _ := h.evalSvc.LoadGT(npm, "krs", file)
	pdfData, pdfInfo := h.loadPDFBase64(npm, "krs", file)

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "edit_krs.html", map[string]interface{}{
		"NPM":          npm,
		"File":         file,
		"GT":           parseGT(gtData, "krs"),
		"GTExists":     len(gtData) > 0,
		"RawPDFBase64": pdfData,
		"PDFInfo":      pdfInfo,
	})
}

// EditKHS handles GET /eval/:npm/khs/:file/edit.
func (h *EvalHandler) EditKHS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")
	gtData, _ := h.evalSvc.LoadGT(npm, "khs", file)
	pdfData, pdfInfo := h.loadPDFBase64(npm, "khs", file)

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "edit_khs.html", map[string]interface{}{
		"NPM":          npm,
		"File":         file,
		"GT":           parseGT(gtData, "khs"),
		"GTExists":     len(gtData) > 0,
		"RawPDFBase64": pdfData,
		"PDFInfo":      pdfInfo,
	})
}

// PDFPreviewInfo describes the outcome of attempting to load the raw PDF
// for the edit page preview. It is rendered to the user so they understand
// why the preview is empty and what to do next.
type PDFPreviewInfo struct {
	Available    bool     `json:"available"`
	Reason       string   `json:"reason"`        // human-readable explanation
	ReasonCode   string   `json:"reason_code"`   // machine-friendly code
	TriedPaths   []string `json:"tried_paths"`   // absolute paths that were checked
	SizeBytes    int      `json:"size_bytes"`    // populated when Available
	OriginalName string   `json:"original_name"` // the raw PDF filename that was resolved
}

// loadPDFBase64 loads the raw PDF file as base64 for embedding in <iframe>.
// It also returns a PDFPreviewInfo describing the outcome so the UI can
// show a specific reason when the preview is empty (instead of a generic
// "tidak tersedia" message).
func (h *EvalHandler) loadPDFBase64(npm, docType, filename string) (string, PDFPreviewInfo) {
	info := PDFPreviewInfo{
		Available:  false,
		TriedPaths: []string{},
	}

	if h.pdfDir == "" {
		info.Reason = "Direktori PDF (DOWNLOAD_DIR) belum dikonfigurasi di server."
		info.ReasonCode = "pdf_dir_unset"
		slog.Warn("pdf preview disabled: pdfDir is empty", "npm", npm, "doc_type", docType, "file", filename)
		return "", info
	}

	// Try common PDF locations in priority order.
	candidates := []string{
		filepath.Join(h.pdfDir, npm, docType, replaceJSONWithPDF(filename)),
		filepath.Join(h.pdfDir, npm, docType, filename),
	}
	info.TriedPaths = candidates

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Debug(
				"pdf preview candidate miss",
				"npm", npm,
				"doc_type", docType,
				"file", filename,
				"path", path,
				"err", err,
			)
			continue
		}

		if len(data) > maxPDFPreviewSize {
			info.Reason = "Berkas PDF melebihi batas pratinjau (" +
				formatSize(maxPDFPreviewSize) + "). Unduh secara manual untuk melihat."
			info.ReasonCode = "pdf_too_large"
			info.OriginalName = filepath.Base(path)
			slog.Warn(
				"pdf preview skipped: file too large",
				"npm", npm,
				"doc_type", docType,
				"file", filename,
				"path", path,
				"size_bytes", len(data),
				"limit_bytes", maxPDFPreviewSize,
			)
			return "", info
		}

		info.Available = true
		info.SizeBytes = len(data)
		info.OriginalName = filepath.Base(path)
		slog.Info(
			"pdf preview loaded",
			"npm", npm,
			"doc_type", docType,
			"file", filename,
			"path", path,
			"size_bytes", len(data),
		)
		return base64.StdEncoding.EncodeToString(data), info
	}

	// No candidate matched — give a specific reason.
	info.Reason = "Berkas PDF mentah tidak ditemukan di direktori unduhan. " +
		"Pastikan dokumen sudah pernah diunduh (POST /api/v1/lms/krs atau /khs) " +
		"sebelum mengedit ground truth."
	info.ReasonCode = "pdf_not_found"
	slog.Warn(
		"pdf preview unavailable: no candidate found",
		"npm", npm,
		"doc_type", docType,
		"file", filename,
		"tried_paths", candidates,
	)
	return "", info
}

// parseGT parses GT JSON into a map for template consumption.
func parseGT(data []byte, docType string) map[string]interface{} {
	result := make(map[string]interface{})
	if len(data) == 0 {
		return result
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return make(map[string]interface{})
	}
	return result
}

// replaceJSONWithPDF replaces .json suffix with .pdf (or removes suffix).
func replaceJSONWithPDF(filename string) string {
	if strings.HasSuffix(filename, ".json") {
		return strings.TrimSuffix(filename, ".json") + ".pdf"
	}
	if !strings.HasSuffix(filename, ".pdf") {
		return filename + ".pdf"
	}
	return filename
}

// formatSize renders a byte count as a short human-readable string.
func formatSize(n int) string {
	const (
		kb = 1024
		mb = 1024 * 1024
	)
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
