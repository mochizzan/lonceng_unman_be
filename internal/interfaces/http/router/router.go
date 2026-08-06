package router

import (
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// Setup registers all application routes on the Fiber app.
func Setup(app *fiber.App, healthHandler *handler.HealthHandler, lmsHandler *handler.LMSHandler, docHandler *handler.DocumentHandler, extractionHandler *handler.ExtractionHandler) {
	v1 := app.Group("/api/v1")

	// Health
	v1.Get("/health", healthHandler.Check)

	// LMS
	v1.Post("/lms/login", lmsHandler.Login)

	// Documents
	v1.Post("/lms/krs", docHandler.DownloadKRS)
	v1.Post("/lms/khs/semesters", docHandler.GetKHSSemesters)
	v1.Post("/lms/khs", docHandler.DownloadKHS)

	// Extraction
	v1.Post("/lms/krs/extract", extractionHandler.ExtractKRS)
	v1.Post("/lms/khs/extract", extractionHandler.ExtractKHS)
	v1.Get("/lms/krs/data/:npm", extractionHandler.GetKRS)
	v1.Get("/lms/khs/data/:npm/:tahun_ajaran/:semester", extractionHandler.GetKHS)
}
