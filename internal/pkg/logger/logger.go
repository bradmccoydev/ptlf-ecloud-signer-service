// Package logger provides a structured JSON logger built on go.uber.org/zap.
package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a new zap.Logger configured for JSON output with the given
// service metadata baked in as default fields.
//
// The level parameter accepts standard strings: "debug", "info", "warn", "error".
// An invalid level string returns an error.
func New(serviceName, version, environment, level string) (*zap.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(lvl),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		InitialFields: map[string]interface{}{
			"service":     serviceName,
			"version":     version,
			"environment": environment,
		},
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return logger, nil
}

// parseLevel converts a level string to the corresponding zapcore.Level.
func parseLevel(level string) (zapcore.Level, error) {
	if level == "" {
		return 0, fmt.Errorf("invalid log level: must not be empty")
	}
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return lvl, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	return lvl, nil
}
