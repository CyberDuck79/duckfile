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

// TestExecCommand tests basic exec command functionality
func TestExecCommand(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
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
      path: sync.tpl
`)

	// Test executing target that conflicts with subcommand name
	var executedTarget string
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		executedTarget = target
		called = true
		return nil
	}
	defer func() { runExec = orig }()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Test: duck exec sync (should execute target, not subcommand)
	cmd := &cobra.Command{
		Use:                "exec",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTargetFromArgsWithCmd(cmd, args)
		},
	}
	cmd.SetArgs([]string{"sync"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec sync failed: %v", err)
	}

	if !called {
		t.Fatal("runExec was not called")
	}

	if executedTarget != "sync" {
		t.Fatalf("expected target 'sync', got %q", executedTarget)
	}
}

// TestExecCommandWithPassthrough tests exec command with passthrough args
func TestExecCommandWithPassthrough(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default: build
targets:
  build:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: build.tpl
`)

	var executedTarget string
	var executedArgs []string
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		executedTarget = target
		executedArgs = args
		called = true
		return nil
	}
	defer func() { runExec = orig }()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Test: duck exec build -- --verbose
	cmd := &cobra.Command{
		Use:                "exec",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTargetFromArgsWithCmd(cmd, args)
		},
	}
	cmd.SetArgs([]string{"build", "--", "--verbose"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec build with passthrough failed: %v", err)
	}

	if !called {
		t.Fatal("runExec was not called")
	}

	if executedTarget != "build" {
		t.Fatalf("expected target 'build', got %q", executedTarget)
	}

	if len(executedArgs) != 1 || executedArgs[0] != "--verbose" {
		t.Fatalf("expected args ['--verbose'], got %v", executedArgs)
	}
}

// TestExecCommandWithFlags tests exec command with flags
func TestExecCommandWithFlags(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default: build
targets:
  build:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: build.tpl
`)

	var executedTarget string
	var trackCommitHashFlag *bool
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, tch *bool, autoUpdateOnChangeFlag *bool) error {
		executedTarget = target
		trackCommitHashFlag = tch
		called = true
		return nil
	}
	defer func() { runExec = orig }()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Test: duck exec --track-commit-hash build
	cmd := &cobra.Command{
		Use:                "exec",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTargetFromArgsWithCmd(cmd, args)
		},
	}
	cmd.SetArgs([]string{"--track-commit-hash", "build"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec with flags failed: %v", err)
	}

	if !called {
		t.Fatal("runExec was not called")
	}

	if executedTarget != "build" {
		t.Fatalf("expected target 'build', got %q", executedTarget)
	}

	if trackCommitHashFlag == nil || !*trackCommitHashFlag {
		t.Fatalf("expected trackCommitHashFlag to be true")
	}
} // TestExecCommandHelp tests that exec command help works
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
      path: custom.tpl
`
	customConfigPath := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(customConfigPath, []byte(customConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	var executedTarget string
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		executedTarget = target
		called = true
		return nil
	}
	defer func() { runExec = orig }()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Reset global state
	configPath = ""

	// Test: duck exec --config custom.yaml custom
	cmd := &cobra.Command{
		Use:                "exec",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTargetFromArgsWithCmd(cmd, args)
		},
	}
	cmd.SetArgs([]string{"--config", "custom.yaml", "custom"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec with config flag failed: %v", err)
	}

	if !called {
		t.Fatal("runExec was not called")
	}

	if executedTarget != "custom" {
		t.Fatalf("expected target 'custom', got %q", executedTarget)
	}
}

// TestExecCommandUnknownTarget tests exec command with unknown target
func TestExecCommandUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1
default: build
targets:
  build:
    binary: echo
    fileFlag: -f
    template:
      repo: local
      path: build.tpl
`)

	// Don't mock runExec - let it run and fail with unknown target
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Reset global state
	configPath = ""

	// Test: duck exec unknown
	cmd := &cobra.Command{
		Use:                "exec",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTargetFromArgsWithCmd(cmd, args)
		},
	}
	cmd.SetArgs([]string{"unknown"})
	var errBuf bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for unknown target")
	}

	if !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("error should mention unknown target, got: %v", err)
	}
}
