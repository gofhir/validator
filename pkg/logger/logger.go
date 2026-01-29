// Package logger provides a simple logging interface for the validator.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents the logging level.
type Level int

// Log levels.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelNone
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return ""
	}
}

// Logger provides logging functionality.
type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	prefix string
}

var defaultLogger = &Logger{
	level:  LevelInfo,
	output: os.Stderr,
	prefix: "gofhir-validator",
}

// Default returns the default logger.
func Default() *Logger {
	return defaultLogger
}

// SetDefault sets the default logger.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// New creates a new logger.
func New(output io.Writer, level Level) *Logger {
	return &Logger{
		level:  level,
		output: output,
		prefix: "gofhir-validator",
	}
}

// SetLevel sets the logging level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

func (l *Logger) log(level Level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(l.output, "[%s] %s [%s] %s\n", timestamp, l.prefix, level.String(), msg)
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

// Info logs an info message.
func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

// Package-level convenience functions.

// Debug logs a debug message using the default logger.
func Debug(format string, args ...any) {
	defaultLogger.Debug(format, args...)
}

// Info logs an info message using the default logger.
func Info(format string, args ...any) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger.
func Warn(format string, args ...any) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger.
func Error(format string, args ...any) {
	defaultLogger.Error(format, args...)
}

// SetLevel sets the level of the default logger.
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// SetOutput sets the output of the default logger.
func SetOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// Disable disables all logging.
func Disable() {
	defaultLogger.SetLevel(LevelNone)
}
