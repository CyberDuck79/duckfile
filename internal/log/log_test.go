package log

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		hasError bool
	}{
		{"error", Error, false},
		{"warn", Warn, false},
		{"info", Info, false},
		{"debug", Debug, false},
		{"ERROR", Error, false}, // Case insensitive
		{"WARN", Warn, false},
		{"INFO", Info, false},
		{"DEBUG", Debug, false},
		{" info ", Info, false}, // Whitespace handling
		{"invalid", Info, true}, // Invalid should return error
		{"", Info, true},        // Empty should return error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseLevel(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("ParseLevel(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseLevel(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{Error, "error"},
		{Warn, "warn"},
		{Info, "info"},
		{Debug, "debug"},
		{Level(999), "info"}, // Unknown level defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Level.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSetGetLevel(t *testing.T) {
	// Save original level
	originalLevel := GetLevel()
	defer SetLevel(originalLevel)

	tests := []Level{Error, Warn, Info, Debug}

	for _, level := range tests {
		SetLevel(level)
		if GetLevel() != level {
			t.Errorf("After SetLevel(%v), GetLevel() = %v, want %v", level, GetLevel(), level)
		}
	}
}

func TestLogLevelHierarchy(t *testing.T) {
	// Save original level and stderr
	originalLevel := GetLevel()
	originalStderr := os.Stderr
	defer func() {
		SetLevel(originalLevel)
		os.Stderr = originalStderr
	}()

	// Test each level
	levels := []struct {
		level Level
		name  string
	}{
		{Error, "Error"},
		{Warn, "Warn"},
		{Info, "Info"},
		{Debug, "Debug"},
	}

	for _, currentLevel := range levels {
		t.Run("Level_"+currentLevel.name, func(t *testing.T) {
			SetLevel(currentLevel.level)

			// Capture stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Test all logging functions
			Errorf("error message")
			Warnf("warn message")
			Infof("info message")
			Debugf("debug message")

			w.Close()
			os.Stderr = originalStderr

			// Read captured output
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Count lines - should match expected hierarchy
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if output == "" {
				lines = []string{} // Empty output
			}

			expectedLines := int(currentLevel.level) + 1 // Levels 0-3 map to 1-4 lines
			if len(lines) != expectedLines {
				t.Errorf("At level %s, expected %d log lines, got %d: %v",
					currentLevel.name, expectedLines, len(lines), lines)
			}
		})
	}
}

func TestDefaultLevel(t *testing.T) {
	// Create a new log state (simulate fresh start)
	originalLevel := GetLevel()
	defer SetLevel(originalLevel)

	// Reset to package default
	currentLevel = Info

	if GetLevel() != Info {
		t.Errorf("Default log level should be Info, got %v", GetLevel())
	}
}

func TestIsLevelEnabled(t *testing.T) {
	originalLevel := GetLevel()
	defer SetLevel(originalLevel)

	tests := []struct {
		currentLevel Level
		testLevel    Level
		expected     bool
	}{
		{Error, Error, true},
		{Error, Warn, false},
		{Error, Info, false},
		{Error, Debug, false},
		{Warn, Error, true},
		{Warn, Warn, true},
		{Warn, Info, false},
		{Warn, Debug, false},
		{Info, Error, true},
		{Info, Warn, true},
		{Info, Info, true},
		{Info, Debug, false},
		{Debug, Error, true},
		{Debug, Warn, true},
		{Debug, Info, true},
		{Debug, Debug, true},
	}

	for _, tt := range tests {
		SetLevel(tt.currentLevel)
		result := IsLevelEnabled(tt.testLevel)
		if result != tt.expected {
			t.Errorf("At level %s, IsLevelEnabled(%s) = %v, want %v",
				tt.currentLevel, tt.testLevel, result, tt.expected)
		}
	}
}

func TestLogFormatting(t *testing.T) {
	originalLevel := GetLevel()
	originalStderr := os.Stderr
	defer func() {
		SetLevel(originalLevel)
		os.Stderr = originalStderr
	}()

	SetLevel(Debug) // Enable all logging

	tests := []struct {
		logFunc func(string, ...any)
		message string
		args    []any
		prefix  string
	}{
		{Errorf, "test error %s", []any{"arg"}, "[duck][error]"},
		{Warnf, "test warn %d", []any{42}, "[duck][warn]"},
		{Infof, "test info %v", []any{true}, "[duck]"},
		{Debugf, "test debug %s", []any{"debug"}, "[duck][debug]"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			// Capture stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			tt.logFunc(tt.message, tt.args...)

			w.Close()
			os.Stderr = originalStderr

			// Read captured output
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := strings.TrimSpace(buf.String())

			if !strings.HasPrefix(output, tt.prefix) {
				t.Errorf("Log output %q should start with %q", output, tt.prefix)
			}

			expectedMessage := fmt.Sprintf(tt.message, tt.args...)
			if !strings.Contains(output, expectedMessage) {
				t.Errorf("Log output %q should contain %q", output, expectedMessage)
			}
		})
	}
}
