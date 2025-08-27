//nolint:errcheck
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMainFunction tests the main function using subprocess pattern
// This test is skipped by default and requires explicit opt-in via environment variable
func TestMainFunction(t *testing.T) {
	if os.Getenv("TEST_MAIN_FUNCTION") != "1" {
		t.Skip("Set TEST_MAIN_FUNCTION=1 to run main() function tests")
	}

	// Build the binary for testing
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "duck-test")

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "duck")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, output)
	}

	tests := []struct {
		name           string
		args           []string
		setupConfig    string
		expectedExit   int
		expectedStdout string
		expectedStderr string
	}{
		{
			name:           "version_command_success",
			args:           []string{"version"},
			expectedExit:   0,
			expectedStdout: "duck version",
		},
		{
			name:           "help_command_success",
			args:           []string{"--help"},
			expectedExit:   0,
			expectedStdout: "Duck fetches, renders, and executes remote Git templates",
		},
		{
			name:           "unknown_command_error",
			args:           []string{"nonexistent-command"},
			expectedExit:   1,
			expectedStderr: "error:",
		},
		{
			name:           "missing_config_error",
			args:           []string{"list"},
			expectedExit:   1,
			expectedStderr: "no config file found",
		},
		{
			name: "list_command_with_config",
			args: []string{"list"},
			setupConfig: `version: 1
default: build
targets:
  build:
    description: Test build target
    binary: make
    fileFlag: -f
    template:
      repo: test-repo
      path: test.tpl
`,
			expectedExit:   0,
			expectedStdout: "TARGET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := t.TempDir()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(testDir)

			// Setup config file if needed
			if tt.setupConfig != "" {
				if err := os.WriteFile("duck.yaml", []byte(tt.setupConfig), 0o644); err != nil {
					t.Fatalf("Failed to create config: %v", err)
				}
			}

			// Execute the binary
			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Dir = testDir
			output, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("Failed to run command: %v", err)
				}
			}

			if exitCode != tt.expectedExit {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}

			outputStr := string(output)

			// Check expected stdout content
			if tt.expectedStdout != "" && !strings.Contains(outputStr, tt.expectedStdout) {
				t.Errorf("expected stdout to contain %q, got: %s", tt.expectedStdout, outputStr)
			}

			// Check expected stderr content
			if tt.expectedStderr != "" && !strings.Contains(outputStr, tt.expectedStderr) {
				t.Errorf("expected stderr to contain %q, got: %s", tt.expectedStderr, outputStr)
			}
		})
	}
}

// TestMainFunctionErrorExit tests that the main function exits with error code 1 on failure
func TestMainFunctionErrorExit(t *testing.T) {
	if os.Getenv("TEST_MAIN_FUNCTION") != "1" {
		t.Skip("Set TEST_MAIN_FUNCTION=1 to run main() function tests")
	}

	// Build the binary for testing
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "duck-test")

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "duck")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, output)
	}

	// Create test directory
	testDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(testDir)

	// Create invalid config that will cause an error
	invalidConfig := `invalid yaml content [[[`
	if err := os.WriteFile("duck.yaml", []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("Failed to create invalid config: %v", err)
	}

	// Execute the binary with a command that requires config
	cmd := exec.Command(binaryPath, "list")
	cmd.Dir = testDir
	output, err := cmd.CombinedOutput()

	// Should exit with error code 1
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run command: %v", err)
		}
	}

	if exitCode != 1 {
		t.Errorf("expected exit code 1 for invalid config, got %d", exitCode)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "error:") {
		t.Errorf("expected error message in output, got: %s", outputStr)
	}
}

// TestMainFunctionSuccessExit tests that the main function exits with code 0 on success
func TestMainFunctionSuccessExit(t *testing.T) {
	if os.Getenv("TEST_MAIN_FUNCTION") != "1" {
		t.Skip("Set TEST_MAIN_FUNCTION=1 to run main() function tests")
	}

	// Build the binary for testing
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "duck-test")

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "duck")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, output)
	}

	// Test that help command exits with 0
	cmd := exec.Command(binaryPath, "--help")
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run command: %v", err)
		}
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help command, got %d", exitCode)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Duck fetches, renders, and executes remote Git templates") {
		t.Errorf("expected help text in output, got: %s", outputStr)
	}
}

// TestMainFunctionSignalHandling tests that the main function handles signals properly
func TestMainFunctionSignalHandling(t *testing.T) {
	if os.Getenv("TEST_MAIN_FUNCTION") != "1" {
		t.Skip("Set TEST_MAIN_FUNCTION=1 to run main() function tests")
	}

	// This test is more complex and would require setting up a long-running command
	// and sending signals to it. For now, we'll skip this as it's more of an integration test
	t.Skip("Signal handling test requires more complex setup")
}
