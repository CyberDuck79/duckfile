//nolint:errcheck
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

func writeConfig(t *testing.T, dir string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRootSelectsDefault ensures invoking the root command with no target
// selects and executes the default target.
func TestRootSelectsDefault(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default: build

targets:
  build:
    name: build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: file.tpl
`)
	// stub runExec
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		if target != "build" {
			t.Fatalf("expected build got %s", target)
		}
		called = true
		return nil
	}
	defer func() { runExec = orig }()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatalf("runExec not called")
	}
}

// TestRootUnknownTarget confirms an unknown CLI target results in an error path.
func TestRootUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default: build

targets:
  build:
    name: build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: file.tpl
`)
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	rootCmd.SetArgs([]string{"unknown"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error")
	}
}

// TestRootVersionFlag validates that the --version flag short-circuits normal
// execution and prints the version string.
func TestVersionSubcommand(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default:
    name: build
    binary: echo
    fileFlag: -f
    template:
        repo: local
        path: file.tpl
`)
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rootCmd.SetArgs([]string{"version"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	err := rootCmd.Execute()
	w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "duck version") {
		t.Fatalf("missing version output: %q", buf.String())
	}
}

// TestRootPassThroughArgs checks that arguments after "--" are forwarded as
// passthrough args to the underlying execution call.
func TestRootPassThroughArgs(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default: build

targets:
  build:
    name: build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: file.tpl
`)
	captured := []string{}
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		captured = append(captured, args...)
		return nil
	}
	defer func() { runExec = orig }()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	rootCmd.SetArgs([]string{"--", "one", "two"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(captured) != 2 || captured[0] != "one" {
		t.Fatalf("passthrough mismatch: %v", captured)
	}
}

// TestConfigFlag validates the --config flag functionality
func TestConfigFlag(t *testing.T) {
	dir := t.TempDir()

	// Create multiple config files for testing
	customConfig := `version: 1
default: custom-build
targets:
  custom-build:
    name: custom-build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: custom.tpl
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(customConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	envConfig := `version: 1
default: env-build
targets:
  env-build:
    name: env-build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: env.tpl
`
	if err := os.WriteFile(filepath.Join(dir, "env.yaml"), []byte(envConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create standard duck.yaml for auto-discovery
	writeConfig(t, dir, `version: 1
default: auto-build
targets:
  auto-build:
    name: auto-build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: auto.tpl
`)

	// Test cases for config precedence
	tests := []struct {
		name           string
		configFlag     string
		envVar         string
		expectedTarget string
		expectError    bool
	}{
		{
			name:           "CLI flag takes precedence over env var and auto-discovery",
			configFlag:     "custom.yaml",
			envVar:         "env.yaml",
			expectedTarget: "custom-build",
		},
		{
			name:           "Environment variable when no CLI flag",
			envVar:         "env.yaml",
			expectedTarget: "env-build",
		},
		{
			name:           "Auto-discovery when no explicit config",
			expectedTarget: "auto-build",
		},
		{
			name:        "Error when CLI flag points to missing file",
			configFlag:  "missing.yaml",
			expectError: true,
		},
		{
			name:        "Error when env var points to missing file",
			envVar:      "missing.yaml",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global state
			configPath = ""

			// Set environment variable if specified
			if tt.envVar != "" {
				os.Setenv("DUCK_CONFIG", tt.envVar)
			} else {
				os.Unsetenv("DUCK_CONFIG")
			}
			defer os.Unsetenv("DUCK_CONFIG")

			// Capture the executed target
			var executedTarget string
			orig := runExec
			runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
				executedTarget = target
				return nil
			}
			defer func() { runExec = orig }()

			// Change to test directory
			oldWd, _ := os.Getwd()
			os.Chdir(dir)
			defer os.Chdir(oldWd)

			// Prepare command args
			args := []string{}
			if tt.configFlag != "" {
				args = append(args, "--config", tt.configFlag)
			}

			// Execute command
			rootCmd.SetArgs(args)
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetErr(&bytes.Buffer{})

			err := rootCmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if executedTarget != tt.expectedTarget {
				t.Fatalf("expected target %q but got %q", tt.expectedTarget, executedTarget)
			}
		})
	}
}

// TestConfigFlagShortForm validates the -c short form of the config flag
func TestConfigFlagShortForm(t *testing.T) {
	dir := t.TempDir()

	customConfig := `version: 1
default: short-build
targets:
  short-build:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: short.tpl
`
	if err := os.WriteFile(filepath.Join(dir, "short.yaml"), []byte(customConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reset global state
	configPath = ""

	var executedTarget string
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		executedTarget = target
		return nil
	}
	defer func() { runExec = orig }()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	rootCmd.SetArgs([]string{"-c", "short.yaml"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executedTarget != "short-build" {
		t.Fatalf("expected target 'short-build' but got %q", executedTarget)
	}
}

// TestConfigFlagWithTarget validates --config flag works with explicit targets
func TestConfigFlagWithTarget(t *testing.T) {
	dir := t.TempDir()

	customConfig := `version: 1
default: build
targets:
  build:
    name: build
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: build.tpl
  test:
    name: test
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: test.tpl
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(customConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reset global state
	configPath = ""

	var executedTarget string
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		executedTarget = target
		return nil
	}
	defer func() { runExec = orig }()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	rootCmd.SetArgs([]string{"--config", "custom.yaml", "test"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executedTarget != "test" {
		t.Fatalf("expected target 'test' but got %q", executedTarget)
	}
}
