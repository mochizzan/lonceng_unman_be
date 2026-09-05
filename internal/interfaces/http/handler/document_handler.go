package handler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

var npmRegexp = regexp.MustCompile(`^[0-9]+$`)

// DocumentHandler handles HTTP requests for document download operations.
type DocumentHandler struct {
	docService service.LMSDocumentService
}

// NewDocumentHandler creates a DocumentHandler with its dependencies.
func NewDocumentHandler(docService service.LMSDocumentService) *DocumentHandler {
	return &DocumentHandler{docService: docService}
}

// validateNPM checks that the NPM is non-empty, contains only digits, and is 8-12 characters.
func validateNPM(npm string) error {
	if npm == "" {
		return apperror.BadRequest("npm is required")
	}
	if !npmRegexp.MatchString(npm) {
		return apperror.BadRequest("npm must contain only digits")
	}
	if len(npm) < 8 || len(npm) > 12 {
		return apperror.BadRequest("npm must be 8-12 characters")
	}
	return nil
}

// DownloadKRS handles POST /api/v1/lms/krs
func (h *DocumentHandler) DownloadKRS(c fiber.Ctx) error {
	var req entity.KRSDownloadRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := validateNPM(req.NPM); err != nil {
		return err
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	result, err := h.docService.DownloadKRS(req)
	if err != nil {
		return apperror.ClassifyDocumentError(err, "KRS download failed")
	}

	return response.Success(c, fiber.StatusOK, result, result.Message)
}

// GetKHSSemesters handles POST /api/v1/lms/khs/semesters
func (h *DocumentHandler) GetKHSSemesters(c fiber.Ctx) error {
	var req entity.KHSSemestersRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := validateNPM(req.NPM); err != nil {
		return err
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	result, err := h.docService.GetKHSSemesters(req)
	if err != nil {
		return apperror.ClassifyDocumentError(err, "fetch KHS semesters failed")
	}

	return response.Success(c, fiber.StatusOK, result, result.Message)
}

// DownloadKHS handles POST /api/v1/lms/khs
func (h *DocumentHandler) DownloadKHS(c fiber.Ctx) error {
	var req entity.KHSDownloadRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := validateNPM(req.NPM); err != nil {
		return err
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}
	if req.TahunAjaran == "" {
		return apperror.BadRequest("tahun_ajaran is required")
	}
	if req.Semester == "" {
		return apperror.BadRequest("semester is required")
	}

	result, err := h.docService.DownloadKHS(req)
	if err != nil {
		return apperror.ClassifyDocumentError(err, "KHS download failed")
	}

	return response.Success(c, fiber.StatusOK, result, result.Message)
}

// DownloadKHSFile handles POST /api/v1/lms/khs/file
func (h *DocumentHandler) DownloadKHSFile(c fiber.Ctx) error {
	var req entity.KHSDownloadRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := validateNPM(req.NPM); err != nil {
		return err
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}
	if req.TahunAjaran == "" {
		return apperror.BadRequest("tahun_ajaran is required")
	}
	if req.Semester == "" {
		return apperror.BadRequest("semester is required")
	}

	filePath, size, err := h.docService.DownloadKHSFile(req)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="KHS_%s_%s_%s.pdf"`,
		req.NPM, strings.ReplaceAll(req.TahunAjaran, "/", "_"), req.Semester,
	))
	c.Set("Content-Length", strconv.FormatInt(size, 10))
	c.Set("Accept-Ranges", "bytes")

	return c.SendFile(filePath)
}
