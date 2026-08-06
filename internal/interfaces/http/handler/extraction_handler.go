package handler

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v3"
	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/interfaces/http/response"
)

// ExtractionHandler handles HTTP requests for PDF extraction.
type ExtractionHandler struct {
	extractionSvc service.ExtractionService
}

// NewExtractionHandler creates a new extraction handler.
func NewExtractionHandler(extractionSvc service.ExtractionService) *ExtractionHandler {
	return &ExtractionHandler{extractionSvc: extractionSvc}
}

// KRSExtractionRequest represents the request body for KRS extraction.
type KRSExtractionRequest struct {
	NPM      string `json:"npm"`
	Password string `json:"password"`
}

// KHSExtractionRequest represents the request body for KHS extraction.
type KHSExtractionRequest struct {
	NPM         string `json:"npm"`
	Password    string `json:"password"`
	TahunAjaran string `json:"tahun_ajaran"`
	Semester    string `json:"semester"`
}

// ExtractKRS handles POST /api/v1/lms/krs/extract
func (h *ExtractionHandler) ExtractKRS(c fiber.Ctx) error {
	var req KRSExtractionRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body: " + err.Error())
	}

	// Validate required fields
	if req.NPM == "" {
		return apperror.BadRequest("npm is required")
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	result, err := h.extractionSvc.ExtractKRS(req.NPM, req.Password)
	if err != nil {
		return apperror.Internal("KRS extraction failed", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KRS extracted successfully")
}

// ExtractKHS handles POST /api/v1/lms/khs/extract
func (h *ExtractionHandler) ExtractKHS(c fiber.Ctx) error {
	var req KHSExtractionRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body: " + err.Error())
	}

	// Validate required fields
	if req.NPM == "" {
		return apperror.BadRequest("npm is required")
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

	result, err := h.extractionSvc.ExtractKHS(req.NPM, req.Password, req.TahunAjaran, req.Semester)
	if err != nil {
		return apperror.Internal("KHS extraction failed", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KHS extracted successfully")
}

// GetKRS handles GET /api/v1/lms/krs/data/:npm
func (h *ExtractionHandler) GetKRS(c fiber.Ctx) error {
	npm := c.Params("npm")
	if npm == "" {
		return apperror.BadRequest("npm parameter is required")
	}

	// Validate NPM format (digits only)
	if !isValidNPM(npm) {
		return apperror.BadRequest("npm must contain only digits")
	}

	data, err := h.extractionSvc.GetKRSExtraction(npm)
	if err != nil {
		return apperror.NotFound("KRS extraction not found for npm: "+npm, err)
	}

	// Return raw JSON bytes in the response envelope
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return apperror.Internal("failed to parse cached extraction", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KRS data retrieved")
}

// GetKHS handles GET /api/v1/lms/khs/data/:npm/:tahun_ajaran/:semester
func (h *ExtractionHandler) GetKHS(c fiber.Ctx) error {
	npm := c.Params("npm")
	tahunAjaran := c.Params("tahun_ajaran")
	semester := c.Params("semester")

	if npm == "" || tahunAjaran == "" || semester == "" {
		return apperror.BadRequest("npm, tahun_ajaran, and semester parameters are required")
	}

	// Validate NPM format (digits only)
	if !isValidNPM(npm) {
		return apperror.BadRequest("npm must contain only digits")
	}

	// Validate semester format
	semester = strings.ToUpper(semester)
	if semester != "GANJIL" && semester != "GENAP" {
		return apperror.BadRequest("semester must be GANJIL or GENAP")
	}

	data, err := h.extractionSvc.GetKHSExtraction(npm, tahunAjaran, semester)
	if err != nil {
		return apperror.NotFound("KHS extraction not found", err)
	}

	// Return raw JSON bytes in the response envelope
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return apperror.Internal("failed to parse cached extraction", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KHS data retrieved")
}

// isValidNPM validates NPM format (digits only, 8-12 characters).
func isValidNPM(npm string) bool {
	if len(npm) < 8 || len(npm) > 12 {
		return false
	}
	for _, c := range npm {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
