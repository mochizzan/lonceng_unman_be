package middleware

import (
	"strings"

	"lonceng_unman_be/internal/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// Register attaches the standard middleware stack to the Fiber app.
// Order matters: recover must be first, then requestid, then logger.
func Register(app *fiber.App, corsCfg config.CORSConfig) {
	// Recover from panics (must be first)
	app.Use(recover.New())

	// Assign a unique request ID to every request
	app.Use(requestid.New())

	// Request logger (runs after requestid is set, so logs include the ID)
	app.Use(logger.New())

	// Gzip compression (after logger, before CORS)
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Split(corsCfg.AllowOrigins, ","),
		AllowMethods: strings.Split(corsCfg.AllowMethods, ","),
		AllowHeaders: strings.Split(corsCfg.AllowHeaders, ","),
	}))
}
