package fibererror_test

import (
	"errors"
	"net/http"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/infrastructure/fibererror"

	"github.com/gofiber/fiber/v3"
)

func TestErrorHandler_AppError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})

	app.Get("/test", func(c fiber.Ctx) error {
		return apperror.BadRequest("name is required")
	})

	req, err := http.NewRequest(fiber.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestErrorHandler_UnhandledError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: fibererror.New(),
	})

	app.Get("/test", func(c fiber.Ctx) error {
		return errors.New("something broke")
	})

	req, err := http.NewRequest(fiber.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}
