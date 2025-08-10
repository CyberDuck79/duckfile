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

func writeConfig(t *testing.T, dir string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRootSelectsDefault(t *testing.T) {
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
	// stub runExec
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string) error {
		if target != "default" {
			t.Fatalf("expected default got %s", target)
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

func TestRootUnknownTarget(t *testing.T) {
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
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string) error {
		return fmt.Errorf("unknown target %s", target)
	}
	defer func() { runExec = orig }()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	rootCmd.SetArgs([]string{"doesnotexist"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRootVersionFlag(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 1

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
	rootCmd.SetArgs([]string{"--version"})
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
func TestRootTargetAliasToDefaultName(t *testing.T) {
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
	called := false
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string) error {
		if target != "default" {
			t.Fatalf("expected alias -> default, got %s", target)
		}
		called = true
		return nil
	}
	defer func() { runExec = orig }()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	rootCmd.SetArgs([]string{"build"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !called {
		t.Fatalf("runExec not called")
	}
}

func TestRootPassThroughArgs(t *testing.T) {
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
	captured := []string{}
	orig := runExec
	runExec = func(cfg *config.DuckConf, target string, args []string) error {
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
