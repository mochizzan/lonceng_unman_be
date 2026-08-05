package router

import (
	"lonceng_unman_be/internal/interfaces/http/handler"

	"github.com/gofiber/fiber/v3"
)

// Setup registers all application routes on the Fiber app.
func Setup(app *fiber.App, healthHandler *handler.HealthHandler) {
	v1 := app.Group("/api/v1")

	// Health
	v1.Get("/health", healthHandler.Check)
}
