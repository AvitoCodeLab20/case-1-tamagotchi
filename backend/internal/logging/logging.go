package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is the application logger type. It aliases slog.Logger so callers can
// depend on this package instead of importing log/slog directly.
type Logger = slog.Logger

// New builds the application logger. It writes structured JSON to stdout and
// honours the LOG_LEVEL environment variable (debug, info, warn, error);
// the default level is info.
func New() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelFromEnv(),
	})

	return slog.New(handler)
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
