package main

import (
	"log"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/middleware"
	"lonceng_unman_be/internal/interfaces/http/handler"
	"lonceng_unman_be/internal/interfaces/http/router"

	"github.com/gofiber/fiber/v3"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: cfg.App.Name,
	})

	// Register middleware
	middleware.Register(app, cfg.CORS)

	// Wire application services
	healthService := service.NewHealthService(cfg.App)

	// Wire HTTP handlers
	healthHandler := handler.NewHealthHandler(healthService)

	// Register routes
	router.Setup(app, healthHandler)

	// Start server
	log.Printf("Starting %s on %s [%s]", cfg.App.Name, cfg.Addr(), cfg.App.Env)
	if err := app.Listen(cfg.Addr()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
