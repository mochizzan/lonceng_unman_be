package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
	"lonceng_unman_be/internal/interfaces/http/evalhtml"
	"lonceng_unman_be/internal/interfaces/http/response"

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
	parser    port.PDFParser
}

// NewEvalHandler creates a new eval handler.
func NewEvalHandler(evalSvc interface {
	Index() (entity.UnifiedEval, error)
	Student(npm string) (entity.StudentEval, error)
	LoadGT(npm, docType, filename string) ([]byte, error)
}, evalDir, pdfDir string, parser port.PDFParser,
) (*EvalHandler, error) {
	tmpl, err := template.ParseFS(evalhtml.TemplatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &EvalHandler{
		evalSvc:   evalSvc,
		evalDir:   evalDir,
		pdfDir:    pdfDir,
		templates: tmpl,
		parser:    parser,
	}, nil
}

// Breadcrumb represents a single breadcrumb item.
type Breadcrumb struct {
	Label string
	URL   string // empty for active/last item
}

// DashboardData wraps entity data with dashboard chrome metadata.
type DashboardData struct {
	Title       string
	ActivePage  string
	Breadcrumbs []Breadcrumb
	ExtraCSS    template.HTML
	ExtraJS     template.HTML
	// Computed counts for summary cards
	PairedCount int
	NeedGTCount int
	// Embedded entity data (UnifiedEval or StudentEval)
	Data interface{}
}

// Index handles GET /eval - uses unified table.
func (h *EvalHandler) Index(c fiber.Ctx) error {
	data, err := h.evalSvc.Index()
	if err != nil {
		return err
	}

	// Compute summary counts
	paired, needGT := 0, 0
	rawCount, extractedCount, gtCount := 0, 0, 0
	for _, r := range data.UnifiedRows {
		if r.Paired {
			paired++
		}
		if !r.HasGT && r.HasExtract {
			needGT++
		}
		if r.HasRaw {
			rawCount++
		}
		if r.HasExtract {
			extractedCount++
		}
		if r.HasGT {
			gtCount++
		}
	}

	// Enrich rows with badge classes
	type enrichedRow struct {
		entity.UnifiedTableRow
		CategoryBadgeClass string
		StatusBadgeClass   string
	}
	rows := make([]enrichedRow, 0, len(data.UnifiedRows))
	for _, r := range data.UnifiedRows {
		catClass := "bg-secondary"
		if r.Category == "KRS" {
			catClass = "bg-primary"
		} else if r.Category == "KHS" {
			catClass = "bg-info"
		}
		statusClass := mapStatusToBadge(r.Status)
		rows = append(rows, enrichedRow{r, catClass, statusClass})
	}

	dashData := map[string]interface{}{
		"Title":       "Dashboard Evaluasi",
		"ActivePage":  "index",
		"Breadcrumbs": []Breadcrumb{{Label: "Dashboard"}},
		"TotalNPMs":   data.TotalNPMs,
		"UnifiedRows": rows,
		"KRSMetrics":  data.KRSMetrics,
		"KHSMetrics":  data.KHSMetrics,
		"PairedCount": paired,
		"NeedGTCount": needGT,
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "index.html", dashData)
}

// Student handles GET /eval/:npm.
func (h *EvalHandler) Student(c fiber.Ctx) error {
	npm := c.Params("npm")
	result, err := h.evalSvc.Student(npm)
	if err != nil {
		return err
	}

	dashData := map[string]interface{}{
		"Title":      "Detail " + npm,
		"ActivePage": "detail",
		"Breadcrumbs": []Breadcrumb{
			{Label: "Dashboard", URL: "/eval"},
			{Label: npm},
		},
		"NPM":          result.NPM,
		"Name":         result.Name,
		"ProgramStudi": result.ProgramStudi,
		"KRSDocs":      result.KRSDocs,
		"KHSDocs":      result.KHSDocs,
		"KRSMetrics":   result.KRSMetrics,
		"KHSMetrics":   result.KHSMetrics,
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "detail.html", dashData)
}

// mapStatusToBadge maps status string to Bootstrap badge classes.
func mapStatusToBadge(status string) string {
	switch status {
	case "Paired":
		return "bg-success-subtle text-success-emphasis"
	case "Butuh Extract":
		return "bg-warning-subtle text-warning-emphasis"
	case "Butuh GT":
		return "bg-danger-subtle text-danger-emphasis"
	default:
		return "bg-secondary-subtle text-secondary-emphasis"
	}
}

// StudentList handles GET /api/v1/eval/students — returns JSON list of all NPMs with their data.
func (h *EvalHandler) StudentList(c fiber.Ctx) error {
	data, err := h.evalSvc.Index()
	if err != nil {
		return err
	}

	// Build a unique list of students from UnifiedRows
	seen := make(map[string]bool)
	students := []map[string]interface{}{}
	for _, r := range data.UnifiedRows {
		if seen[r.NPM] {
			continue
		}
		seen[r.NPM] = true
		students = append(students, map[string]interface{}{
			"npm":  r.NPM,
			"name": r.Name,
		})
	}

	return c.JSON(map[string]interface{}{
		"status": "success",
		"data":   students,
	})
}

// StudentPage handles GET /eval/student — standalone Student data page.
func (h *EvalHandler) StudentPage(c fiber.Ctx) error {
	dashData := map[string]interface{}{
		"Title":      "Data Mahasiswa",
		"ActivePage": "student",
		"Breadcrumbs": []Breadcrumb{
			{Label: "Dashboard", URL: "/eval"},
			{Label: "Mahasiswa"},
		},
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "student.html", dashData)
}

// KRSPage handles GET /eval/krs — standalone KRS data page.
func (h *EvalHandler) KRSPage(c fiber.Ctx) error {
	dashData := map[string]interface{}{
		"Title":      "Data KRS",
		"ActivePage": "krs",
		"Breadcrumbs": []Breadcrumb{
			{Label: "Dashboard", URL: "/eval"},
			{Label: "KRS"},
		},
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "krs.html", dashData)
}

// KHSPage handles GET /eval/khs — standalone KHS data page.
func (h *EvalHandler) KHSPage(c fiber.Ctx) error {
	dashData := map[string]interface{}{
		"Title":      "Data KHS",
		"ActivePage": "khs",
		"Breadcrumbs": []Breadcrumb{
			{Label: "Dashboard", URL: "/eval"},
			{Label: "KHS"},
		},
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "khs.html", dashData)
}

// EditKRS handles GET /eval/:npm/krs/:file/edit.
func (h *EvalHandler) EditKRS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")
	gtData, _ := h.evalSvc.LoadGT(npm, "krs", file)
	pdfData, pdfInfo := h.loadPDFBase64(npm, "krs", file)

	dashData := map[string]interface{}{
		"Title":      "Edit GT KRS - " + npm,
		"ActivePage": "edit",
		"Breadcrumbs": []Breadcrumb{
			{Label: "Dashboard", URL: "/eval"},
			{Label: npm, URL: "/eval/" + npm},
			{Label: "Edit KRS"},
		},
		"NPM":          npm,
		"File":         file,
		"GT":           parseGT(gtData, "krs"),
		"GTExists":     len(gtData) > 0,
		"RawPDFBase64": pdfData,
		"PDFInfo":      pdfInfo,
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "edit_krs.html", dashData)
}

// EditKHS handles GET /eval/:npm/khs/:file/edit.
func (h *EvalHandler) EditKHS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")
	gtData, _ := h.evalSvc.LoadGT(npm, "khs", file)
	pdfData, pdfInfo := h.loadPDFBase64(npm, "khs", file)

	dashData := map[string]interface{}{
		"Title":      "Edit GT KHS - " + npm,
		"ActivePage": "edit",
		"Breadcrumbs": []Breadcrumb{
			{Label: "Dashboard", URL: "/eval"},
			{Label: npm, URL: "/eval/" + npm},
			{Label: "Edit KHS"},
		},
		"NPM":          npm,
		"File":         file,
		"GT":           parseGT(gtData, "khs"),
		"GTExists":     len(gtData) > 0,
		"RawPDFBase64": pdfData,
		"PDFInfo":      pdfInfo,
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "edit_khs.html", dashData)
}

// AutoExtractKRS handles POST /api/v1/eval/:npm/krs/:file/auto-extract.
// It finds the KRS PDF, parses it using the existing parser, and returns
// the extraction result as JSON so the frontend can populate the form.
func (h *EvalHandler) AutoExtractKRS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")

	if err := validateNPM(npm); err != nil {
		return err
	}

	pdfPath, err := h.findKRSFile(npm, file)
	if err != nil {
		if errors.Is(err, apperror.ErrPDFNotFound) {
			return apperror.NotFound("KRS PDF not found for npm: "+npm, err)
		}
		return apperror.Internal("failed to find KRS PDF", err)
	}

	extraction, err := h.parser.ParseKRS(pdfPath, npm)
	if err != nil {
		return apperror.Internal("KRS extraction failed", err)
	}

	data, err := h.parser.MarshalToJSON(extraction)
	if err != nil {
		return apperror.Internal("failed to marshal extraction", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return apperror.Internal("failed to parse extraction", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KRS auto-extracted successfully")
}

// AutoExtractKHS handles POST /api/v1/eval/:npm/khs/:file/auto-extract.
// It finds the KHS PDF, parses it using the existing parser, and returns
// the extraction result as JSON so the frontend can populate the form.
func (h *EvalHandler) AutoExtractKHS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")

	if err := validateNPM(npm); err != nil {
		return err
	}

	tahunAjaran, semester, err := parseKHSFilename(file)
	if err != nil {
		return err
	}

	pdfPath, err := h.findKHSFile(npm, file)
	if err != nil {
		if errors.Is(err, apperror.ErrPDFNotFound) {
			return apperror.NotFound("KHS PDF not found for npm: "+npm, err)
		}
		return apperror.Internal("failed to find KHS PDF", err)
	}

	extraction, err := h.parser.ParseKHS(pdfPath, npm, tahunAjaran, semester)
	if err != nil {
		return apperror.Internal("KHS extraction failed", err)
	}

	data, err := h.parser.MarshalToJSON(extraction)
	if err != nil {
		return apperror.Internal("failed to marshal extraction", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return apperror.Internal("failed to parse extraction", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KHS auto-extracted successfully")
}

// findKRSFile finds the KRS PDF file for a given NPM and JSON filename.
// The JSON filename is like "semester_8.json"; the PDF is "semester_8.pdf".
func (h *EvalHandler) findKRSFile(npm, filename string) (string, error) {
	if h.pdfDir == "" {
		return "", fmt.Errorf("pdf dir not configured: %w", apperror.ErrPDFNotFound)
	}

	baseName := strings.TrimSuffix(filename, ".json")
	pdfName := baseName + ".pdf"
	candidate := filepath.Join(h.pdfDir, npm, entity.DocTypeKRS.String(), pdfName)

	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("krs pdf not found: %s: %w", candidate, apperror.ErrPDFNotFound)
		}
		return "", fmt.Errorf("stat krs pdf: %w", err)
	}

	return candidate, nil
}

// findKHSFile finds the KHS PDF file for a given NPM and JSON filename.
// The JSON filename is like "2022_2023_GANJIL.json"; the PDF is "2022_2023_GANJIL.pdf".
func (h *EvalHandler) findKHSFile(npm, filename string) (string, error) {
	if h.pdfDir == "" {
		return "", fmt.Errorf("pdf dir not configured: %w", apperror.ErrPDFNotFound)
	}

	baseName := strings.TrimSuffix(filename, ".json")
	pdfName := baseName + ".pdf"
	candidate := filepath.Join(h.pdfDir, npm, entity.DocTypeKHS.String(), pdfName)

	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("khs pdf not found: %s: %w", candidate, apperror.ErrPDFNotFound)
		}
		return "", fmt.Errorf("stat khs pdf: %w", err)
	}

	return candidate, nil
}

// parseKHSFilename extracts tahun ajaran and semester from a KHS filename.
// Expected format: "2022_2023_GANJIL.json" → ("2022_2023", "GANJIL").
func parseKHSFilename(filename string) (string, string, error) {
	base := strings.TrimSuffix(filename, ".json")
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return "", "", apperror.BadRequest("invalid KHS filename format, expected: tahunAwal_tahunAkhir_SEMESTER.json")
	}

	semester := parts[len(parts)-1]
	tahunAjaran := strings.Join(parts[:len(parts)-1], "_")
	return tahunAjaran, semester, nil
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

	// Resolve h.pdfDir to an absolute path so relative values like
	// "./downloads" do not silently miss files when the process CWD
	// differs from the directory the operator expects.
	absPDFDir, absErr := filepath.Abs(h.pdfDir)
	if absErr != nil {
		absPDFDir = h.pdfDir
	}

	// Try common PDF locations in priority order. Both candidates are
	// reported with their absolute form so the operator can see exactly
	// where the server looked (vs. where the file actually is).
	candidateNames := []string{
		replaceJSONWithPDF(filename),
		filename,
	}
	candidates := make([]string, 0, len(candidateNames))
	for _, name := range candidateNames {
		candidates = append(candidates, filepath.Join(absPDFDir, npm, docType, name))
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

	// No candidate matched — give a specific reason that points the
	// operator at the most likely root cause (CWD mismatch).
	cwd, _ := os.Getwd()
	reason := "Berkas PDF mentah tidak ditemukan di direktori unduhan. " +
		"Pastikan dokumen sudah pernah diunduh (POST /api/v1/lms/krs atau /khs) " +
		"sebelum mengedit ground truth."
	if absPDFDir != h.pdfDir {
		reason += fmt.Sprintf(
			" Catatan: DOWNLOAD_DIR=%q telah di-resolve ke %q terhadap CWD=%q.",
			h.pdfDir, absPDFDir, cwd,
		)
	}
	info.Reason = reason
	info.ReasonCode = "pdf_not_found"
	slog.Warn(
		"pdf preview unavailable: no candidate found",
		"npm", npm,
		"doc_type", docType,
		"file", filename,
		"pdf_dir", h.pdfDir,
		"abs_pdf_dir", absPDFDir,
		"cwd", cwd,
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
