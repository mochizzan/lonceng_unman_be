package fibererror

import (
	"errors"
	"log/slog"

	"lonceng_unman_be/internal/apperror"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// apiResponse is the standard error JSON envelope returned to clients.
type apiResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// New returns a Fiber ErrorHandler that:
//   - Extracts apperror.AppError or *fiber.Error for controlled status codes/messages
//   - Logs the full internal error server-side via slog (never exposed to client)
//   - Sends a sanitized JSON response to the client
//   - Includes the request ID as trace_id when available
func New() fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		message := "An unexpected error occurred"

		var appErr *apperror.AppError
		var fiberErr *fiber.Error

		switch {
		case errors.As(err, &appErr):
			code = appErr.StatusCode
			message = appErr.PublicMsg
			if appErr.Internal != nil {
				slog.Error(
					"request error",
					"method", c.Method(),
					"path", c.Path(),
					"status", code,
					"internal", appErr.Internal,
				)
			}
		case errors.As(err, &fiberErr):
			code = fiberErr.Code
			message = fiberErr.Message
		default:
			slog.Error(
				"unhandled error",
				"method", c.Method(),
				"path", c.Path(),
				"err", err,
			)
		}

		traceID := requestid.FromContext(c)

		return c.Status(code).JSON(apiResponse{
			Status:  "error",
			Message: message,
			TraceID: traceID,
		})
	}
}
