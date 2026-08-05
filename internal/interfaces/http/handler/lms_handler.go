package handler

import (
	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// LMSHandler handles HTTP requests for LMS operations.
type LMSHandler struct {
	lmsService service.LMSLogin
}

// NewLMSHandler creates an LMSHandler with its dependencies.
func NewLMSHandler(lmsService service.LMSLogin) *LMSHandler {
	return &LMSHandler{lmsService: lmsService}
}

// Login handles POST /api/v1/lms/login.
func (h *LMSHandler) Login(c fiber.Ctx) error {
	var req entity.LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body: " + err.Error())
	}

	if req.NPM == "" {
		return apperror.BadRequest("npm is required")
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	result, err := h.lmsService.Login(req)
	if err != nil {
		return apperror.Internal("login operation failed", err)
	}

	return response.Success(c, fiber.StatusOK, result, result.Message)
}
