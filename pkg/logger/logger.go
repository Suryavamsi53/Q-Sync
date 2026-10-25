package logger

import (
	"log/slog"
	"os"
)

// InitLogger initializes a production-ready structured JSON logger
func InitLogger(serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler).With(
		slog.String("service", serviceName),
	)
	slog.SetDefault(logger)
	return logger
}
