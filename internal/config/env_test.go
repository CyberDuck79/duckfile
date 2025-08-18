package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindEnvFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	tests := []struct {
		name           string
		createFiles    []string
		expectedResult string
	}{
		{
			name:           "no env files",
			createFiles:    []string{},
			expectedResult: "",
		},
		{
			name:           "only .env",
			createFiles:    []string{".env"},
			expectedResult: ".env",
		},
		{
			name:           "only .duck/.env",
			createFiles:    []string{".duck/.env"},
			expectedResult: ".duck/.env",
		},
		{
			name:           "only .env.duck",
			createFiles:    []string{".env.duck"},
			expectedResult: ".env.duck",
		},
		{
			name:           "priority order: .env wins",
			createFiles:    []string{".env", ".duck/.env", ".env.duck"},
			expectedResult: ".env",
		},
		{
			name:           "priority order: .duck/.env wins over .env.duck",
			createFiles:    []string{".duck/.env", ".env.duck"},
			expectedResult: ".duck/.env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			os.RemoveAll(".env")
			os.RemoveAll(".duck")
			os.RemoveAll(".env.duck")

			// Create test files
			for _, file := range tt.createFiles {
				dir := filepath.Dir(file)
				if dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatalf("failed to create directory %s: %v", dir, err)
					}
				}
				if err := os.WriteFile(file, []byte("TEST=value"), 0644); err != nil {
					t.Fatalf("failed to create file %s: %v", file, err)
				}
			}

			result := findEnvFile()
			if result != tt.expectedResult {
				t.Errorf("findEnvFile() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedCount int
		expectedError bool
		expectedVars  map[string]string
		existingVars  map[string]string // Environment variables to set before loading
	}{
		{
			name:          "empty file",
			content:       "",
			expectedCount: 0,
			expectedError: false,
			expectedVars:  map[string]string{},
		},
		{
			name:          "empty path",
			content:       "",
			expectedCount: 0,
			expectedError: false,
			expectedVars:  map[string]string{},
		},
		{
			name: "basic key-value pairs",
			content: `KEY1=value1
KEY2=value2
KEY3=value3`,
			expectedCount: 3,
			expectedError: false,
			expectedVars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
		{
			name: "comments and empty lines",
			content: `# This is a comment
KEY1=value1

# Another comment
KEY2=value2

`,
			expectedCount: 2,
			expectedError: false,
			expectedVars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		},
		{
			name: "quoted values",
			content: `KEY1="quoted value"
KEY2='single quoted'
KEY3=unquoted`,
			expectedCount: 3,
			expectedError: false,
			expectedVars: map[string]string{
				"KEY1": "quoted value",
				"KEY2": "single quoted",
				"KEY3": "unquoted",
			},
		},
		{
			name: "values with spaces",
			content: `KEY1= value with spaces 
KEY2=  "  quoted with spaces  "  
KEY3=   'single quoted with spaces'   `,
			expectedCount: 3,
			expectedError: false,
			expectedVars: map[string]string{
				"KEY1": "value with spaces",
				"KEY2": "  quoted with spaces  ",
				"KEY3": "single quoted with spaces",
			},
		},
		{
			name: "empty values",
			content: `KEY1=
KEY2=""
KEY3=''`,
			expectedCount: 3,
			expectedError: false,
			expectedVars: map[string]string{
				"KEY1": "",
				"KEY2": "",
				"KEY3": "",
			},
		},
		{
			name: "values with equals signs",
			content: `URL=https://example.com:8080/path?param=value
EQUATION=1+1=2`,
			expectedCount: 2,
			expectedError: false,
			expectedVars: map[string]string{
				"URL":      "https://example.com:8080/path?param=value",
				"EQUATION": "1+1=2",
			},
		},
		{
			name: "existing environment variables take precedence",
			content: `KEY1=from_file
KEY2=from_file
NEW_KEY=from_file`,
			existingVars: map[string]string{
				"KEY1": "from_env",
			},
			expectedCount: 2, // Only KEY2 and NEW_KEY are set (KEY1 already exists)
			expectedError: false,
			expectedVars: map[string]string{
				"KEY1":    "from_env", // Original value preserved
				"KEY2":    "from_file",
				"NEW_KEY": "from_file",
			},
		},
		{
			name:          "invalid format - no equals",
			content:       "INVALID_LINE",
			expectedCount: 0,
			expectedError: true,
		},
		{
			name:          "invalid format - empty key",
			content:       "=value",
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up existing environment variables
			for key, value := range tt.existingVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			// Clean up any test variables that might be set
			for key := range tt.expectedVars {
				if _, exists := tt.existingVars[key]; !exists {
					os.Unsetenv(key)
					defer os.Unsetenv(key)
				}
			}

			var path string
			if tt.content != "" || tt.name != "empty path" {
				// Create temporary file
				tmpfile, err := os.CreateTemp("", "test.env")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpfile.Name())

				if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
					t.Fatal(err)
				}
				if err := tmpfile.Close(); err != nil {
					t.Fatal(err)
				}
				path = tmpfile.Name()
			}

			count, err := LoadEnvFile(path)

			if tt.expectedError {
				if err == nil {
					t.Errorf("LoadEnvFile() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("LoadEnvFile() unexpected error: %v", err)
				return
			}

			if count != tt.expectedCount {
				t.Errorf("LoadEnvFile() count = %d, want %d", count, tt.expectedCount)
			}

			// Check that expected variables are set correctly
			for key, expectedValue := range tt.expectedVars {
				actualValue := os.Getenv(key)
				if actualValue != expectedValue {
					t.Errorf("Expected %s=%q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestRemoveQuotes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"unquoted", "unquoted"},
		{`"double quoted"`, "double quoted"},
		{`'single quoted'`, "single quoted"},
		{`"partial quote`, `"partial quote`},
		{`partial quote"`, `partial quote"`},
		{`'partial quote`, `'partial quote`},
		{`partial quote'`, `partial quote'`},
		{`"mixed quotes'`, `"mixed quotes'`},
		{`'mixed quotes"`, `'mixed quotes"`},
		{`""`, ""},
		{`''`, ""},
		{`"a"`, "a"},
		{`'a'`, "a"},
		{`"multiple words"`, "multiple words"},
		{`'multiple words'`, "multiple words"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := removeQuotes(tt.input)
			if result != tt.expected {
				t.Errorf("removeQuotes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadEnvFileIfExists(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	t.Run("no env file", func(t *testing.T) {
		var logMessages []string
		logFunc := func(format string, args ...any) {
			logMessages = append(logMessages, fmt.Sprintf(format, args...))
		}

		err := LoadEnvFileIfExists(logFunc)
		if err != nil {
			t.Errorf("LoadEnvFileIfExists() unexpected error: %v", err)
		}

		if len(logMessages) != 0 {
			t.Errorf("Expected no log messages, got: %v", logMessages)
		}
	})

	t.Run("with env file", func(t *testing.T) {
		// Create .env file
		envContent := `TEST_KEY=test_value
ANOTHER_KEY=another_value`
		err := os.WriteFile(".env", []byte(envContent), 0644)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(".env")

		// Clean up environment
		os.Unsetenv("TEST_KEY")
		os.Unsetenv("ANOTHER_KEY")
		defer os.Unsetenv("TEST_KEY")
		defer os.Unsetenv("ANOTHER_KEY")

		var logMessages []string
		logFunc := func(format string, args ...any) {
			logMessages = append(logMessages, fmt.Sprintf(format, args...))
		}

		err = LoadEnvFileIfExists(logFunc)
		if err != nil {
			t.Errorf("LoadEnvFileIfExists() unexpected error: %v", err)
		}

		if len(logMessages) != 1 {
			t.Errorf("Expected 1 log message, got: %v", logMessages)
		} else if !strings.Contains(logMessages[0], "loaded 2 environment variables from .env") {
			t.Errorf("Unexpected log message: %s", logMessages[0])
		}

		// Verify variables were loaded
		if os.Getenv("TEST_KEY") != "test_value" {
			t.Errorf("Expected TEST_KEY=test_value, got %s", os.Getenv("TEST_KEY"))
		}
		if os.Getenv("ANOTHER_KEY") != "another_value" {
			t.Errorf("Expected ANOTHER_KEY=another_value, got %s", os.Getenv("ANOTHER_KEY"))
		}
	})

	t.Run("no logging function", func(t *testing.T) {
		// Create .env file
		envContent := `SILENT_KEY=silent_value`
		err := os.WriteFile(".env", []byte(envContent), 0644)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(".env")

		// Clean up environment
		os.Unsetenv("SILENT_KEY")
		defer os.Unsetenv("SILENT_KEY")

		err = LoadEnvFileIfExists(nil)
		if err != nil {
			t.Errorf("LoadEnvFileIfExists() unexpected error: %v", err)
		}

		// Verify variable was loaded
		if os.Getenv("SILENT_KEY") != "silent_value" {
			t.Errorf("Expected SILENT_KEY=silent_value, got %s", os.Getenv("SILENT_KEY"))
		}
	})
}

func TestLoadEnvFileIfExistsSilent(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	// Create .env file
	envContent := `SILENT_TEST_KEY=silent_test_value`
	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(".env")

	// Clean up environment
	os.Unsetenv("SILENT_TEST_KEY")
	defer os.Unsetenv("SILENT_TEST_KEY")

	err = LoadEnvFileIfExistsSilent()
	if err != nil {
		t.Errorf("LoadEnvFileIfExistsSilent() unexpected error: %v", err)
	}

	// Verify variable was loaded
	if os.Getenv("SILENT_TEST_KEY") != "silent_test_value" {
		t.Errorf("Expected SILENT_TEST_KEY=silent_test_value, got %s", os.Getenv("SILENT_TEST_KEY"))
	}
}
