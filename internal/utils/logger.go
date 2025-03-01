package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log zerolog.Logger

// InitLogger initializes the global logger with console and file outputs
func InitLogger(level string, filePath string, maxSize, maxBackups, maxAge int, compress bool) error {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Configure rotating file logger
	fileLogger := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    maxSize,    // megabytes
		MaxBackups: maxBackups, // files
		MaxAge:     maxAge,     // days
		Compress:   compress,   // gzip old files
	}

	// Create multi-writer for both console and file
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	multi := zerolog.MultiLevelWriter(consoleWriter, fileLogger)

	// Initialize global logger
	log = zerolog.New(multi).With().Timestamp().Logger().Level(logLevel)

	log.Info().
		Str("level", level).
		Str("file", filePath).
		Msg("Logger initialized")

	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *zerolog.Logger {
	return &log
}

// Debug logs a debug message
func Debug(msg string, fields ...interface{}) {
	event := log.Debug()
	addFields(event, fields...)
	event.Msg(msg)
}

// Info logs an info message
func Info(msg string, fields ...interface{}) {
	event := log.Info()
	addFields(event, fields...)
	event.Msg(msg)
}

// Warn logs a warning message
func Warn(msg string, fields ...interface{}) {
	event := log.Warn()
	addFields(event, fields...)
	event.Msg(msg)
}

// Error logs an error message
func Error(msg string, err error, fields ...interface{}) {
	event := log.Error().Err(err)
	addFields(event, fields...)
	event.Msg(msg)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, err error, fields ...interface{}) {
	event := log.Fatal().Err(err)
	addFields(event, fields...)
	event.Msg(msg)
}

// addFields adds key-value pairs to the log event
func addFields(event *zerolog.Event, fields ...interface{}) {
	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			break
		}
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		value := fields[i+1]
		event.Interface(key, value)
	}
}

// Timer returns elapsed time since start in a human-readable format
func Timer(start time.Time) string {
	elapsed := time.Since(start)
	if elapsed < time.Second {
		return fmt.Sprintf("%dms", elapsed.Milliseconds())
	}
	if elapsed < time.Minute {
		return fmt.Sprintf("%.1fs", elapsed.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
}

// Close ensures all logs are written before shutdown
func Close() error {
	// No need to close zerolog logger as it writes immediately
	return nil
}
