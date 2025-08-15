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
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig) error {
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
	runExec = func(cfg *config.DuckConf, target string, args []string, securityCfg *config.SecurityConfig) error {
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
