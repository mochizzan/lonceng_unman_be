package response

import (
	"github.com/gofiber/fiber/v3"
)

// APIResponse is the standard JSON envelope returned by all endpoints.
type APIResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
	Errors  any    `json:"errors,omitempty"`
}

// Success sends a success response with the given status code.
func Success(c fiber.Ctx, status int, data any, message string) error {
	return c.Status(status).JSON(APIResponse{
		Status:  "success",
		Data:    data,
		Message: message,
	})
}

// Error sends an error response with the given status code.
func Error(c fiber.Ctx, status int, message string, errors ...any) error {
	resp := APIResponse{
		Status:  "error",
		Message: message,
	}
	if len(errors) > 0 && errors[0] != nil {
		resp.Errors = errors[0]
	}
	return c.Status(status).JSON(resp)
}
