package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"lonceng_unman_be/internal/infrastructure/logger"
)

// captureLogger creates a logger writing to a buffer so we can inspect output.
func captureLogger(env string) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	l := logger.New(env, &buf)
	return l, &buf
}

func TestNew_DevelopmentReturnsTextHandler(t *testing.T) {
	l, buf := captureLogger("development")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	handler := l.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if _, ok := handler.(*slog.TextHandler); !ok {
		t.Errorf("expected *slog.TextHandler in development, got %T", handler)
	}

	// Verify DEBUG level is active — log at Debug and check output exists
	l.Debug("test debug message")
	if buf.Len() == 0 {
		t.Error("expected debug output in development mode, got nothing")
	}

	// Verify output is text (key=value pairs), not JSON
	output := buf.String()
	if strings.HasPrefix(output, "{") {
		t.Errorf("expected text format in development, got JSON-like output: %s", output)
	}
}

func TestNew_ProductionReturnsJSONHandler(t *testing.T) {
	l, buf := captureLogger("production")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	handler := l.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if _, ok := handler.(*slog.JSONHandler); !ok {
		t.Errorf("expected *slog.JSONHandler in production, got %T", handler)
	}

	// Verify INFO level — log at Debug (should be suppressed) then Info (should appear)
	l.Debug("this should not appear")
	if buf.Len() != 0 {
		t.Error("expected no debug output in production mode")
	}

	l.Info("test info message")
	if buf.Len() == 0 {
		t.Error("expected info output in production mode")
	}

	// Verify output is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Errorf("expected valid JSON in production output, got error: %v", err)
	}
}

func TestNew_AllEnvironmentsReturnLogger(t *testing.T) {
	envs := []string{"development", "staging", "production", "testing"}
	for _, env := range envs {
		l, buf := captureLogger(env)
		if l == nil {
			t.Errorf("expected non-nil logger for env %q", env)
		}
		// Verify logger actually works by logging something
		l.Info("test", "env", env)
		if buf.Len() == 0 {
			t.Errorf("expected output from logger for env %q", env)
		}
	}
}
