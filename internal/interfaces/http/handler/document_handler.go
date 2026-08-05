package handler

import (
	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// DocumentHandler handles HTTP requests for document download operations.
type DocumentHandler struct {
	docService service.LMSDocumentService
}

// NewDocumentHandler creates a DocumentHandler with its dependencies.
func NewDocumentHandler(docService service.LMSDocumentService) *DocumentHandler {
	return &DocumentHandler{docService: docService}
}

// DownloadKRS handles GET /api/v1/lms/krs?npm=xxx
func (h *DocumentHandler) DownloadKRS(c fiber.Ctx) error {
	npm := c.Query("npm")
	if npm == "" {
		return apperror.BadRequest("npm query parameter is required")
	}

	result, err := h.docService.DownloadKRS(npm)
	if err != nil {
		return apperror.Internal("KRS download failed", err)
	}

	return response.Success(c, fiber.StatusOK, result, result.Message)
}

// GetKHSSemesters handles GET /api/v1/lms/khs/semesters?npm=xxx
func (h *DocumentHandler) GetKHSSemesters(c fiber.Ctx) error {
	npm := c.Query("npm")
	if npm == "" {
		return apperror.BadRequest("npm query parameter is required")
	}

	result, err := h.docService.GetKHSSemesters(npm)
	if err != nil {
		return apperror.Internal("fetch KHS semesters failed", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KHS semesters retrieved")
}

// DownloadKHS handles GET /api/v1/lms/khs?npm=xxx&tahun_ajaran=xxx&semester=xxx
func (h *DocumentHandler) DownloadKHS(c fiber.Ctx) error {
	npm := c.Query("npm")
	tahunAjaran := c.Query("tahun_ajaran")
	semester := c.Query("semester")

	if npm == "" {
		return apperror.BadRequest("npm query parameter is required")
	}
	if tahunAjaran == "" {
		return apperror.BadRequest("tahun_ajaran query parameter is required")
	}
	if semester == "" {
		return apperror.BadRequest("semester query parameter is required")
	}

	result, err := h.docService.DownloadKHS(npm, tahunAjaran, semester)
	if err != nil {
		return apperror.Internal("KHS download failed", err)
	}

	return response.Success(c, fiber.StatusOK, result, result.Message)
}
