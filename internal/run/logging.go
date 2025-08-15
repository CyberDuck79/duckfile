package run

import (
	"fmt"
	"os"
	"strings"
)

// LogLevel controls verbosity: Error (min) < Warn < Info (default) < Debug (max)
type LogLevel int

const (
	LogError LogLevel = iota
	LogWarn
	LogInfo // Default level as per spec
	LogDebug
)

var currentLogLevel = LogInfo // Changed default to Info as per spec

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogError:
		return "error"
	case LogWarn:
		return "warn"
	case LogInfo:
		return "info"
	case LogDebug:
		return "debug"
	default:
		return "info"
	}
}

// ParseLogLevel converts a string to a LogLevel
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error":
		return LogError, nil
	case "warn":
		return LogWarn, nil
	case "info":
		return LogInfo, nil
	case "debug":
		return LogDebug, nil
	default:
		return LogInfo, fmt.Errorf("invalid log level %q, must be one of: error, warn, info, debug", s)
	}
}

// SetLogLevel sets the logging level for the run package.
func SetLogLevel(l LogLevel) { currentLogLevel = l }

// GetLogLevel returns the current logging level
func GetLogLevel() LogLevel { return currentLogLevel }

//nolint:unused
func logError(msg string, args ...any) {
	if currentLogLevel >= LogError {
		fmt.Fprintf(os.Stderr, "[duck][error] %s\n", fmt.Sprintf(msg, args...))
	}
}

func logWarn(msg string, args ...any) {
	if currentLogLevel >= LogWarn {
		fmt.Fprintf(os.Stderr, "[duck][warn] %s\n", fmt.Sprintf(msg, args...))
	}
}

func logInfo(msg string, args ...any) {
	if currentLogLevel >= LogInfo {
		fmt.Fprintf(os.Stderr, "[duck] %s\n", fmt.Sprintf(msg, args...))
	}
}

func logDebug(msg string, args ...any) {
	if currentLogLevel >= LogDebug {
		fmt.Fprintf(os.Stderr, "[duck][debug] %s\n", fmt.Sprintf(msg, args...))
	}
}
