package handler

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

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

// ExtractionResponse represents the response for extraction endpoints.
// Returns success/error status only, not the extracted data.
type ExtractionResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	NPM       string    `json:"npm"`
	Timestamp time.Time `json:"timestamp"`
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
// Always re-extracts and overwrites existing cache.
func (h *ExtractionHandler) ExtractKRS(c fiber.Ctx) error {
	var req KRSExtractionRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body: " + err.Error())
	}

	// Validate required fields
	if err := validateNPM(req.NPM); err != nil {
		return err
	}
	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	result, err := h.extractionSvc.ExtractKRS(req.NPM, req.Password)
	if err != nil {
		if errors.Is(err, apperror.ErrPDFNotFound) {
			return apperror.NotFound("KRS PDF not found for npm: "+req.NPM, err)
		}
		return apperror.Internal("KRS extraction failed", err)
	}

	resp := ExtractionResponse{
		Success:   result.Success,
		Message:   result.Message,
		NPM:       result.NPM,
		Timestamp: result.Timestamp,
	}

	return response.Success(c, fiber.StatusOK, resp, "KRS extracted successfully")
}

// ExtractKHS handles POST /api/v1/lms/khs/extract
// Always re-extracts and overwrites existing cache.
func (h *ExtractionHandler) ExtractKHS(c fiber.Ctx) error {
	var req KHSExtractionRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.BadRequest("invalid request body: " + err.Error())
	}

	// Validate required fields
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

	result, err := h.extractionSvc.ExtractKHS(req.NPM, req.Password, req.TahunAjaran, req.Semester)
	if err != nil {
		if errors.Is(err, apperror.ErrPDFNotFound) {
			return apperror.NotFound("KHS PDF not found", err)
		}
		return apperror.Internal("KHS extraction failed", err)
	}

	resp := ExtractionResponse{
		Success:   result.Success,
		Message:   result.Message,
		NPM:       result.NPM,
		Timestamp: result.Timestamp,
	}

	return response.Success(c, fiber.StatusOK, resp, "KHS extracted successfully")
}

// GetKRS handles GET /api/v1/lms/krs/data/:npm
func (h *ExtractionHandler) GetKRS(c fiber.Ctx) error {
	npm := c.Params("npm")
	if err := validateNPM(npm); err != nil {
		return err
	}

	data, err := h.extractionSvc.GetKRSExtraction(npm)
	if err != nil {
		if errors.Is(err, apperror.ErrExtractionNotFound) || errors.Is(err, os.ErrNotExist) {
			return apperror.NotFound("KRS extraction not found for npm: "+npm, err)
		}
		if errors.Is(err, os.ErrPermission) {
			return apperror.Forbidden("permission denied accessing KRS extraction")
		}
		return apperror.Internal("failed to retrieve KRS extraction", err)
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

	if tahunAjaran == "" || semester == "" {
		return apperror.BadRequest("tahun_ajaran and semester parameters are required")
	}

	if err := validateNPM(npm); err != nil {
		return err
	}

	// Validate semester format
	semester = strings.ToUpper(semester)
	if semester != "GANJIL" && semester != "GENAP" {
		return apperror.BadRequest("semester must be GANJIL or GENAP")
	}

	data, err := h.extractionSvc.GetKHSExtraction(npm, tahunAjaran, semester)
	if err != nil {
		if errors.Is(err, apperror.ErrExtractionNotFound) || errors.Is(err, os.ErrNotExist) {
			return apperror.NotFound("KHS extraction not found", err)
		}
		if errors.Is(err, os.ErrPermission) {
			return apperror.Forbidden("permission denied accessing KHS extraction")
		}
		return apperror.Internal("failed to retrieve KHS extraction", err)
	}

	// Return raw JSON bytes in the response envelope
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return apperror.Internal("failed to parse cached extraction", err)
	}

	return response.Success(c, fiber.StatusOK, result, "KHS data retrieved")
}
