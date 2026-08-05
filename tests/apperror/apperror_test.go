package apperror

import (
	"errors"
	"testing"

	"lonceng_unman_be/internal/apperror"

	"github.com/gofiber/fiber/v3"
)

func TestNotFound(t *testing.T) {
	cause := errors.New("db: no rows")
	err := apperror.NotFound("user not found", cause)

	if err.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404, got %d", err.StatusCode)
	}
	if err.Error() != "user not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "user not found")
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is should find the internal cause")
	}
}

func TestBadRequest(t *testing.T) {
	err := apperror.BadRequest("invalid input")
	if err.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", err.StatusCode)
	}
	if err.Error() != "invalid input" {
		t.Errorf("Error() = %q, want %q", err.Error(), "invalid input")
	}
}

func TestUnauthorized(t *testing.T) {
	err := apperror.Unauthorized("missing token")
	if err.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", err.StatusCode)
	}
}

func TestForbidden(t *testing.T) {
	err := apperror.Forbidden("insufficient permissions")
	if err.StatusCode != fiber.StatusForbidden {
		t.Errorf("expected status 403, got %d", err.StatusCode)
	}
}

func TestInternal(t *testing.T) {
	cause := errors.New("connection refused")
	err := apperror.Internal("service unavailable", cause)

	if err.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", err.StatusCode)
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is should find the internal cause")
	}
	if err.Unwrap() != cause {
		t.Fatal("Unwrap should return the internal cause")
	}
}

func TestErrorsAs_AppError(t *testing.T) {
	err := apperror.NotFound("not found", nil)
	var target *apperror.AppError
	if !errors.As(err, &target) {
		t.Fatal("errors.As should extract *AppError")
	}
	if target.StatusCode != fiber.StatusNotFound {
		t.Errorf("extracted AppError has wrong status: %d", target.StatusCode)
	}
}
