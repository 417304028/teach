package log

import (
	"log"
	"os"
	"strings"
)

type level int

const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelError
)

var (
	logger  *log.Logger
	minLevel level
)

func Init(levelStr string, jsonFormat bool) {
	minLevel = parseLevel(levelStr)
	var flags int
	if !jsonFormat {
		flags = log.LstdFlags
	}
	logger = log.New(os.Stdout, "", flags)
}

func parseLevel(levelStr string) level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return levelDebug
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}

func Info(msg string, args ...any) {
	if minLevel > levelInfo {
		return
	}
	logger.Printf("[INFO] "+msg, args...)
}

func Error(msg string, args ...any) {
	if minLevel > levelError {
		return
	}
	logger.Printf("[ERROR] "+msg, args...)
}

func Debug(msg string, args ...any) {
	if minLevel > levelDebug {
		return
	}
	logger.Printf("[DEBUG] "+msg, args...)
}

func Warn(msg string, args ...any) {
	if minLevel > levelWarn {
		return
	}
	logger.Printf("[WARN] "+msg, args...)
}

func Fatal(msg string, args ...any) {
	Error(msg, args...)
	os.Exit(1)
}
