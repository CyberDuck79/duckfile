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

func listDuckContent(t *testing.T, dir string) []string {
	// list .duck content
	files, err := os.ReadDir(filepath.Join(dir, ".duck"))
	if err != nil {
		t.Fatalf("failed to read .duck dir: %v", err)
	}
	var names []string
	for _, f := range files {
		if f.IsDir() {
			names = append(names, f.Name())
		}
	}
	return names
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
		t.Fatalf("objects dir missing precondition: %v", listDuckContent(t, dir))
	}
	rootCmd.SetArgs([]string{"clean"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("clean all: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".duck", "objects")); err == nil {
		t.Fatalf("objects dir still exists: %v", listDuckContent(t, dir))
	}
	if _, err := os.Stat(filepath.Join(dir, ".duck", "build")); err == nil {
		t.Fatalf("build dir still exists: %v", listDuckContent(t, dir))
	}
	if _, err := os.Stat(filepath.Join(dir, ".duck", "t1")); err == nil {
		t.Fatalf("t1 dir still exists: %v", listDuckContent(t, dir))
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
	if _, err := os.Stat(filepath.Join(dir, ".duck", "objects")); err != nil {
		t.Fatalf("objects dir missing precondition: %v", listDuckContent(t, dir))
	}
	// capture other target symlink path before clean
	otherLink := filepath.Join(dir, ".duck", "t1", "b")
	if _, err := os.Lstat(otherLink); err != nil {
		t.Fatalf("expected t1 symlink before clean")
	}
	// clean default target only
	rootCmd.SetArgs([]string{"clean", "build"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("clean default: %v", err)
	}
	// check build dir
	if _, err := os.Lstat(filepath.Join(dir, ".duck", "build")); err == nil {
		t.Fatalf("build dir still exists: %v", listDuckContent(t, dir))
	}
	// default symlink removed
	if _, err := os.Lstat(filepath.Join(dir, ".duck", "build", "a")); err == nil {
		t.Fatalf("default symlink still present: %v", listDuckContent(t, dir))
	}
	// other target still present
	if _, err := os.Lstat(otherLink); err != nil {
		t.Fatalf("t1 symlink removed unexpectedly: %v", listDuckContent(t, dir))
	}
}
