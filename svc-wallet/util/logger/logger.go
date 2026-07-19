package logger

import (
	"log/slog"
	"os"
)

// InitLogger configures the global slog instance to output JSON.
// You will call this exactly once in your cmd/main.go.
func InitLogger() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	// Use NewJSONHandler to satisfy the "Structured logs" deliverable
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	// Set this as the global default so you can just call slog.Info() anywhere
	slog.SetDefault(logger)
}
