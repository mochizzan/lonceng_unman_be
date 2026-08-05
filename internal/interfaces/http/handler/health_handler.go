package handler

import (
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// HealthHandler handles HTTP requests for health checks.
type HealthHandler struct {
	healthService service.HealthChecker
}

// NewHealthHandler creates a HealthHandler with its dependencies.
func NewHealthHandler(healthService service.HealthChecker) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

// Check handles GET /api/v1/health and returns the service health status.
func (h *HealthHandler) Check(c fiber.Ctx) error {
	status := h.healthService.Check()
	return response.Success(c, fiber.StatusOK, status, "Service is healthy")
}
