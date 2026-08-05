package apperror

import "github.com/gofiber/fiber/v3"

// AppError is a structured error that separates public messages (safe to send
// to clients) from internal details (logged server-side only, never exposed).
//
// Usage in handlers:
//
//	return apperror.NotFound("user not found", err)
//	return apperror.BadRequest("name is required")
//	return apperror.Internal("database connection failed", err)
type AppError struct {
	StatusCode int
	PublicMsg  string
	Internal   error
}

// Error implements the error interface. Returns the public message.
func (e *AppError) Error() string {
	return e.PublicMsg
}

// Unwrap allows errors.Is / errors.As to reach the internal error.
func (e *AppError) Unwrap() error {
	return e.Internal
}

// StatusCodeInt returns the HTTP status code as an int (for Fiber).
func (e *AppError) StatusCodeInt() int {
	return e.StatusCode
}

// --- Constructors ---

// NotFound creates a 404 error.
func NotFound(msg string, internal error) *AppError {
	return &AppError{
		StatusCode: fiber.StatusNotFound,
		PublicMsg:  msg,
		Internal:   internal,
	}
}

// BadRequest creates a 400 error.
func BadRequest(msg string) *AppError {
	return &AppError{
		StatusCode: fiber.StatusBadRequest,
		PublicMsg:  msg,
	}
}

// Unauthorized creates a 401 error.
func Unauthorized(msg string) *AppError {
	return &AppError{
		StatusCode: fiber.StatusUnauthorized,
		PublicMsg:  msg,
	}
}

// Forbidden creates a 403 error.
func Forbidden(msg string) *AppError {
	return &AppError{
		StatusCode: fiber.StatusForbidden,
		PublicMsg:  msg,
	}
}

// Internal creates a 500 error with an internal cause that is logged but not exposed.
func Internal(msg string, internal error) *AppError {
	return &AppError{
		StatusCode: fiber.StatusInternalServerError,
		PublicMsg:  msg,
		Internal:   internal,
	}
}
