package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"marry-me/internal/config"
)

// Setup configures the global logger based on configuration
func Setup(cfg *config.Config) {
	// Set log level
	level := parseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)

	// Configure output
	var output io.Writer = os.Stdout

	if !cfg.JSONLogs {
		// Pretty console output
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	// Enable verbose logging if requested
	if cfg.Verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

// parseLevel converts a string to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// Event creates a logger with event context
func Event(eventID string, eventType string) zerolog.Logger {
	return log.With().
		Str("event_id", eventID).
		Str("event_type", eventType).
		Logger()
}

// Worker creates a logger with worker context
func Worker(workerID string, team string) zerolog.Logger {
	return log.With().
		Str("worker_id", workerID).
		Str("team", team).
		Logger()
}

// Team creates a logger with team context
func Team(teamName string) zerolog.Logger {
	return log.With().
		Str("team", teamName).
		Logger()
}

// Component creates a logger with component context
func Component(name string) zerolog.Logger {
	return log.With().
		Str("component", name).
		Logger()
}
