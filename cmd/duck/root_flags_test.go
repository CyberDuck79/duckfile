//nolint:errcheck
package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// Helper to run root command with commit hash flags
func runRootCLIWithFlags(t *testing.T, dir string, args ...string) error {
	t.Helper()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	rootCmd.SetArgs(args)
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	return rootCmd.Execute()
}

// TestRootCommitHashTrackingFlags tests the --track-commit-hash and --no-track-commit-hash flags in root command
func TestRootCommitHashTrackingFlags(t *testing.T) {
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

	// Test stub to capture flags
	var capturedTrackFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedTrackFlag = trackCommitHashFlag
		// autoUpdateOnChangeFlag is not captured in this test
		return nil
	}
	defer func() { runExec = orig }()

	// Test --track-commit-hash flag
	capturedTrackFlag = nil
	err := runRootCLIWithFlags(t, dir, "--track-commit-hash")
	if err != nil {
		t.Fatalf("root with --track-commit-hash failed: %v", err)
	}
	if capturedTrackFlag == nil || !*capturedTrackFlag {
		t.Fatalf("expected trackCommitHash=true, got: %v", capturedTrackFlag)
	}

	// Test --no-track-commit-hash flag
	capturedTrackFlag = nil
	err = runRootCLIWithFlags(t, dir, "--no-track-commit-hash")
	if err != nil {
		t.Fatalf("root with --no-track-commit-hash failed: %v", err)
	}
	if capturedTrackFlag == nil || *capturedTrackFlag {
		t.Fatalf("expected trackCommitHash=false, got: %v", capturedTrackFlag)
	}
}

// TestRootAutoUpdateFlags tests the --auto-update-on-change and --no-auto-update-on-change flags
func TestRootAutoUpdateFlags(t *testing.T) {
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

	// Test stub to capture flags
	var capturedAutoFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		// trackCommitHashFlag is not captured in this test
		capturedAutoFlag = autoUpdateOnChangeFlag
		return nil
	}
	defer func() { runExec = orig }()

	// Test --auto-update-on-change flag
	capturedAutoFlag = nil
	err := runRootCLIWithFlags(t, dir, "--auto-update-on-change")
	if err != nil {
		t.Fatalf("root with --auto-update-on-change failed: %v", err)
	}
	if capturedAutoFlag == nil || !*capturedAutoFlag {
		t.Fatalf("expected autoUpdateOnChange=true, got: %v", capturedAutoFlag)
	}

	// Test --no-auto-update-on-change flag
	capturedAutoFlag = nil
	err = runRootCLIWithFlags(t, dir, "--no-auto-update-on-change")
	if err != nil {
		t.Fatalf("root with --no-auto-update-on-change failed: %v", err)
	}
	if capturedAutoFlag == nil || *capturedAutoFlag {
		t.Fatalf("expected autoUpdateOnChange=false, got: %v", capturedAutoFlag)
	}
}

// TestRootCommitHashFlagCombination tests combining both commit hash flags
func TestRootCommitHashFlagCombination(t *testing.T) {
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

	// Test stub to capture flags
	var capturedTrackFlag *bool
	var capturedAutoFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedTrackFlag = trackCommitHashFlag
		capturedAutoFlag = autoUpdateOnChangeFlag
		return nil
	}
	defer func() { runExec = orig }()

	// Test combining both flags
	capturedTrackFlag = nil
	capturedAutoFlag = nil
	err := runRootCLIWithFlags(t, dir, "--track-commit-hash", "--auto-update-on-change")
	if err != nil {
		t.Fatalf("root with both flags failed: %v", err)
	}
	if capturedTrackFlag == nil || !*capturedTrackFlag {
		t.Fatalf("expected trackCommitHash=true, got: %v", capturedTrackFlag)
	}
	if capturedAutoFlag == nil || !*capturedAutoFlag {
		t.Fatalf("expected autoUpdateOnChange=true, got: %v", capturedAutoFlag)
	}
}

// TestRootFlagsWithTarget tests flags work with specific targets
func TestRootFlagsWithTarget(t *testing.T) {
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
  test:
    name: test
    binary: echo
    fileFlag: -f
    template:
      repo: local
    path: test.tpl
`)

	// Test stub to capture target and flags
	var capturedTarget string
	var capturedTrackFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedTarget = target
		capturedTrackFlag = trackCommitHashFlag
		return nil
	}
	defer func() { runExec = orig }()

	// Test flags with specific target
	capturedTarget = ""
	capturedTrackFlag = nil
	err := runRootCLIWithFlags(t, dir, "--track-commit-hash", "test")
	if err != nil {
		t.Fatalf("root with flags and target failed: %v", err)
	}
	if capturedTarget != "test" {
		t.Fatalf("expected target=test, got: %s", capturedTarget)
	}
	if capturedTrackFlag == nil || !*capturedTrackFlag {
		t.Fatalf("expected trackCommitHash=true, got: %v", capturedTrackFlag)
	}
}

// TestRootFlagsWithPassthrough tests flags work with passthrough args
func TestRootFlagsWithPassthrough(t *testing.T) {
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

	// Test stub to capture passthrough args and flags
	var capturedArgs []string
	var capturedTrackFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedArgs = args
		capturedTrackFlag = trackCommitHashFlag
		return nil
	}
	defer func() { runExec = orig }()

	// Test flags with passthrough args
	capturedArgs = nil
	capturedTrackFlag = nil
	err := runRootCLIWithFlags(t, dir, "--track-commit-hash", "--", "arg1", "arg2")
	if err != nil {
		t.Fatalf("root with flags and passthrough failed: %v", err)
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "arg1" || capturedArgs[1] != "arg2" {
		t.Fatalf("expected args=[arg1, arg2], got: %v", capturedArgs)
	}
	if capturedTrackFlag == nil || !*capturedTrackFlag {
		t.Fatalf("expected trackCommitHash=true, got: %v", capturedTrackFlag)
	}
}

// TestRootFlagsWithLogLevel tests flags work with --log-level
func TestRootFlagsWithLogLevel(t *testing.T) {
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

	// Test stub to capture flags
	var capturedTrackFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedTrackFlag = trackCommitHashFlag
		return nil
	}
	defer func() { runExec = orig }()

	// Test flags with --log-level
	capturedTrackFlag = nil
	err := runRootCLIWithFlags(t, dir, "--log-level", "debug", "--track-commit-hash")
	if err != nil {
		t.Fatalf("root with log-level and commit hash flags failed: %v", err)
	}
	if capturedTrackFlag == nil || !*capturedTrackFlag {
		t.Fatalf("expected trackCommitHash=true, got: %v", capturedTrackFlag)
	}
}

// TestRootWithoutCommitHashFlags tests root works without any commit hash flags
func TestRootWithoutCommitHashFlags(t *testing.T) {
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

	// Test stub to capture flags
	var capturedTrackFlag *bool
	var capturedAutoFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedTrackFlag = trackCommitHashFlag
		capturedAutoFlag = autoUpdateOnChangeFlag
		return nil
	}
	defer func() { runExec = orig }()

	// Test root without any commit hash flags (should pass nil values)
	capturedTrackFlag = nil
	capturedAutoFlag = nil
	err := runRootCLIWithFlags(t, dir)
	if err != nil {
		t.Fatalf("root without commit hash flags failed: %v", err)
	}
	if capturedTrackFlag != nil {
		t.Fatalf("expected trackCommitHash=nil, got: %v", capturedTrackFlag)
	}
	if capturedAutoFlag != nil {
		t.Fatalf("expected autoUpdateOnChange=nil, got: %v", capturedAutoFlag)
	}
}

// TestRootFlagParsing tests various flag parsing edge cases
func TestRootFlagParsing(t *testing.T) {
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

	// Test stub to capture flags
	var capturedTrackFlag *bool
	var capturedAutoFlag *bool
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
		capturedTrackFlag = trackCommitHashFlag
		capturedAutoFlag = autoUpdateOnChangeFlag
		return nil
	}
	defer func() { runExec = orig }()

	testCases := []struct {
		name        string
		args        []string
		expectTrack *bool
		expectAuto  *bool
	}{
		{
			name:        "both positive flags",
			args:        []string{"--track-commit-hash", "--auto-update-on-change"},
			expectTrack: func() *bool { b := true; return &b }(),
			expectAuto:  func() *bool { b := true; return &b }(),
		},
		{
			name:        "both negative flags",
			args:        []string{"--no-track-commit-hash", "--no-auto-update-on-change"},
			expectTrack: func() *bool { b := false; return &b }(),
			expectAuto:  func() *bool { b := false; return &b }(),
		},
		{
			name:        "mixed flags",
			args:        []string{"--track-commit-hash", "--no-auto-update-on-change"},
			expectTrack: func() *bool { b := true; return &b }(),
			expectAuto:  func() *bool { b := false; return &b }(),
		},
		{
			name:        "flags with target",
			args:        []string{"--track-commit-hash", "build"},
			expectTrack: func() *bool { b := true; return &b }(),
			expectAuto:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			capturedTrackFlag = nil
			capturedAutoFlag = nil
			err := runRootCLIWithFlags(t, dir, tc.args...)
			if err != nil {
				t.Fatalf("root command failed: %v", err)
			}

			// Check tracking flag
			if tc.expectTrack == nil {
				if capturedTrackFlag != nil {
					t.Fatalf("expected trackCommitHash=nil, got: %v", capturedTrackFlag)
				}
			} else {
				if capturedTrackFlag == nil {
					t.Fatalf("expected trackCommitHash=%v, got: nil", *tc.expectTrack)
				}
				if *capturedTrackFlag != *tc.expectTrack {
					t.Fatalf("expected trackCommitHash=%v, got: %v", *tc.expectTrack, *capturedTrackFlag)
				}
			}

			// Check auto-update flag
			if tc.expectAuto == nil {
				if capturedAutoFlag != nil {
					t.Fatalf("expected autoUpdateOnChange=nil, got: %v", capturedAutoFlag)
				}
			} else {
				if capturedAutoFlag == nil {
					t.Fatalf("expected autoUpdateOnChange=%v, got: nil", *tc.expectAuto)
				}
				if *capturedAutoFlag != *tc.expectAuto {
					t.Fatalf("expected autoUpdateOnChange=%v, got: %v", *tc.expectAuto, *capturedAutoFlag)
				}
			}
		})
	}
}
