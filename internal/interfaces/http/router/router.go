package router

import (
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// Setup registers all application routes on the Fiber app.
func Setup(app *fiber.App, healthHandler *handler.HealthHandler, lmsHandler *handler.LMSHandler, docHandler *handler.DocumentHandler) {
	v1 := app.Group("/api/v1")

	// Health
	v1.Get("/health", healthHandler.Check)

	// LMS
	v1.Post("/lms/login", lmsHandler.Login)

	// Documents
	v1.Get("/lms/krs", docHandler.DownloadKRS)
	v1.Get("/lms/khs/semesters", docHandler.GetKHSSemesters)
	v1.Get("/lms/khs", docHandler.DownloadKHS)
}
