//nolint:errcheck,unused
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// mockReader simulates user input for interactive tests
type mockReader struct {
	responses []string
	index     int
}

func newMockReader(responses ...string) *mockReader {
	return &mockReader{responses: responses, index: 0}
}

func (m *mockReader) ReadString(delim byte) (string, error) {
	if m.index >= len(m.responses) {
		return "", fmt.Errorf("no more mock responses available")
	}
	response := m.responses[m.index] + string(delim)
	m.index++
	return response, nil
}

// TestInitCommand tests the init command through cobra
func TestInitCommand(t *testing.T) {
	tests := []struct {
		name        string
		setupDir    func(string) // setup function for test directory
		expectError bool
		errorMsg    string
	}{
		{
			name: "success_new_directory",
			setupDir: func(dir string) {
				// Leave directory empty - no existing duck.yaml
			},
			expectError: false,
		},
		{
			name: "error_existing_duck_yaml",
			setupDir: func(dir string) {
				// Create existing duck.yaml
				existingConfig := `version: 1
default: existing
targets:
  existing:
    binary: echo
    template:
      repo: existing-repo
      path: test.tpl
`
				err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(existingConfig), 0o644)
				if err != nil {
					t.Fatalf("Failed to create existing duck.yaml: %v", err)
				}
			},
			expectError: true,
			errorMsg:    "duck.yaml already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setupDir(dir)

			oldWd, _ := os.Getwd()
			os.Chdir(dir)
			defer os.Chdir(oldWd)

			// Mock runInitWizard to avoid interactive input
			originalRunInitWizard := runInitWizardFunc
			var wizardCalled bool
			runInitWizardFunc = func() error {
				wizardCalled = true
				if tt.expectError && tt.errorMsg != "duck.yaml already exists" {
					return fmt.Errorf("wizard error")
				}
				// Create a minimal duck.yaml for successful test
				cfg := &config.DuckConf{
					Version: 1,
					Default: "test",
					Targets: map[string]config.Target{
						"test": {
							Binary: "echo",
							Template: config.Template{
								Repo: "test-repo",
								Path: "test.tpl",
							},
						},
					},
				}
				return cfg.Save("duck.yaml")
			}
			defer func() { runInitWizardFunc = originalRunInitWizard }()

			rootCmd.SetArgs([]string{"init"})
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
				if !wizardCalled {
					t.Fatalf("expected runInitWizard to be called")
				}
				// Verify duck.yaml was created
				if _, err := os.Stat("duck.yaml"); err != nil {
					t.Fatalf("duck.yaml was not created: %v", err)
				}
			}
		})
	}
}

// TestRunInitWizardSuccess tests the successful path of runInitWizard
func TestRunInitWizardSuccess(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Mock stdin with user responses
	originalStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Mock stdout to capture output
	originalStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	// Mock the runTargetWizard function to avoid nested complexity
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		if !isDefault {
			t.Errorf("runTargetWizard should be called with isDefault=true for first target")
		}
		return config.Target{
			Binary:   "make",
			FileFlag: "-f",
			Template: config.Template{
				Repo: "https://github.com/example/templates.git",
				Ref:  "main",
				Path: "Makefile.tpl",
			},
		}, "build", nil
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	// Provide input for additional target prompt (answer "no")
	go func() {
		defer w.Close()
		fmt.Fprint(w, "n\n")
	}()

	// Run the wizard
	err := runInitWizard()

	// Restore stdin/stdout
	outW.Close()
	os.Stdin = originalStdin
	os.Stdout = originalStdout

	// Read captured output
	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(outR)

	if err != nil {
		t.Fatalf("runInitWizard failed: %v", err)
	}

	// Verify duck.yaml was created
	if _, err := os.Stat("duck.yaml"); err != nil {
		t.Fatalf("duck.yaml was not created: %v", err)
	}

	// Load and verify the created config
	cfg, err := config.Load("duck.yaml")
	if err != nil {
		t.Fatalf("failed to load created config: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Default != "build" {
		t.Errorf("expected default 'build', got %q", cfg.Default)
	}
	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}
	if target, exists := cfg.Targets["build"]; !exists {
		t.Errorf("expected 'build' target to exist")
	} else {
		if target.Binary != "make" {
			t.Errorf("expected binary 'make', got %q", target.Binary)
		}
		if target.Template.Repo != "https://github.com/example/templates.git" {
			t.Errorf("expected specific repo, got %q", target.Template.Repo)
		}
	}

	// Verify output contains expected messages
	output := outputBuf.String()
	if !strings.Contains(output, "Created duck.yaml with default target 'build'") {
		t.Errorf("missing success message in output: %q", output)
	}
}

// TestRunInitWizardAddMultipleTargets tests adding multiple targets
func TestRunInitWizardAddMultipleTargets(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Mock stdin with user responses (yes to add another target, then no)
	originalStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Mock stdout to capture output
	originalStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	callCount := 0
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		callCount++
		switch callCount {
		case 1:
			if !isDefault {
				t.Errorf("first call should have isDefault=true")
			}
			return config.Target{
				Binary:   "make",
				FileFlag: "-f",
				Template: config.Template{
					Repo: "repo1",
					Path: "make.tpl",
				},
			}, "build", nil
		case 2:
			if isDefault {
				t.Errorf("second call should have isDefault=false")
			}
			return config.Target{
				Binary:   "go",
				FileFlag: "-f",
				Template: config.Template{
					Repo: "repo2",
					Path: "test.tpl",
				},
			}, "test", nil
		default:
			t.Errorf("unexpected call count: %d", callCount)
			return config.Target{}, "", fmt.Errorf("too many calls")
		}
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	// Provide input (yes for another target, then no)
	go func() {
		defer w.Close()
		fmt.Fprint(w, "y\n") // Add another target? yes
		fmt.Fprint(w, "n\n") // Add another target? no
	}()

	// Run the wizard
	err := runInitWizard()

	// Restore stdin/stdout
	outW.Close()
	os.Stdin = originalStdin
	os.Stdout = originalStdout

	// Read captured output
	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(outR)

	if err != nil {
		t.Fatalf("runInitWizard failed: %v", err)
	}

	// Verify duck.yaml was created with both targets
	cfg, err := config.Load("duck.yaml")
	if err != nil {
		t.Fatalf("failed to load created config: %v", err)
	}

	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(cfg.Targets))
	}

	if _, exists := cfg.Targets["build"]; !exists {
		t.Errorf("expected 'build' target to exist")
	}
	if _, exists := cfg.Targets["test"]; !exists {
		t.Errorf("expected 'test' target to exist")
	}

	// Verify output messages
	output := outputBuf.String()
	if !strings.Contains(output, "Created duck.yaml with default target 'build'") {
		t.Errorf("missing initial success message in output: %q", output)
	}
	if !strings.Contains(output, "Added target test") {
		t.Errorf("missing add target message in output: %q", output)
	}
}

// TestRunInitWizardTargetWizardError tests error handling when runTargetWizard fails
func TestRunInitWizardTargetWizardError(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Mock the runTargetWizard function to return an error
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		return config.Target{}, "", fmt.Errorf("wizard input error")
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	err := runInitWizard()

	if err == nil {
		t.Fatalf("expected error from runInitWizard but got none")
	}
	if !strings.Contains(err.Error(), "wizard input error") {
		t.Fatalf("expected error from wizard, got: %v", err)
	}

	// Verify duck.yaml was not created
	if _, err := os.Stat("duck.yaml"); err == nil {
		t.Fatalf("duck.yaml should not have been created after error")
	}
}

// TestRunInitWizardConfigSaveError tests error handling when config save fails
func TestRunInitWizardConfigSaveError(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create a directory named duck.yaml to cause save error
	if err := os.Mkdir("duck.yaml", 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Mock the runTargetWizard function
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		return config.Target{
			Binary: "echo",
			Template: config.Template{
				Repo: "test-repo",
				Path: "test.tpl",
			},
		}, "test", nil
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	err := runInitWizard()

	if err == nil {
		t.Fatalf("expected error from config save but got none")
	}
	if !strings.Contains(err.Error(), "duck.yaml") {
		t.Fatalf("expected error related to duck.yaml, got: %v", err)
	}
}

// TestRunInitWizardExistingTargetSkip tests that existing targets are skipped when adding additional targets
func TestRunInitWizardExistingTargetSkip(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Mock stdin with user responses
	originalStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Mock stdout to capture output
	originalStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	callCount := 0
	originalRunTargetWizard := runTargetWizardFunc
	runTargetWizardFunc = func(isDefault bool) (config.Target, string, error) {
		callCount++
		switch callCount {
		case 1:
			return config.Target{
				Binary:   "make",
				FileFlag: "-f",
				Template: config.Template{Repo: "repo1", Path: "make.tpl"},
			}, "build", nil
		case 2:
			// Return same target name to test skip behavior
			return config.Target{
				Binary:   "different",
				FileFlag: "-f",
				Template: config.Template{Repo: "repo2", Path: "test.tpl"},
			}, "build", nil // Same name as first target
		default:
			t.Errorf("unexpected call count: %d", callCount)
			return config.Target{}, "", fmt.Errorf("too many calls")
		}
	}
	defer func() { runTargetWizardFunc = originalRunTargetWizard }()

	// Provide input (yes for another target, then no)
	go func() {
		defer w.Close()
		fmt.Fprint(w, "y\n") // Add another target? yes
		fmt.Fprint(w, "n\n") // Add another target? no
	}()

	// Run the wizard
	err := runInitWizard()

	// Restore stdin/stdout
	outW.Close()
	os.Stdin = originalStdin
	os.Stdout = originalStdout

	// Read captured output
	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(outR)

	if err != nil {
		t.Fatalf("runInitWizard failed: %v", err)
	}

	// Verify only one target exists (the duplicate was skipped)
	cfg, err := config.Load("duck.yaml")
	if err != nil {
		t.Fatalf("failed to load created config: %v", err)
	}

	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target (duplicate skipped), got %d", len(cfg.Targets))
	}

	// Verify the original target wasn't modified
	if target, exists := cfg.Targets["build"]; !exists {
		t.Errorf("expected 'build' target to exist")
	} else if target.Binary != "make" {
		t.Errorf("target should not have been modified, got binary: %q", target.Binary)
	}

	// Verify skip message in output
	output := outputBuf.String()
	if !strings.Contains(output, "Target already exists; skipping") {
		t.Errorf("missing skip message in output: %q", output)
	}
}
