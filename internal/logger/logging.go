package logger

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config controls logger behavior.
type Config struct {
	Verbose   bool   // pretty output for development
	Level     string // debug|info|warn|error
	Component string // component/service name
	Out       io.Writer
	TimeFunc  func() time.Time // injected for deterministic tests
}

// Setup configures the global zerolog logger and returns a component-scoped logger.
// It sets timestamps and ensures every log line includes "component".
func Setup(cfg Config) zerolog.Logger {
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.TimeFunc == nil {
		cfg.TimeFunc = time.Now
	}
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFunc = cfg.TimeFunc

	level := parseLevel(cfg.Level)

	var w io.Writer = cfg.Out
	if cfg.Verbose {
		// Pretty printing for development
		cw := zerolog.ConsoleWriter{
			Out:        cfg.Out,
			TimeFormat: time.RFC3339Nano,
			NoColor:    false,
		}
		w = cw
	}

	base := zerolog.New(w).Level(level).With().
		Timestamp().
		Str("component", cfg.Component).
		Logger()

	// Set global loggers too (optional but common)
	log.Logger = base

	return base
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zerolog.DebugLevel
	case "info", "":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		// safest default
		return zerolog.InfoLevel
	}
}

// NewBuffer is a helper for tests (optional).
func NewBuffer() *bytes.Buffer { return new(bytes.Buffer) }