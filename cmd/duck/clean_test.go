package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/run"
)

// helper to stub clone for sync before clean tests
func stubCloneForClean(t *testing.T) {
	saved := run.TestGetCloneFunc()
	run.TestSetCloneFunc(func(repo, ref, intoDir string) (string, error) {
		repoDir := filepath.Join(intoDir, "repo")
		os.MkdirAll(repoDir, 0o755)
		// minimal template names
		names := []string{"a.tpl", "b.tpl"}
		for _, n := range names {
			os.WriteFile(filepath.Join(repoDir, n), []byte(n), 0o644)
		}
		return repoDir, nil
	})
	t.Cleanup(func() { run.TestSetCloneFunc(saved) })
}

// writeCleanConfig writes config with default + t1
func writeCleanConfig(t *testing.T, dir string) {
	body := `version: 1
default: build
  
targets:
  build:
    template:
      repo: r1
      path: a.tpl
    binary: echo
    fileFlag: -f
    variables: {}
    args: "--x"
  t1:
    template:
      repo: r2
      path: b.tpl
    binary: echo
    fileFlag: -f
    variables: {}
`
	if err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// syncAll renders both targets (used as precondition for clean tests)
func syncAll(t *testing.T) {
	rootCmd.SetArgs([]string{"sync"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("sync precondition: %v", err)
	}
}

// TestCLICleanSingleTarget ensures cleaning one target leaves the other intact (its symlink/object remain).
func TestCLICleanSingleTarget(t *testing.T) {
	dir := t.TempDir()
	stubCloneForClean(t)
	writeCleanConfig(t, dir)
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	syncAll(t)
	// capture other target symlink path before clean
	otherLink := filepath.Join(dir, ".duck", "t1", "b")
	if _, err := os.Lstat(otherLink); err != nil {
		t.Fatalf("expected t1 symlink before clean")
	}
	// clean default target only
	rootCmd.SetArgs([]string{"clean", "default"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("clean default: %v", err)
	}
	// default symlink removed
	if _, err := os.Lstat(filepath.Join(dir, ".duck", "build", "a")); err == nil {
		t.Fatalf("default symlink still present")
	}
	// other target still present
	if _, err := os.Lstat(otherLink); err != nil {
		t.Fatalf("t1 symlink removed unexpectedly")
	}
}

// TestCLICleanAll purges .duck/objects and per-target directories for all targets.
func TestCLICleanAll(t *testing.T) {
	dir := t.TempDir()
	stubCloneForClean(t)
	writeCleanConfig(t, dir)
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	syncAll(t)
	if _, err := os.Stat(filepath.Join(dir, ".duck", "objects")); err != nil {
		t.Fatalf("objects dir missing precondition")
	}
	rootCmd.SetArgs([]string{"clean"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("clean all: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".duck", "objects")); err == nil {
		t.Fatalf("objects dir still exists")
	}
	if _, err := os.Stat(filepath.Join(dir, ".duck", "default")); err == nil {
		t.Fatalf("default dir still exists")
	}
	if _, err := os.Stat(filepath.Join(dir, ".duck", "t1")); err == nil {
		t.Fatalf("t1 dir still exists")
	}
}
