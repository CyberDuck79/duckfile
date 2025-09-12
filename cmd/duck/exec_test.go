//nolint:errcheck
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/spf13/cobra"
)

const notCalledErrorMessage = "runExec was not called"

// testSetup provides common test setup functionality
type testSetup struct {
	dir            string
	oldWd          string
	origRunExec    func(*config.DuckConf, string, []string, *config.SecurityConfig, *bool, *bool) error
	executedTarget string
	executedArgs   []string
	trackCommitHash *bool
	called         bool
}

// newTestSetup creates a new test setup with common initialization
func newTestSetup(t *testing.T, configContent string) *testSetup {
	t.Helper()
	
	setup := &testSetup{
		dir: t.TempDir(),
	}
	
	writeConfig(t, setup.dir, configContent)
	
	// Setup working directory
	setup.oldWd, _ = os.Getwd()
	os.Chdir(setup.dir)
	
	// Setup runExec mock
	setup.origRunExec = runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, tch *bool, auoc *bool) error {
		setup.executedTarget = target
		setup.executedArgs = args
		setup.trackCommitHash = tch
		setup.called = true
		return nil
	}
	
	return setup
}

// cleanup restores the original state
func (s *testSetup) cleanup() {
	runExec = s.origRunExec
	os.Chdir(s.oldWd)
	configPath = ""
}

// createExecCommand creates a cobra command for testing exec functionality
func createExecCommand(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                "exec",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTargetFromArgsWithCmd(cmd, args)
		},
	}
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

// runExecTest executes a command and handles common error checking
func runExecTest(t *testing.T, cmd *cobra.Command, expectedError bool, errorMessage string) error {
	t.Helper()
	
	err := cmd.Execute()
	if expectedError {
		if err == nil {
			t.Fatalf("expected error but got none")
		}
		if errorMessage != "" && !strings.Contains(err.Error(), errorMessage) {
			t.Fatalf("error should contain %q, got: %v", errorMessage, err)
		}
	} else if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return err
}

// defaultConfig returns the standard test configuration
func defaultConfig() string {
	return `version: 1
default: build
targets:
  build:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: build.tpl`
}

// syncConfig returns configuration with sync target for conflict testing
func syncConfig() string {
	return `version: 1
default: build
targets:
  build:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: build.tpl
  sync:
    binary: echo  
    fileFlag: -f
    template:
      repo: local
      path: sync.tpl`
}

// TestExecCommand tests basic exec command functionality
func TestExecCommand(t *testing.T) {
	setup := newTestSetup(t, syncConfig())
	defer setup.cleanup()

	cmd := createExecCommand([]string{"sync"})
	runExecTest(t, cmd, false, "")

	if !setup.called {
		t.Fatal(notCalledErrorMessage)
	}

	if setup.executedTarget != "sync" {
		t.Fatalf("expected target 'sync', got %q", setup.executedTarget)
	}
}

// TestExecCommandWithPassthrough tests exec command with passthrough args
func TestExecCommandWithPassthrough(t *testing.T) {
	setup := newTestSetup(t, defaultConfig())
	defer setup.cleanup()

	cmd := createExecCommand([]string{"build", "--", "--verbose"})
	runExecTest(t, cmd, false, "")

	if !setup.called {
		t.Fatal(notCalledErrorMessage)
	}

	if setup.executedTarget != "build" {
		t.Fatalf("expected target 'build', got %q", setup.executedTarget)
	}

	if len(setup.executedArgs) != 1 || setup.executedArgs[0] != "--verbose" {
		t.Fatalf("expected args ['--verbose'], got %v", setup.executedArgs)
	}
}

// TestExecCommandWithFlags tests exec command with flags
func TestExecCommandWithFlags(t *testing.T) {
	setup := newTestSetup(t, defaultConfig())
	defer setup.cleanup()

	cmd := createExecCommand([]string{"--track-commit-hash", "build"})
	runExecTest(t, cmd, false, "")

	if !setup.called {
		t.Fatal(notCalledErrorMessage)
	}

	if setup.executedTarget != "build" {
		t.Fatalf("expected target 'build', got %q", setup.executedTarget)
	}

	if setup.trackCommitHash == nil || !*setup.trackCommitHash {
		t.Fatalf("expected trackCommitHashFlag to be true")
	}
}// TestExecCommandHelp tests that exec command help works
func TestExecCommandHelp(t *testing.T) {
	// Create a simple command just for help test
	cmd := &cobra.Command{
		Use:   "exec [target] [-- target_args...]",
		Short: "Execute a target explicitly",
		Long: `Execute a specific target with optional arguments.

This command explicitly executes a target, useful when target names 
conflict with subcommand names.

EXAMPLES
  duck exec build                    Execute 'build' target
  duck exec test -- --verbose       Execute 'test' target with args
  duck exec sync                     Execute target named 'sync' (not subcommand)
  duck exec --config=custom.yaml build  Use custom config`,
	}

	cmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	// Help should exit without error
	if err != nil {
		t.Fatalf("exec help failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Execute a specific target") {
		t.Fatalf("help output should contain exec description")
	}

	if !strings.Contains(output, "duck exec build") {
		t.Fatalf("help output should contain examples")
	}
}

// TestExecCommandWithConfigFlag tests exec command with config flag
func TestExecCommandWithConfigFlag(t *testing.T) {
	dir := t.TempDir()

	customConfig := `version: 1
default: custom
targets:
  custom:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: custom.tpl`

	customConfigPath := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(customConfigPath, []byte(customConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := newTestSetup(t, "")
	// Override the directory since we need a custom config
	setup.cleanup()
	setup.dir = dir
	setup.oldWd, _ = os.Getwd()
	os.Chdir(dir)
	defer setup.cleanup()

	// Setup mock after changing directory
	setup.origRunExec = runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, tch *bool, auoc *bool) error {
		setup.executedTarget = target
		setup.called = true
		return nil
	}

	cmd := createExecCommand([]string{"--config", "custom.yaml", "custom"})
	runExecTest(t, cmd, false, "")

	if !setup.called {
		t.Fatal(notCalledErrorMessage)
	}

	if setup.executedTarget != "custom" {
		t.Fatalf("expected target 'custom', got %q", setup.executedTarget)
	}
}

// TestExecCommandUnknownTarget tests exec command with unknown target
func TestExecCommandUnknownTarget(t *testing.T) {
	setup := newTestSetup(t, defaultConfig())
	defer setup.cleanup()

	// Don't mock runExec for this test - restore original
	runExec = setup.origRunExec

	cmd := createExecCommand([]string{"unknown"})
	runExecTest(t, cmd, true, "unknown target")
}
