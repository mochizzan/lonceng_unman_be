package main

import (
	"log/slog"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/infrastructure/fibererror"
	"lonceng_unman_be/internal/infrastructure/logger"
	"lonceng_unman_be/internal/infrastructure/middleware"
	"lonceng_unman_be/internal/infrastructure/session"
	"lonceng_unman_be/internal/interfaces/http/handler"
	"lonceng_unman_be/internal/interfaces/http/router"

	"github.com/gofiber/fiber/v3"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Initialize structured logger and set as global default
	log := logger.New(cfg.App.Env)
	slog.SetDefault(log)

	// Create Fiber app with custom error handler
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: fibererror.New(),
	})

	// Register middleware (recover → requestid → logger → cors)
	middleware.Register(app, cfg.CORS)

	// Wire session manager (in-memory cache with TTL)
	sessionMgr := session.NewManager(cfg)
	defer sessionMgr.CloseAll()

	// Wire application services
	healthService := service.NewHealthService(cfg.App)
	lmsService := service.NewLMSService(cfg, sessionMgr)
	docService := service.NewLMSDocumentService(cfg, sessionMgr)

	// Wire HTTP handlers
	healthHandler := handler.NewHealthHandler(healthService)
	lmsHandler := handler.NewLMSHandler(lmsService)
	docHandler := handler.NewDocumentHandler(docService)

	// Register routes
	router.Setup(app, healthHandler, lmsHandler, docHandler)

	// Start server
	log.Info(
		"starting server",
		"app", cfg.App.Name,
		"addr", cfg.Addr(),
		"env", cfg.App.Env,
	)
	if err := app.Listen(cfg.Addr()); err != nil {
		log.Error("server error", "err", err)
	}
}
