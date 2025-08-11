package run

import (
	"fmt"
	"os"
)

// LogLevel controls verbosity: Debug > Verbose > None.
type LogLevel int

const (
	LogNone LogLevel = iota
	LogVerbose
	LogDebug
)

var currentLogLevel = LogNone

// SetLogLevel sets the logging level for the run package.
func SetLogLevel(l LogLevel) { currentLogLevel = l }

func logVerbose(msg string, args ...any) {
	if currentLogLevel >= LogVerbose {
		fmt.Fprintf(os.Stderr, "[duck] %s\n", fmt.Sprintf(msg, args...))
	}
}

func logDebug(msg string, args ...any) {
	if currentLogLevel >= LogDebug {
		fmt.Fprintf(os.Stderr, "[duck][debug] %s\n", fmt.Sprintf(msg, args...))
	}
}
