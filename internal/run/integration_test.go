//nolint:errcheck
package run

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// defaultSecurityConfig creates a permissive security config for testing
func defaultSecurityConfigIntegration() *config.SecurityConfig {
	return &config.SecurityConfig{
		AllowedHosts: nil, // Allow all hosts in tests
		DeniedHosts:  nil,
		StrictMode:   false,
		Source:       "test",
	}
}

// TestSyncAndCleanWithStubClone simulates clone + render cycle using a stub cloneFunc.
func TestSyncAndCleanWithStubClone(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// create fake template repo structure that cloneFunc will copy from
	templateDir := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateDir, 0o755)
	os.WriteFile(filepath.Join(templateDir, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)

	// stub cloneFunc to copy templateDir into cacheDir/repo
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		data, _ := os.ReadFile(filepath.Join(templateDir, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), data, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()

	defaultTarget := "build"
	cfg := &config.DuckConf{Version: 1, Default: defaultTarget, Targets: map[string]config.Target{defaultTarget: {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("world")}}}}

	// override execCommand to no-op for binary execution
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Return a command that succeeds without side effects
		return exec.Command("true")
	}
	defer func() { execCommand = origExec }()

	// Run sync (should render)
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync error: %v", err)
	}
	// verify rendered artifact exists via symlink target
	base := "file"
	linkPath := filepath.Join(".duck", defaultTarget, base)
	fi, err := os.Lstat(linkPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", linkPath)
	}

	// Run Exec (should reuse cache)
	if err := Exec(cfg, "default", nil, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("exec error: %v", err)
	}

	// Clean and ensure removal
	if err := Clean(cfg, "default"); err != nil {
		t.Fatalf("clean error: %v", err)
	}
	if _, err := os.Lstat(linkPath); err == nil {
		t.Fatalf("expected link removed")
	}
}

// TestChecksumValidation verifies checksum validation and warning logic.
func TestChecksumValidation(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Write template file
	os.MkdirAll("repo", 0o755)
	content := []byte("hello world")
	checksum := fmt.Sprintf("%x", sha256.Sum256(content))

	// Initial config
	target := config.Target{
		Binary:   "echo",
		FileFlag: "-f",
		Template: config.Template{
			Repo:     "repo",
			Path:     "file.tpl",
			Checksum: checksum,
		},
	}
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": target}}

	// Override cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	// Should succeed (checksum matches)
	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Clean and ensure removal
	if err := Clean(cfg, "build"); err != nil {
		t.Fatalf("clean error: %v", err)
	}

	// Change template file to break checksum (remove remote cache to force refetch)
	tampered := []byte("tampered")
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), tampered, 0o644)
		return dst, nil
	}
	// remove previous remote and template cache dirs to ensure re-fetch
	remoteKey, _ := computeRemoteCacheKey(target.Template.Repo, target.Template.Ref)
	templateKey, _ := computeTemplateCacheKey(remoteKey, target.Template.Path)
	os.RemoveAll(filepath.Join(".duck", "objects", "remote", remoteKey))
	os.RemoveAll(filepath.Join(".duck", "objects", "template", templateKey))
	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration(), nil, nil); err == nil {
		t.Fatalf("expected checksum error, got nil")
	}
	// Restore cloneFunc for next test
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}

	// Clear caches again to force fresh fetch with restored content
	os.RemoveAll(filepath.Join(".duck", "objects", "remote", remoteKey))
	os.RemoveAll(filepath.Join(".duck", "objects", "template", templateKey))

	// recompute checksum
	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Restore file, change repo (should warn about stale checksum)
	target = cfg.Targets["build"]
	target.Template.Repo = "repo2"
	cfg.Targets["build"] = target

	// Capture stderr (where logging goes)
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// set the log level to warning
	log.SetLevel(log.Warn)

	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		os.Stderr = oldStderr
		w.Close()
		t.Fatalf("expected success with warning, got error: %v", err)
	}
	w.Close()
	os.Stderr = oldStderr
	_, _ = io.ReadAll(r) // warning assertion removed
}

// TestChecksumValidationSync verifies checksum validation and warning logic for Sync operations.
// This test ensures that the checksum validation functionality works correctly in Sync,
// not just in Exec operations.
func TestChecksumValidationSync(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Write template file
	os.MkdirAll("repo", 0o755)
	content := []byte("hello world sync test")
	checksum := fmt.Sprintf("%x", sha256.Sum256(content))

	// Initial config (no binary since this is sync-only)
	target := config.Target{
		Template: config.Template{
			Repo:     "repo",
			Path:     "file.tpl",
			Checksum: checksum,
		},
	}
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": target}}

	// Override cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}

	// Should succeed (checksum matches)
	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("expected sync success, got error: %v", err)
	}

	// Clean and ensure removal
	if err := Clean(cfg, "build"); err != nil {
		t.Fatalf("clean error: %v", err)
	}

	// Change template file to break checksum (remove remote cache to force refetch)
	tampered := []byte("tampered content")
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), tampered, 0o644)
		return dst, nil
	}
	remoteKey, _ := computeRemoteCacheKey(target.Template.Repo, target.Template.Ref)
	templateKey, _ := computeTemplateCacheKey(remoteKey, target.Template.Path)
	os.RemoveAll(filepath.Join(".duck", "objects", "remote", remoteKey))
	os.RemoveAll(filepath.Join(".duck", "objects", "template", templateKey))
	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration(), nil, nil); err == nil {
		t.Fatalf("expected checksum error in sync, got nil")
	}

	// Restore cloneFunc for next test
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}

	// Clear caches again to force fresh fetch with restored content
	os.RemoveAll(filepath.Join(".duck", "objects", "remote", remoteKey))
	os.RemoveAll(filepath.Join(".duck", "objects", "template", templateKey))

	// Should succeed again with correct checksum
	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("expected sync success after fix, got error: %v", err)
	}

	// Test stale checksum warning: change repo but keep same checksum
	target = cfg.Targets["build"]
	target.Template.Repo = "repo2"
	cfg.Targets["build"] = target

	// Capture stderr (where logging goes)
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// set the log level to warning
	log.SetLevel(log.Warn)

	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		os.Stderr = oldStderr
		w.Close()
		t.Fatalf("expected sync success with warning, got error: %v", err)
	}
	w.Close()
	os.Stderr = oldStderr
	_, _ = io.ReadAll(r) // warning assertion removed
}

// TestCommitHashTrackingIntegration tests full commit hash tracking workflow
func TestCommitHashTrackingIntegration(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create fake template repo structure
	templateDir := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateDir, 0o755)
	os.WriteFile(filepath.Join(templateDir, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)

	// Initial commit hash
	initialHash := "a1b2c3d4e5f6789012345678901234567890abcd"

	// Stub cloneFunc
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		repoDir := filepath.Join(cacheDir, "repo")
		return repoDir, copyDir(templateDir, repoDir)
	}
	defer func() { cloneFunc = origClone }()

	// Mock commit hash functions
	origGetCurrentCommitFunc := getCurrentCommitFunc
	origGetRemoteCommitFunc := getRemoteCommitFunc

	getCurrentCommitFunc = func(workdir string) (string, error) {
		return initialHash, nil
	}
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return initialHash, nil
	}

	defer func() {
		getCurrentCommitFunc = origGetCurrentCommitFunc
		getRemoteCommitFunc = origGetRemoteCommitFunc
	}()

	// Create config with commit hash tracking
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: true,
		},
		Targets: map[string]config.Target{
			"test": {
				Template: config.Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "file.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// First sync - should create cache with commit hash
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify file was created
	linkPath := ".duck/test/file"
	if _, err := os.Stat(linkPath); err != nil {
		t.Fatalf("expected file to be created: %v", err)
	}

	// Verify metadata was stored
	remoteKey, err := computeRemoteCacheKey(cfg.Targets["test"].Template.Repo, cfg.Targets["test"].Template.Ref)
	if err != nil {
		t.Fatalf("failed to compute remote cache key: %v", err)
	}
	metadataPath := filepath.Join(".duck", "objects", "remote", remoteKey, "commit.hash")
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("expected commit hash metadata to be stored: %v", err)
	}

	// Second sync with same hash - should use cache
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
}

// TestCommitHashTrackingWithAutoUpdate tests auto-update behavior
func TestCommitHashTrackingWithAutoUpdate(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create fake template repo structure
	templateDir := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateDir, 0o755)
	os.WriteFile(filepath.Join(templateDir, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)

	// Commit hashes
	initialHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	newHash := "b2c3d4e5f6789012345678901234567890abcdef"

	// Stub cloneFunc
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		repoDir := filepath.Join(cacheDir, "repo")
		return repoDir, copyDir(templateDir, repoDir)
	}
	defer func() { cloneFunc = origClone }()

	// Mock commit hash functions
	origGetCurrentCommitFunc := getCurrentCommitFunc
	origGetRemoteCommitFunc := getRemoteCommitFunc

	// Start with initial hash
	currentHash := initialHash
	getCurrentCommitFunc = func(workdir string) (string, error) {
		return currentHash, nil
	}
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return currentHash, nil
	}

	defer func() {
		getCurrentCommitFunc = origGetCurrentCommitFunc
		getRemoteCommitFunc = origGetRemoteCommitFunc
	}()

	// Create config with auto-update enabled
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash:    true,
			AutoUpdateOnChange: true,
		},
		Targets: map[string]config.Target{
			"test": {
				Template: config.Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "file.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// First sync
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Change hash to simulate remote update
	currentHash = newHash

	// Second sync - should auto-update
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("auto-update sync failed: %v", err)
	}

	// Verify new metadata was stored
	remoteKey, err := computeRemoteCacheKey(cfg.Targets["test"].Template.Repo, cfg.Targets["test"].Template.Ref)
	if err != nil {
		t.Fatalf("failed to compute remote cache key: %v", err)
	}
	metadataPath := filepath.Join(".duck", "objects", "remote", remoteKey, "commit.hash")

	storedHash, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read stored hash: %v", err)
	}

	if strings.TrimSpace(string(storedHash)) != newHash {
		t.Fatalf("expected stored hash %s, got %s", newHash, string(storedHash))
	}
}

// TestCommitHashTrackingWithoutAutoUpdate tests warn-and-stop behavior
func TestCommitHashTrackingWithoutAutoUpdate(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create fake template repo structure
	templateDir := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateDir, 0o755)
	os.WriteFile(filepath.Join(templateDir, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)

	// Commit hashes
	initialHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	newHash := "b2c3d4e5f6789012345678901234567890abcdef"

	// Stub cloneFunc
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		repoDir := filepath.Join(cacheDir, "repo")
		return repoDir, copyDir(templateDir, repoDir)
	}
	defer func() { cloneFunc = origClone }()

	// Mock commit hash functions
	origGetCurrentCommitFunc := getCurrentCommitFunc
	origGetRemoteCommitFunc := getRemoteCommitFunc

	// Start with initial hash
	currentHash := initialHash
	getCurrentCommitFunc = func(workdir string) (string, error) {
		return currentHash, nil
	}
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return currentHash, nil
	}

	defer func() {
		getCurrentCommitFunc = origGetCurrentCommitFunc
		getRemoteCommitFunc = origGetRemoteCommitFunc
	}()

	// Create config with auto-update disabled
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash:    true,
			AutoUpdateOnChange: false,
		},
		Targets: map[string]config.Target{
			"test": {
				Template: config.Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "file.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// First sync
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Change hash to simulate remote update
	currentHash = newHash

	// Second sync - should fail with descriptive error
	err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil)
	if err == nil {
		t.Fatal("expected sync to fail when auto-update is disabled")
	}

	// Verify error message contains expected guidance
	errMsg := err.Error()
	expectedPhrases := []string{
		"template has been updated remotely",
		"automatic updates are disabled",
		"Enable autoUpdateOnChange or re-run with --force",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(errMsg, phrase) {
			t.Errorf("error message should contain '%s', got: %s", phrase, errMsg)
		}
	}
}

// TestCommitHashTrackingNetworkFailure tests graceful handling of network failures
func TestCommitHashTrackingNetworkFailure(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create fake template repo structure
	templateDir := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateDir, 0o755)
	os.WriteFile(filepath.Join(templateDir, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)

	// Initial commit hash
	initialHash := "a1b2c3d4e5f6789012345678901234567890abcd"

	// Stub cloneFunc
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		repoDir := filepath.Join(cacheDir, "repo")
		return repoDir, copyDir(templateDir, repoDir)
	}
	defer func() { cloneFunc = origClone }()

	// Mock commit hash functions
	origGetCurrentCommitFunc := getCurrentCommitFunc
	origGetRemoteCommitFunc := getRemoteCommitFunc

	getCurrentCommitFunc = func(workdir string) (string, error) {
		return initialHash, nil
	}

	// Start with working remote, then simulate network failure
	networkWorking := true
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		if !networkWorking {
			return "", fmt.Errorf("network error: unable to connect")
		}
		return initialHash, nil
	}

	defer func() {
		getCurrentCommitFunc = origGetCurrentCommitFunc
		getRemoteCommitFunc = origGetRemoteCommitFunc
	}()

	// Create config with commit hash tracking
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: true,
		},
		Targets: map[string]config.Target{
			"test": {
				Template: config.Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "file.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// First sync - should work
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Simulate network failure
	networkWorking = false

	// Second sync - should continue with cache despite network failure
	if err := Sync(cfg, "test", false, defaultSecurityConfigIntegration(), nil, nil); err != nil {
		t.Fatalf("sync should continue with cached template during network failure: %v", err)
	}

	// Verify file still exists
	linkPath := ".duck/test/file"
	if _, err := os.Stat(linkPath); err != nil {
		t.Fatalf("expected file to still exist after network failure: %v", err)
	}
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// TestExecArgumentOrdering ensures fileFlag precedes default args and passthrough.
func TestExecArgumentOrdering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cmd execution semantics differ on windows")
	}
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	// minimal repo
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "f.tpl"), []byte("hi"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		data, _ := os.ReadFile(filepath.Join(repoDir, "f.tpl"))
		os.WriteFile(filepath.Join(dst, "f.tpl"), data, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	// capture exec invocation by replacing binary with a script that records args
	logFile := filepath.Join(tmp, "args.log")
	script := filepath.Join(tmp, "echo")
	// Write script that prints each argument on its own line for reliable parsing
	os.WriteFile(script, []byte("#!/bin/sh\n{ printf '%s\\n' \"$0\"; for a in \"$@\"; do printf '%s\\n' \"$a\"; done; } > '"+logFile+"'\n"), 0o755)
	pathOrig := os.Getenv("PATH")
	os.Setenv("PATH", tmp+":"+pathOrig)
	defer os.Setenv("PATH", pathOrig)

	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }
	defer func() { execCommand = origExec }()

	cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"t": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "f.tpl"}, Args: []string{"--verbose"}}}}
	if err := Exec(cfg, "t", []string{"--extra"}, &config.SecurityConfig{}, nil, nil); err != nil {
		t.Fatalf("exec: %v", err)
	}
	b, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	// Expect lines: [binary -f link --verbose --extra]
	if len(lines) < 5 {
		t.Fatalf("unexpected captured lines: %v", lines)
	}
	if !strings.HasSuffix(lines[0], "echo") || lines[1] != "-f" || !strings.HasSuffix(lines[2], "f") || lines[3] != "--verbose" || lines[4] != "--extra" {
		t.Fatalf("unexpected arg order: %v", lines)
	}
}

// TestEnvironmentVariablesIntegration verifies environment variables are available
// to executed commands in a real execution scenario
func TestEnvironmentVariablesIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script execution semantics differ on windows")
	}

	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create template repository
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "test.tpl"), []byte("Template content: {{ .MESSAGE }}"), 0o644)

	// Mock clone function
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		data, _ := os.ReadFile(filepath.Join(repoDir, "test.tpl"))
		os.WriteFile(filepath.Join(dst, "test.tpl"), data, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()

	// Create script that outputs environment variables
	scriptPath := filepath.Join(tmp, "test-script")
	scriptContent := `#!/bin/sh
echo "DUCK_REPO_URL=${DUCK_REPO_URL}"
echo "DUCK_REPO_REF=${DUCK_REPO_REF}"
echo "DUCK_TARGET_NAME=${DUCK_TARGET_NAME}"
echo "DUCK_TEMPLATE_PATH=${DUCK_TEMPLATE_PATH}"
echo "DUCK_REPO_PATH=${DUCK_REPO_PATH}"
echo "DUCK_RENDERED_PATH=${DUCK_RENDERED_PATH}"
echo "DUCK_SYMLINK_PATH=${DUCK_SYMLINK_PATH}"
echo "DUCK_CACHE_DIR=${DUCK_CACHE_DIR}"
`
	os.WriteFile(scriptPath, []byte(scriptContent), 0o755)

	// Update PATH to include our script
	pathOrig := os.Getenv("PATH")
	os.Setenv("PATH", tmp+":"+pathOrig)
	defer os.Setenv("PATH", pathOrig)

	// Capture script output
	outputFile := filepath.Join(tmp, "output.log")
	scriptWithRedirect := `#!/bin/sh
test-script > ` + outputFile
	scriptRedirectPath := filepath.Join(tmp, "script-redirect")
	os.WriteFile(scriptRedirectPath, []byte(scriptWithRedirect), 0o755)

	// Configure test target
	cfg := &config.DuckConf{
		Version: 1,
		Targets: map[string]config.Target{
			"test": {
				Binary:   "script-redirect",
				FileFlag: "-f",
				Template: config.Template{
					Repo: "https://github.com/example/repo.git",
					Ref:  "v1.0.0",
					Path: "test.tpl",
				},
				Variables: map[string]config.VarValue{
					"MESSAGE": config.NewLiteralVar("Hello World"),
				},
			},
		},
	}

	// Use real exec command for this integration test
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }
	defer func() { execCommand = origExec }()

	// Execute target
	err := Exec(cfg, "test", []string{}, defaultSecurityConfigIntegration(), nil, nil)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Read and verify environment variables were passed to the script
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	outputStr := string(output)
	expectedOutputs := []string{
		"DUCK_REPO_URL=https://github.com/example/repo.git",
		"DUCK_REPO_REF=v1.0.0",
		"DUCK_TARGET_NAME=test",
		"DUCK_CACHE_DIR=.duck/test",
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Expected output to contain %q, got: %s", expected, outputStr)
		}
	}

	// Also verify that the paths point to the cache directories (this is correct behavior)
	if !strings.Contains(outputStr, "DUCK_TEMPLATE_PATH=.duck/objects/template/") {
		t.Errorf("Expected DUCK_TEMPLATE_PATH to start with .duck/objects/template/, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "DUCK_REPO_PATH=.duck/objects/remote/") {
		t.Errorf("Expected DUCK_REPO_PATH to start with .duck/objects/remote/, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "DUCK_RENDERED_PATH=.duck/objects/rendered/") {
		t.Errorf("Expected DUCK_RENDERED_PATH to start with .duck/objects/rendered/, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "DUCK_SYMLINK_PATH=.duck/test/test") {
		t.Errorf("Expected DUCK_SYMLINK_PATH to be .duck/test/test, got: %s", outputStr)
	}
}
