package handler

import (
	"encoding/json"
	"html/template"
	"strings"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/evalhtml"

	"github.com/gofiber/fiber/v3"
)

// EvalDataHandler handles SSR for KRS/KHS data pages.
// Calls ExtractionService directly — does NOT use HTTP endpoints.
type EvalDataHandler struct {
	extractionSvc extractionServiceInterface
	templates     *template.Template
}

type extractionServiceInterface interface {
	GetKRSExtraction(npm string) ([]byte, error)
	GetKHSExtraction(npm string, tahunAjaran string, semester string) ([]byte, error)
}

// NewEvalDataHandler creates a new eval data handler.
func NewEvalDataHandler(extractionSvc extractionServiceInterface) (*EvalDataHandler, error) {
	tmpl, err := template.New("eval_data").Funcs(templateFuncs).ParseFS(evalhtml.TemplatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &EvalDataHandler{
		extractionSvc: extractionSvc,
		templates:     tmpl,
	}, nil
}

// KRSPage handles GET /eval/krs?npm=...
func (h *EvalDataHandler) KRSPage(c fiber.Ctx) error {
	npm := c.Query("npm")
	data := map[string]interface{}{
		"Title":       "Data KRS",
		"ActivePage":  "krs",
		"Breadcrumbs": []Breadcrumb{{Label: "Dashboard", URL: "/eval"}, {Label: "KRS"}},
	}
	if npm != "" {
		if err := validateNPM(npm); err != nil {
			data["Error"] = "NPM tidak valid: " + err.Error()
		} else {
			raw, err := h.extractionSvc.GetKRSExtraction(npm)
			if err != nil {
				data["Error"] = "Data KRS tidak ditemukan untuk NPM " + npm + ". Silakan ekstrak terlebih melalui halaman dokumen."
			} else {
				var krs entity.KRSExtraction
				if err := json.Unmarshal(raw, &krs); err == nil {
					data["KRS"] = krs.KRS
					data["NPM"] = npm
				} else {
					data["Error"] = "Gagal membaca data KRS. Format file tidak valid."
				}
			}
		}
	}
	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "krs.html", data)
}

// KHSPage handles GET /eval/khs?npm=...&tahun_ajaran=...&semester=...
func (h *EvalDataHandler) KHSPage(c fiber.Ctx) error {
	npm := c.Query("npm")
	tahun := c.Query("tahun_ajaran")
	semester := c.Query("semester")
	data := map[string]interface{}{
		"Title":       "Data KHS",
		"ActivePage":  "khs",
		"Breadcrumbs": []Breadcrumb{{Label: "Dashboard", URL: "/eval"}, {Label: "KHS"}},
	}
	if npm != "" || tahun != "" || semester != "" {
		// Only attempt to load if at least one param is provided
		if npm == "" {
			data["Error"] = "NPM wajib diisi."
		} else if err := validateNPM(npm); err != nil {
			data["Error"] = "NPM tidak valid: " + err.Error()
		} else if tahun == "" {
			data["Error"] = "Tahun ajaran wajib diisi."
		} else if semester == "" {
			data["Error"] = "Semester wajib dipilih."
		} else {
			semester = strings.ToUpper(semester)
			raw, err := h.extractionSvc.GetKHSExtraction(npm, tahun, semester)
			if err != nil {
				data["Error"] = "Data KHS tidak ditemukan untuk NPM " + npm + " semester " + semester + " " + tahun + ". Silakan ekstrak terlebih melalui halaman dokumen."
			} else {
				var khs entity.KHSExtraction
				if err := json.Unmarshal(raw, &khs); err == nil {
					data["KHS"] = khs.KHS
					data["NPM"] = npm
				} else {
					data["Error"] = "Gagal membaca data KHS. Format file tidak valid."
				}
			}
		}
	}
	// Pass these so the form can retain values
	data["TahunAjaran"] = tahun
	data["Semester"] = semester
	data["NPM"] = npm
	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "khs.html", data)
}
