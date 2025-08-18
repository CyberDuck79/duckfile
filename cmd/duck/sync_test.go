//nolint:errcheck
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CyberDuck79/duckfile/internal/run"
)

// runSyncCLI executes the `duck sync` command with provided arguments inside dir.
func runSyncCLI(t *testing.T, dir string, args ...string) {
	t.Helper()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	rootCmd.SetArgs(append([]string{"sync"}, args...))
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("sync command failed: %v", err)
	}
}

// writeSyncConfig writes a config with: default, t1 (binary), nobin (no binary).
func writeSyncConfig(t *testing.T, dir string) {
	t.Helper()
	body := `version: 1
default: def

targets:
  def:
    description: default target
    binary: echo
    fileFlag: -f
    template:
      repo: repoDef
      ref: main
      path: default.tpl
    variables: {}
  t1:
    description: target one
    binary: echo
    fileFlag: -f
    template:
      repo: repoOne
      ref: main
      path: one.tpl
    variables: {}
  nobin:
    description: no binary target
    template:
      repo: repoNoBin
      ref: main
      path: nobin.tpl
    variables: {}
`
	if err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// stubClone installs a cloneFunc stub that creates a fake repo directory containing the
// requested template path and returns its path. Content is static (or passed string) so
// force re-render detection relies on file modtime changes, not content differences.
func stubClone(t *testing.T, content string) {
	saved := getCloneFunc()
	setCloneFunc(func(repo, ref, intoDir string) (string, error) {
		repoDir := filepath.Join(intoDir, "repo")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return "", err
		}
		names := []string{"default.tpl", "one.tpl", "nobin.tpl"}
		for _, n := range names {
			p := filepath.Join(repoDir, n)
			// Always overwrite to allow force re-render content changes; object file guarded by existing file unless force.
			os.WriteFile(p, fmt.Appendf(nil, "%s\n", content), 0o644)
		}
		return repoDir, nil
	})
	t.Cleanup(func() { setCloneFunc(saved) })
}

// Helper wrappers for cloneFunc seam (exposed via test_seams.go in run package)
func getCloneFunc() func(string, string, string) (string, error)  { return run.TestGetCloneFunc() }
func setCloneFunc(f func(string, string, string) (string, error)) { run.TestSetCloneFunc(f) }

// getSymlinkTarget resolves the absolute target file of the symlink at link.
func getSymlinkTarget(t *testing.T, link string) string {
	t.Helper()
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink %s: %v", link, err)
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(link), dest)
	}
	abs, _ := filepath.Abs(dest)
	return abs
}

// TestCLISyncForceRerender validates -f flag triggers re-render (modtime increase) while a plain
// subsequent sync without -f skips rendering (modtime unchanged).
func TestCLISyncForceRerender(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubClone(t, "INITIAL1")
	runSyncCLI(t, dir) // initial render
	link := filepath.Join(dir, ".duck", "def", "default")
	obj := getSymlinkTarget(t, link)
	before, _ := os.ReadFile(obj)
	// Plain sync (no -f) should NOT overwrite existing object; keep same content stub
	runSyncCLI(t, dir)
	mid, _ := os.ReadFile(obj)
	if string(mid) != string(before) {
		t.Fatalf("unexpected content change without -f: before=%q mid=%q", string(before), string(mid))
	}
	// Change stub content THEN force sync to induce update
	stubClone(t, "UPDATED")
	// Force sync should re-render (overwriting object file) even though key unchanged => content changes
	runSyncCLI(t, dir, "-f")
	after, _ := os.ReadFile(obj)
	if string(after) == string(before) {
		t.Fatalf("expected content change after force sync; still %q", string(after))
	}
}

// TestCLISyncAllCreatesSymlinks verifies that running `duck sync` with no target
// renders all targets (default + each named) and creates per-target symlinks.
func TestCLISyncAllCreatesSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	// Provide clone stub
	stubClone(t, "CONTENT")
	runSyncCLI(t, dir) // no args => all targets
	// Expect symlinks under .duck/<target>/<basename>
	for _, tgt := range []string{"def", "t1", "nobin"} {
		base := map[string]string{"def": "default", "t1": "one", "nobin": "nobin"}[tgt]
		link := filepath.Join(dir, ".duck", tgt, base)
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink for target %s at %s", tgt, link)
		}
	}
}

// TestCLISyncSingleTargetOnlyUpdatesRequested ensures syncing one target leaves others' object
// files untouched (modtime unchanged) while updating the requested target when forced.
func TestCLISyncSingleTargetOnlyUpdatesRequested(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubClone(t, "CONTENT")
	runSyncCLI(t, dir) // initial sync all
	// Capture object file modtimes via symlink resolution
	getObj := func(target, base string) string {
		link := filepath.Join(dir, ".duck", target, base)
		return getSymlinkTarget(t, link)
	}
	defObj := getObj("def", "default")
	t1Obj := getObj("t1", "one")
	defInfo1, _ := os.Stat(defObj)
	t1Info1, _ := os.Stat(t1Obj)
	time.Sleep(1100 * time.Millisecond) // ensure modtime tick
	// Force sync only t1
	runSyncCLI(t, dir, "-f", "t1")
	defInfo2, _ := os.Stat(defObj)
	t1Info2, _ := os.Stat(t1Obj)
	if !defInfo1.ModTime().Equal(defInfo2.ModTime()) {
		t.Fatalf("default target unexpectedly modified during single-target sync")
	}
	if !t1Info2.ModTime().After(t1Info1.ModTime()) {
		t.Fatalf("t1 object modtime not updated under force sync")
	}
}

// TestCLISyncNoBinaryTarget ensures a target without a binary (sync-only) still syncs successfully
// and produces the expected symlink/object files.
func TestCLISyncNoBinaryTarget(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubClone(t, "CONTENT")
	runSyncCLI(t, dir, "nobin")
	link := filepath.Join(dir, ".duck", "nobin", "nobin")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink for nobin target")
	}
}

// TestSyncWithConfigFlag verifies that the --config flag works with the sync command
func TestSyncWithConfigFlag(t *testing.T) {
	dir := t.TempDir()

	// Create a custom config file
	customConfig := `version: 1
default: custom

targets:
  custom:
    description: custom target
    binary: echo
    fileFlag: -f
    template:
      repo: repoCustom
      ref: main
      path: custom.tpl
    variables: {}
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(customConfig), 0o644); err != nil {
		t.Fatalf("write custom config: %v", err)
	}

	// Provide clone stub that creates custom.tpl
	saved := getCloneFunc()
	setCloneFunc(func(repo, ref, intoDir string) (string, error) {
		repoDir := filepath.Join(intoDir, "repo")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return "", err
		}
		// Create the custom.tpl file that the config expects
		p := filepath.Join(repoDir, "custom.tpl")
		os.WriteFile(p, []byte("CUSTOM_CONTENT\n"), 0o644)
		return repoDir, nil
	})
	t.Cleanup(func() { setCloneFunc(saved) })

	// Change to test directory
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	// Reset global state
	configPath = ""

	// Run sync with --config flag
	rootCmd.SetArgs([]string{"sync", "--config", "custom.yaml"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("sync with --config failed: %v", err)
	}

	// Verify the custom target was synced
	link := filepath.Join(dir, ".duck", "custom", "custom")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink for custom target at %s", link)
	}

	// Verify content
	obj := getSymlinkTarget(t, link)
	content, err := os.ReadFile(obj)
	if err != nil {
		t.Fatalf("failed to read object file: %v", err)
	}
	if !strings.Contains(string(content), "CUSTOM_CONTENT") {
		t.Fatalf("expected CUSTOM_CONTENT in object file, got: %s", string(content))
	}
}
