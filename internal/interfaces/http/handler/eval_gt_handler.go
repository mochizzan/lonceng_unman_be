package handler

import (
	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// EvalGTHandler handles JSON POST for saving ground truth.
type EvalGTHandler struct {
	evalSvc interface {
		SaveKRS(npm string, filename string, req entity.SaveGTRequest) error
		SaveKHS(npm string, filename string, req entity.SaveGTRequest) error
	}
}

// NewEvalGTHandler creates a new eval GT handler.
func NewEvalGTHandler(evalSvc interface {
	SaveKRS(npm string, filename string, req entity.SaveGTRequest) error
	SaveKHS(npm string, filename string, req entity.SaveGTRequest) error
},
) *EvalGTHandler {
	return &EvalGTHandler{evalSvc: evalSvc}
}

// SaveKRS handles POST /api/v1/eval/:npm/krs/:file
func (h *EvalGTHandler) SaveKRS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")

	var req entity.SaveGTRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := h.evalSvc.SaveKRS(npm, file, req); err != nil {
		return err
	}

	return response.Success(c, fiber.StatusOK, map[string]interface{}{
		"npm":      npm,
		"doc_type": "krs",
		"file":     file,
	}, "GT saved successfully")
}

// SaveKHS handles POST /api/v1/eval/:npm/khs/:file
func (h *EvalGTHandler) SaveKHS(c fiber.Ctx) error {
	npm := c.Params("npm")
	file := c.Params("file")

	var req entity.SaveGTRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := h.evalSvc.SaveKHS(npm, file, req); err != nil {
		return err
	}

	return response.Success(c, fiber.StatusOK, map[string]interface{}{
		"npm":      npm,
		"doc_type": "khs",
		"file":     file,
	}, "GT saved successfully")
}
