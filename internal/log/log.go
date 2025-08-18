// Package log provides centralized logging functionality for all duckfile components.
// It supports hierarchical log levels (Error < Warn < Info < Debug) and consistent
// formatting across the entire application.
package log

import (
	"fmt"
	"os"
	"strings"
)

// Level controls verbosity: Error (min) < Warn < Info (default) < Debug (max)
type Level int

const (
	Error Level = iota
	Warn
	Info // Default level as per spec
	Debug
)

var currentLevel = Info // Default to Info as per spec

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case Error:
		return "error"
	case Warn:
		return "warn"
	case Info:
		return "info"
	case Debug:
		return "debug"
	default:
		return "info"
	}
}

// ParseLevel converts a string to a Level
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error":
		return Error, nil
	case "warn":
		return Warn, nil
	case "info":
		return Info, nil
	case "debug":
		return Debug, nil
	default:
		return Info, fmt.Errorf("invalid log level %q, must be one of: error, warn, info, debug", s)
	}
}

// SetLevel sets the logging level globally.
func SetLevel(l Level) { currentLevel = l }

// GetLevel returns the current logging level
func GetLevel() Level { return currentLevel }

// Errorf logs an error message if the current level is Error or higher.
// Error messages are prefixed with [duck][error].
func Errorf(msg string, args ...any) {
	if currentLevel >= Error {
		fmt.Fprintf(os.Stderr, "[duck][error] %s\n", fmt.Sprintf(msg, args...))
	}
}

// Warnf logs a warning message if the current level is Warn or higher.
// Warning messages are prefixed with [duck][warn].
func Warnf(msg string, args ...any) {
	if currentLevel >= Warn {
		fmt.Fprintf(os.Stderr, "[duck][warn] %s\n", fmt.Sprintf(msg, args...))
	}
}

// Infof logs an info message if the current level is Info or higher.
// Info messages are prefixed with [duck].
func Infof(msg string, args ...any) {
	if currentLevel >= Info {
		fmt.Fprintf(os.Stderr, "[duck] %s\n", fmt.Sprintf(msg, args...))
	}
}

// Debugf logs a debug message if the current level is Debug or higher.
// Debug messages are prefixed with [duck][debug].
func Debugf(msg string, args ...any) {
	if currentLevel >= Debug {
		fmt.Fprintf(os.Stderr, "[duck][debug] %s\n", fmt.Sprintf(msg, args...))
	}
}

// IsLevelEnabled returns true if the given level would be logged at the current log level.
// This can be used to avoid expensive operations when logging is disabled.
func IsLevelEnabled(l Level) bool {
	return currentLevel >= l
}
