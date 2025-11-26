package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log is the global logger instance
	Log *zap.Logger
	// Sugar is the global sugared logger instance
	Sugar *zap.SugaredLogger
)

// Init initializes the global logger with the specified level
// Supported levels: debug, info, warn, error
func Init(level string) error {
	zapLevel := parseLevel(level)

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	Sugar = Log.Sugar()
	return nil
}

// InitWithOptions initializes the logger with custom options
func InitWithOptions(level string, jsonFormat bool) error {
	zapLevel := parseLevel(level)

	encoding := "console"
	if jsonFormat {
		encoding = "json"
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if jsonFormat {
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      false,
		Encoding:         encoding,
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	Sugar = Log.Sugar()
	return nil
}

// Sync flushes any buffered log entries
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// parseLevel converts a string level to zapcore.Level
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// GetLevelFromEnv reads log level from environment variable
func GetLevelFromEnv() string {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		return "info"
	}
	return level
}

// IsJSONFormatFromEnv reads log format from environment variable
func IsJSONFormatFromEnv() bool {
	format := os.Getenv("LOG_FORMAT")
	return strings.ToLower(format) == "json"
}
