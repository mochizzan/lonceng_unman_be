package fibererror

import (
	"errors"
	"log/slog"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/interfaces/http/response"

	"github.com/gofiber/fiber/v3"
)

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
			slog.Warn(
				"fiber error",
				"method", c.Method(),
				"path", c.Path(),
				"status", code,
				"message", message,
			)
		default:
			slog.Error(
				"unhandled error",
				"method", c.Method(),
				"path", c.Path(),
				"err", err,
			)
		}

		return response.Error(c, code, message, nil)
	}
}
