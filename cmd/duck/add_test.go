//nolint:errcheck
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestAddCommand tests the add command through cobra
func TestAddCommand(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig string
		expectError bool
		errorMsg    string
		mockTarget  config.Target
		mockName    string
		mockError   error
	}{
		{
			name: "success_add_new_target",
			setupConfig: `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
`,
			expectError: false,
			mockTarget: config.Target{
				Binary:   "go",
				FileFlag: "-f",
				Template: config.Template{
					Repo: "new-repo",
					Path: "new.tpl",
				},
			},
			mockName: "test",
		},
		{
			name: "error_no_config_file",
			// No config file setup
			setupConfig: "",
			expectError: true,
			errorMsg:    "no config file found",
		},
		{
			name: "error_reserved_name_default",
			setupConfig: `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
`,
			expectError: true,
			errorMsg:    "cannot add target with reserved name 'default'",
			mockTarget:  config.Target{Binary: "echo"},
			mockName:    "default",
		},
		{
			name: "error_target_already_exists",
			setupConfig: `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
  test:
    binary: go
    fileFlag: -f
    template:
      repo: test-repo
      path: test.tpl
`,
			expectError: true,
			errorMsg:    "target test already exists",
			mockTarget:  config.Target{Binary: "echo"},
			mockName:    "test",
		},
		{
			name: "error_wizard_failure",
			setupConfig: `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
`,
			expectError: true,
			errorMsg:    "wizard failed",
			mockError:   fmt.Errorf("wizard failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			oldWd, _ := os.Getwd()
			os.Chdir(dir)
			defer os.Chdir(oldWd)

			// Setup config file if provided
			if tt.setupConfig != "" {
				if err := os.WriteFile("duck.yaml", []byte(tt.setupConfig), 0o644); err != nil {
					t.Fatalf("Failed to create config: %v", err)
				}
			}

			// Mock runTargetWizard
			originalRunTargetWizard := runTargetWizardFunc
			var wizardCalled bool
			runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
				wizardCalled = true
				if isDefault {
					t.Errorf("add command should call wizard with isDefault=false")
				}
				if tt.mockError != nil {
					return config.Target{}, "", tt.mockError
				}
				return tt.mockTarget, tt.mockName, nil
			}
			defer func() { runTargetWizardFunc = originalRunTargetWizard }()

			// Execute add command
			rootCmd.SetArgs([]string{"add"})
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetErr(&bytes.Buffer{})

			err := rootCmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error containing %q but got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !wizardCalled && tt.setupConfig != "" {
					t.Fatalf("expected runTargetWizard to be called")
				}

				// Verify target was added to config
				cfg, err := config.Load("duck.yaml")
				if err != nil {
					t.Fatalf("failed to load updated config: %v", err)
				}

				if target, exists := cfg.Targets[tt.mockName]; !exists {
					t.Errorf("expected target %q to be added", tt.mockName)
				} else {
					if target.Binary != tt.mockTarget.Binary {
						t.Errorf("expected binary %q, got %q", tt.mockTarget.Binary, target.Binary)
					}
				}
			}
		})
	}
}

// TestAddCommandConfigInitialization tests that add command properly initializes empty targets map
func TestAddCommandConfigInitialization(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create config with valid default target but no targets map initially
	configWithNilTargets := `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
`
	if err := os.WriteFile("duck.yaml", []byte(configWithNilTargets), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Mock runTargetWizard
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		return config.Target{
			Binary:   "echo",
			FileFlag: "-f",
			Template: config.Template{
				Repo: "test-repo",
				Path: "test.tpl",
			},
		}, "new-target", nil
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	// Execute add command
	rootCmd.SetArgs([]string{"add"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify config was updated properly
	cfg, err := config.Load("duck.yaml")
	if err != nil {
		t.Fatalf("failed to load updated config: %v", err)
	}

	if cfg.Targets == nil {
		t.Fatalf("targets map should be initialized")
	}

	if _, exists := cfg.Targets["new-target"]; !exists {
		t.Errorf("expected new-target to be added")
	}
}

// TestAddCommandSaveError tests error handling when config save fails
func TestAddCommandSaveError(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create valid config
	setupConfig := `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
`
	if err := os.WriteFile("duck.yaml", []byte(setupConfig), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Make duck.yaml read-only to cause save failure
	if err := os.Chmod("duck.yaml", 0o444); err != nil {
		t.Fatalf("Failed to make file read-only: %v", err)
	}

	// Mock runTargetWizard
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		return config.Target{
			Binary:   "echo",
			FileFlag: "-f",
			Template: config.Template{
				Repo: "test-repo",
				Path: "test.tpl",
			},
		}, "new-target", nil
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	// Execute add command
	rootCmd.SetArgs([]string{"add"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error from config save but got none")
	}

	// Restore permissions for cleanup
	os.Chmod("duck.yaml", 0o644)
}

// TestAddCommandInvalidConfig tests error handling with invalid existing config
func TestAddCommandInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create invalid YAML config
	invalidConfig := `version: 1
default: build
targets:
  build:
    binary: make
    template:
      repo: missing-path  # invalid: missing required path field
`
	if err := os.WriteFile("duck.yaml", []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Execute add command
	rootCmd.SetArgs([]string{"add"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error from invalid config but got none")
	}
}

// TestAddCommandOutput tests the output message when target is successfully added
func TestAddCommandOutput(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create valid config
	setupConfig := `version: 1
default: build
targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: existing-repo
      path: existing.tpl
`
	if err := os.WriteFile("duck.yaml", []byte(setupConfig), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Mock runTargetWizard
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		return config.Target{
			Binary:   "test-binary",
			FileFlag: "-f",
			Template: config.Template{
				Repo: "test-repo",
				Path: "test.tpl",
			},
		}, "my-target", nil
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	// Capture stdout
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute add command
	rootCmd.SetArgs([]string{"add"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()

	// Restore stdout
	w.Close()
	os.Stdout = originalStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read captured output
	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(r)
	output := outputBuf.String()

	if !strings.Contains(output, "Added target my-target") {
		t.Errorf("expected success message in output, got: %q", output)
	}
}
