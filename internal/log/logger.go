package log

import (
	"log/slog"
	"os"
)

var defaultLogger *slog.Logger

func Init(level string, jsonFormat bool) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func With(args ...any) *slog.Logger {
	return defaultLogger.With(args...)
}
