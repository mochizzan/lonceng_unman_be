package response

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
)

// APIResponse is the standard JSON envelope returned by all endpoints.
type APIResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// extractTraceID pulls the request ID from the X-Request-Id response header
// (set by Fiber's requestid middleware) or from the request ID local.
func extractTraceID(c fiber.Ctx) string {
	if id := string(c.Response().Header.Peek("X-Request-Id")); id != "" {
		return id
	}
	if id := c.Locals("requestid"); id != nil {
		return fmt.Sprintf("%v", id)
	}
	return ""
}

// Success sends a success response with the given status code.
func Success(c fiber.Ctx, status int, data any, message string) error {
	return c.Status(status).JSON(APIResponse{
		Status:  "success",
		Data:    data,
		Message: message,
		TraceID: extractTraceID(c),
	})
}

// Error sends an error response with the given status code.
func Error(c fiber.Ctx, status int, message string, errors ...any) error {
	resp := APIResponse{
		Status:  "error",
		Message: message,
		TraceID: extractTraceID(c),
	}
	if len(errors) > 0 && errors[0] != nil {
		resp.Errors = errors[0]
	}
	return c.Status(status).JSON(resp)
}
