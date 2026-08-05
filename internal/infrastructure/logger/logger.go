package logger

import (
	"io"
	"log/slog"
	"os"
)

// New creates a structured logger configured for the given environment.
// In development it logs at DEBUG level with human-readable output.
// In production it logs at INFO level as JSON.
// If w is nil, output goes to os.Stdout.
func New(env string, w ...io.Writer) *slog.Logger {
	var out io.Writer = os.Stdout
	if len(w) > 0 && w[0] != nil {
		out = w[0]
	}

	var handler slog.Handler

	if env == "development" {
		handler = slog.NewTextHandler(out, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	} else {
		handler = slog.NewJSONHandler(out, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return slog.New(handler)
}
