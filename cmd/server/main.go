package main

import (
	"log/slog"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/infrastructure/extractor"
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

	// Configure extractor limits from config
	extractor.SetMaxPDFSize(cfg.App.MaxPDFSize)

	// Create Fiber app with custom error handler
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: fibererror.New(),
		BodyLimit:    int(cfg.App.MaxBodySize),
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
	parser := &parserAdapter{}
	cache := extractor.NewCacheManager(cfg.App.ExtractDir)
	extractionService := service.NewExtractionService(cfg.App.DownloadDir, cfg.App.ExtractDir, parser, cache, sessionMgr)

	// Wire HTTP handlers
	healthHandler := handler.NewHealthHandler(healthService)
	lmsHandler := handler.NewLMSHandler(lmsService)
	docHandler := handler.NewDocumentHandler(docService)
	extractionHandler := handler.NewExtractionHandler(extractionService)

	// Register routes
	router.Setup(app, healthHandler, lmsHandler, docHandler, extractionHandler)

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

type parserAdapter struct{}

func (a *parserAdapter) ParseKRS(path string, npm string) (*entity.KRSExtraction, error) {
	return extractor.ParseKRS(path, npm)
}

func (a *parserAdapter) ParseKHS(path string, npm string, tahunAjaran string, semester string) (*entity.KHSExtraction, error) {
	return extractor.ParseKHS(path, npm, tahunAjaran, semester)
}

func (a *parserAdapter) MarshalToJSON(v interface{}) ([]byte, error) {
	return extractor.MarshalJSON(v)
}
