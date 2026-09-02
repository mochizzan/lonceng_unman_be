package router

import (
	"io/fs"
	"path"
	"strings"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/auth"
	"lonceng_unman_be/internal/interfaces/http/evalhtml"
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// Setup registers all application routes on the Fiber app.
func Setup(app *fiber.App, healthHandler *handler.HealthHandler, lmsHandler *handler.LMSHandler, docHandler *handler.DocumentHandler, extractionHandler *handler.ExtractionHandler, studentProfileHandler *handler.StudentProfileHandler, evalHandler *handler.EvalHandler, evalDataHandler *handler.EvalDataHandler, evalGTHandler *handler.EvalGTHandler, authHandler *handler.AuthHandler, cfg *config.Config) {
	v1 := app.Group("/api/v1")

	// Health
	v1.Get("/health", healthHandler.Check)

	// LMS
	v1.Post("/lms/login", lmsHandler.Login)

	// Documents
	v1.Post("/lms/krs", docHandler.DownloadKRS)
	v1.Post("/lms/khs/semesters", docHandler.GetKHSSemesters)
	v1.Post("/lms/khs", docHandler.DownloadKHS)
	v1.Post("/lms/khs/file", docHandler.DownloadKHSFile)

	// Extraction
	v1.Post("/lms/krs/extract", extractionHandler.ExtractKRS)
	v1.Post("/lms/khs/extract", extractionHandler.ExtractKHS)
	v1.Post("/lms/krs/data", extractionHandler.GetKRS)
	v1.Post("/lms/khs/data", extractionHandler.GetKHS)

	// Student Profile
	v1.Post("/lms/student-profile", studentProfileHandler.Scrape)
	v1.Post("/lms/student-profile/data", studentProfileHandler.Get)
	v1.Post("/lms/student-profile/photo", studentProfileHandler.GetPhoto)

	// Serve the static asset tree (CSS modules, JS bundles) under /eval/static/*.
	// Registered OUTSIDE auth scope — must remain public so the login page
	// can load its CSS/JS before the user authenticates.
	// All assets are read from evalhtml.StaticFS (embedded).
	app.Get("/eval/static/*", staticHandler)

	// Auth endpoints (public — login page, logout, rate-limited POST)
	app.Get("/eval/login", authHandler.LoginPage)
	app.Post("/eval/login", authHandler.Login)
	app.Get("/eval/logout", authHandler.Logout)

	// Eval auth middleware (protects all /eval HTML and /api/v1/eval JSON routes)
	evalAuth := auth.EvalAuth(cfg, authHandler.Limiter())

	// Eval JSON API group (under /api/v1) — protected
	evalV1 := v1.Group("/eval", evalAuth)
	evalV1.Post("/:npm/krs/:file", evalGTHandler.SaveKRS)
	evalV1.Post("/:npm/khs/:file", evalGTHandler.SaveKHS)
	evalV1.Post("/:npm/krs/:file/auto-extract", evalHandler.AutoExtractKRS)
	evalV1.Post("/:npm/khs/:file/auto-extract", evalHandler.AutoExtractKHS)
	evalV1.Get("/:npm/krs/:file/extraction-status", evalHandler.ExtractionStatusKRS)
	evalV1.Get("/:npm/khs/:file/extraction-status", evalHandler.ExtractionStatusKHS)
	evalV1.Post("/:npm/krs/:file/extract-and-save", evalHandler.ExtractAndSaveKRS)
	evalV1.Post("/:npm/khs/:file/extract-and-save", evalHandler.ExtractAndSaveKHS)

	// Eval HTML pages (outside /api/v1) — protected
	// Static routes must be registered before parameterized routes.
	evalHTML := app.Group("/eval", evalAuth)
	evalHTML.Get("/student", evalHandler.StudentPage)
	evalHTML.Get("/krs", evalDataHandler.KRSPage)
	evalHTML.Get("/khs", evalDataHandler.KHSPage)
	evalHTML.Get("/", evalHandler.Index)
	evalHTML.Get("/pipeline", evalHandler.PipelinePage)
	evalHTML.Get("/:npm", evalHandler.Student)
	evalHTML.Get("/:npm/krs/:file/edit", evalHandler.EditKRS)
	evalHTML.Get("/:npm/khs/:file/edit", evalHandler.EditKHS)
}

// staticContentType maps file extensions to their canonical MIME type.
var staticContentType = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "application/javascript; charset=utf-8",
	".mjs":   "application/javascript; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

// staticHandler serves a single file from evalhtml.StaticFS.
//
// The wildcard path (e.g. "/eval/static/css/main.css") is normalized and
// looked up under the "static/" prefix of the embed.FS. Path traversal is
// rejected by cleaning the relative path and ensuring it stays within the
// "static/" subtree.
func staticHandler(c fiber.Ctx) error {
	// c.Params("*") is the wildcard suffix after the prefix.
	rel := c.Params("*")
	if rel == "" {
		rel = "index.html"
	}

	// Normalize and reject path traversal.
	cleaned := path.Clean("/" + rel)
	if strings.Contains(cleaned, "..") {
		return apperror.Forbidden("invalid path")
	}

	fsPath := "static" + cleaned
	data, err := fs.ReadFile(evalhtml.StaticFS, fsPath)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "static asset not found")
	}

	ext := strings.ToLower(path.Ext(cleaned))
	if ct, ok := staticContentType[ext]; ok {
		c.Set("Content-Type", ct)
	}
	// Long-lived cache for static assets (they're embedded in the binary).
	c.Set("Cache-Control", "public, max-age=31536000, immutable")

	return c.Send(data)
}
