package logger

import (
	"log"
	"os"
)

// Level defines the severity of a log message.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger is a simple, structured logger.
type Logger struct {
	level Level
	l     *log.Logger
}

// New creates a new Logger with the specified minimum log level.
func New(level Level) *Logger {
	return &Logger{
		level: level,
		l:     log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Debug logs a message at the debug level.
func (l *Logger) Debug(msg string, args ...any) {
	if l.level <= LevelDebug {
		l.l.Printf("[DEBUG] "+msg, args...)
	}
}

// Info logs a message at the info level.
func (l *Logger) Info(msg string, args ...any) {
	if l.level <= LevelInfo {
		l.l.Printf("[INFO] "+msg, args...)
	}
}

// Warn logs a message at the warn level.
func (l *Logger) Warn(msg string, args ...any) {
	if l.level <= LevelWarn {
		l.l.Printf("[WARN] "+msg, args...)
	}
}

// Error logs a message at the error level.
func (l *Logger) Error(msg string, args ...any) {
	if l.level <= LevelError {
		l.l.Printf("[ERROR] "+msg, args...)
	}
}
