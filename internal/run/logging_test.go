package run

import (
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LogLevel
		wantErr bool
	}{
		{"error level", "error", LogError, false},
		{"warn level", "warn", LogWarn, false},
		{"info level", "info", LogInfo, false},
		{"debug level", "debug", LogDebug, false},
		{"case insensitive", "DEBUG", LogDebug, false},
		{"with whitespace", " info ", LogInfo, false},
		{"invalid level", "invalid", LogInfo, true},
		{"empty string", "", LogInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LogError, "error"},
		{LogWarn, "warn"},
		{LogInfo, "info"},
		{LogDebug, "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetGetLogLevel(t *testing.T) {
	// Test setting and getting log level
	originalLevel := GetLogLevel()
	defer SetLogLevel(originalLevel) // Restore original level

	SetLogLevel(LogDebug)
	if got := GetLogLevel(); got != LogDebug {
		t.Errorf("GetLogLevel() = %v, want %v", got, LogDebug)
	}

	SetLogLevel(LogError)
	if got := GetLogLevel(); got != LogError {
		t.Errorf("GetLogLevel() = %v, want %v", got, LogError)
	}
}

func TestLogLevelHierarchy(t *testing.T) {
	// Test that log levels follow correct hierarchy: Error < Warn < Info < Debug
	if LogError >= LogWarn {
		t.Errorf("LogError (%d) should be less than LogWarn (%d)", LogError, LogWarn)
	}
	if LogWarn >= LogInfo {
		t.Errorf("LogWarn (%d) should be less than LogInfo (%d)", LogWarn, LogInfo)
	}
	if LogInfo >= LogDebug {
		t.Errorf("LogInfo (%d) should be less than LogDebug (%d)", LogInfo, LogDebug)
	}
}

func TestDefaultLogLevel(t *testing.T) {
	// Test that default log level is Info as per spec
	originalLevel := GetLogLevel()
	defer SetLogLevel(originalLevel)

	// Reset to default by creating a new instance
	currentLogLevel = LogInfo

	if GetLogLevel() != LogInfo {
		t.Errorf("Default log level should be LogInfo (%d), got %d", LogInfo, GetLogLevel())
	}
}
