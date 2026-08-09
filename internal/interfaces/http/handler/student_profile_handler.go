package handler

import (
	"fmt"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

// StudentProfileHandler handles HTTP requests for student profile operations.
type StudentProfileHandler struct {
	profileSvc service.StudentProfileService
}

// NewStudentProfileHandler creates a StudentProfileHandler.
func NewStudentProfileHandler(profileSvc service.StudentProfileService) *StudentProfileHandler {
	return &StudentProfileHandler{profileSvc: profileSvc}
}

// Scrape handles POST /api/v1/lms/student-profile
func (h *StudentProfileHandler) Scrape(c fiber.Ctx) error {
	var req entity.StudentProfileRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := validateNPM(req.NPM); err != nil {
		return err
	}

	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	result, err := h.profileSvc.Scrape(req)
	if err != nil {
		return apperror.Internal("failed to scrape student profile", err)
	}

	return response.Success(c, fiber.StatusOK, result, "Student profile downloaded")
}

// Get handles POST /api/v1/lms/student-profile/data
func (h *StudentProfileHandler) Get(c fiber.Ctx) error {
	var req entity.StudentProfileRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := validateNPM(req.NPM); err != nil {
		return err
	}

	profile, err := h.profileSvc.Get(req)
	if err != nil {
		return apperror.NotFound("student profile not found. Use POST /api/v1/lms/student-profile to fetch", err)
	}

	return response.Success(c, fiber.StatusOK, profile, "Student profile retrieved")
}

// GetPhoto handles POST /api/v1/lms/student-profile/photo
// Returns the photo as binary or 204 No Content if not found.
func (h *StudentProfileHandler) GetPhoto(c fiber.Ctx) error {
	var req entity.StudentProfileRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := validateNPM(req.NPM); err != nil {
		return err
	}

	if req.Password == "" {
		return apperror.BadRequest("password is required")
	}

	photoData, contentType, err := h.profileSvc.GetPhoto(req)
	if err != nil {
		return apperror.Internal("failed to get student photo", err)
	}

	// No photo found
	if photoData == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}

	// Set headers for streaming + caching
	c.Set("Content-Type", contentType)
	c.Set("Content-Length", fmt.Sprintf("%d", len(photoData)))
	c.Set("Cache-Control", "max-age=900") // 15 minutes

	return c.Send(photoData)
}
