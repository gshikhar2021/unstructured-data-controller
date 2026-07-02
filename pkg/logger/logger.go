package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const loggerKey contextKey = "logger_ctx"

// Init configures the default slog logger with JSON output to stdout.
func Init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

// NewContext returns a new context with a logger that includes request_id, tool_name, and username.
func NewContext(ctx context.Context, requestID, toolName, username string) context.Context {
	l := slog.Default().With(
		"request_id", requestID,
		"tool_name", toolName,
		"username", username,
	)
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext extracts the logger from context. Falls back to slog.Default() if not set.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
