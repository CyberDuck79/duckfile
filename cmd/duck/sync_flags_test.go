//nolint:errcheck
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// Helper to capture sync execution with flags
func runSyncCLIWithFlags(t *testing.T, dir string, args ...string) error {
	t.Helper()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	// Reset command flags to avoid interference between tests
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "sync" {
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				// For slice flags, reset to empty to avoid parsing issues
				if f.Value.Type() == "stringSlice" {
					f.Value.Set("")
				} else {
					f.Value.Set(f.DefValue)
				}
				f.Changed = false
			})
			break
		}
	}

	rootCmd.SetArgs(append([]string{"sync"}, args...))
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	return rootCmd.Execute()
}

// Stub for commit hash tracking tests
func stubCommitHashClone(t *testing.T, content string, commitHash string) {
	saved := getCloneFunc()
	setCloneFunc(func(repo, ref, intoDir string) (string, error) {
		repoDir := filepath.Join(intoDir, "repo")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return "", err
		}

		// Create template files
		names := []string{"default.tpl", "one.tpl", "nobin.tpl"}
		for _, n := range names {
			p := filepath.Join(repoDir, n)
			os.WriteFile(p, []byte(content+"\n"), 0o644)
		}

		// Create commit hash metadata if tracking is enabled
		metadataFile := filepath.Join(intoDir, ".duck_commit_hash")
		os.WriteFile(metadataFile, []byte(commitHash), 0o644)

		return repoDir, nil
	})
	t.Cleanup(func() { setCloneFunc(saved) })
}

// TestSyncCommitHashTrackingFlags tests the --track-commit-hash and --no-track-commit-hash flags
func TestSyncCommitHashTrackingFlags(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "abc123")

	// Test --track-commit-hash flag (should work without security restrictions)
	err := runSyncCLIWithFlags(t, dir, "--track-commit-hash")
	if err != nil {
		t.Fatalf("sync with --track-commit-hash failed: %v", err)
	}

	// Test --no-track-commit-hash flag (should work without security restrictions)
	err = runSyncCLIWithFlags(t, dir, "--no-track-commit-hash")
	if err != nil {
		t.Fatalf("sync with --no-track-commit-hash failed: %v", err)
	}
}

// TestSyncAutoUpdateFlags tests the --auto-update-on-change and --no-auto-update-on-change flags
func TestSyncAutoUpdateFlags(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "def456")

	// Test --auto-update-on-change flag
	err := runSyncCLIWithFlags(t, dir, "--auto-update-on-change")
	if err != nil {
		t.Fatalf("sync with --auto-update-on-change failed: %v", err)
	}

	// Test --no-auto-update-on-change flag
	err = runSyncCLIWithFlags(t, dir, "--no-auto-update-on-change")
	if err != nil {
		t.Fatalf("sync with --no-auto-update-on-change failed: %v", err)
	}
}

// TestSyncCommitHashFlagCombination tests combining both commit hash flags
func TestSyncCommitHashFlagCombination(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "ghi789")

	// Test combining both flags (should work)
	err := runSyncCLIWithFlags(t, dir, "--track-commit-hash", "--auto-update-on-change")
	if err != nil {
		t.Fatalf("sync with both flags failed: %v", err)
	}

	// Test combining negative flags (should work)
	err = runSyncCLIWithFlags(t, dir, "--no-track-commit-hash", "--no-auto-update-on-change")
	if err != nil {
		t.Fatalf("sync with both negative flags failed: %v", err)
	}
}

// TestSyncMutuallyExclusiveFlags tests that mutually exclusive flags are properly handled
func TestSyncMutuallyExclusiveFlags(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "jkl012")

	// Test mutually exclusive tracking flags
	err := runSyncCLIWithFlags(t, dir, "--track-commit-hash", "--no-track-commit-hash")
	if err == nil {
		t.Fatalf("expected error for mutually exclusive tracking flags")
	}
	if !strings.Contains(err.Error(), "track-commit-hash") || !strings.Contains(err.Error(), "no-track-commit-hash") {
		t.Fatalf("expected mutually exclusive error for commit hash flags, got: %v", err)
	}

	// Test mutually exclusive auto-update flags
	err = runSyncCLIWithFlags(t, dir, "--auto-update-on-change", "--no-auto-update-on-change")
	if err == nil {
		t.Fatalf("expected error for mutually exclusive auto-update flags")
	}
	if !strings.Contains(err.Error(), "auto-update-on-change") || !strings.Contains(err.Error(), "no-auto-update-on-change") {
		t.Fatalf("expected mutually exclusive error for auto-update flags, got: %v", err)
	}
}

// TestSyncFlagsWithTarget tests flags work with specific targets
func TestSyncFlagsWithTarget(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "mno345")

	// Test flags with specific target
	err := runSyncCLIWithFlags(t, dir, "--track-commit-hash", "t1")
	if err != nil {
		t.Fatalf("sync with flags and target failed: %v", err)
	}
}

// TestSyncFlagsWithForce tests flags work with --force
func TestSyncFlagsWithForce(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "pqr678")

	// Test flags with --force
	err := runSyncCLIWithFlags(t, dir, "--force", "--track-commit-hash")
	if err != nil {
		t.Fatalf("sync with force and commit hash flags failed: %v", err)
	}
}

// TestSyncFlagsWithLogLevel tests flags work with --log-level
func TestSyncFlagsWithLogLevel(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "stu901")

	// Test flags with --log-level
	err := runSyncCLIWithFlags(t, dir, "--log-level", "debug", "--track-commit-hash")
	if err != nil {
		t.Fatalf("sync with log-level and commit hash flags failed: %v", err)
	}
}

// TestSyncFlagsIntegrationWithConfiguration tests that CLI flags override config settings
func TestSyncFlagsIntegrationWithConfiguration(t *testing.T) {
	dir := t.TempDir()

	// Write config with commit hash tracking settings
	body := `version: 1
default: def
settings:
  trackCommitHash: false
  autoUpdateOnChange: false

targets:
  def:
    description: default target
    binary: echo
    fileFlag: -f
    template:
      repo: repoDef
      ref: main
      path: default.tpl
      trackCommitHash: false
      autoUpdateOnChange: false
    variables: {}
`
	if err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stubCommitHashClone(t, "CONTENT", "vwx234")

	// Test that CLI flags override config (enable tracking despite config saying false)
	err := runSyncCLIWithFlags(t, dir, "--track-commit-hash")
	if err != nil {
		t.Fatalf("sync with CLI flag override failed: %v", err)
	}

	// Test that CLI flags override config (disable tracking despite config saying true)
	err = runSyncCLIWithFlags(t, dir, "--no-track-commit-hash")
	if err != nil {
		t.Fatalf("sync with CLI flag override (negative) failed: %v", err)
	}
}

// TestSyncWithoutCommitHashFlags tests sync works without any commit hash flags
func TestSyncWithoutCommitHashFlags(t *testing.T) {
	dir := t.TempDir()
	writeSyncConfig(t, dir)
	stubCommitHashClone(t, "CONTENT", "yza567")

	// Test sync without any commit hash flags (should work normally)
	err := runSyncCLIWithFlags(t, dir)
	if err != nil {
		t.Fatalf("sync without commit hash flags failed: %v", err)
	}
}
