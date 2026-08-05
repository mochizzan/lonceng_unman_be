package middleware

import (
	"strings"

	"lonceng_unman_be/internal/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// Register attaches the standard middleware stack to the Fiber app.
func Register(app *fiber.App, corsCfg config.CORSConfig) {
	// Recover from panics
	app.Use(recover.New())

	// Request logger
	app.Use(logger.New())

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Split(corsCfg.AllowOrigins, ","),
		AllowMethods: strings.Split(corsCfg.AllowMethods, ","),
		AllowHeaders: strings.Split(corsCfg.AllowHeaders, ","),
	}))
}
