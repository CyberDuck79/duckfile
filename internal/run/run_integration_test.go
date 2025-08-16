//nolint:errcheck
package run

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
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
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
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
	if err := Sync(cfg, "", false, defaultSecurityConfig()); err != nil {
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
	if err := Exec(cfg, "default", nil, defaultSecurityConfigIntegration()); err != nil {
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
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	// Should succeed (checksum matches)
	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration()); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Clean and ensure removal
	if err := Clean(cfg, "build"); err != nil {
		t.Fatalf("clean error: %v", err)
	}

	// Change template file to break checksum
	tampered := []byte("tampered")
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), tampered, 0o644)
		return dst, nil
	}
	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration()); err == nil {
		t.Fatalf("expected checksum error, got nil")
	}
	// Restore cloneFunc for next test
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}

	// recompute checksum
	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration()); err != nil {
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
	currentLogLevel = LogWarn

	if err := Exec(cfg, "build", nil, defaultSecurityConfigIntegration()); err != nil {
		os.Stderr = oldStderr
		w.Close()
		t.Fatalf("expected success with warning, got error: %v", err)
	}
	w.Close()
	os.Stderr = oldStderr
	output, _ := io.ReadAll(r)
	if !strings.Contains(string(output), "[duck][warn] template config (repo/ref/path/vars) changed but checksum is unchanged") {
		t.Fatalf("expected warning in output, got: %s", string(output))
	}
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
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}

	// Should succeed (checksum matches)
	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration()); err != nil {
		t.Fatalf("expected sync success, got error: %v", err)
	}

	// Clean and ensure removal
	if err := Clean(cfg, "build"); err != nil {
		t.Fatalf("clean error: %v", err)
	}

	// Change template file to break checksum
	tampered := []byte("tampered content")
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), tampered, 0o644)
		return dst, nil
	}
	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration()); err == nil {
		t.Fatalf("expected checksum error in sync, got nil")
	}

	// Restore cloneFunc for next test
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		os.WriteFile(filepath.Join(dst, "file.tpl"), content, 0o644)
		return dst, nil
	}

	// Should succeed again with correct checksum
	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration()); err != nil {
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
	currentLogLevel = LogWarn

	if err := Sync(cfg, "build", false, defaultSecurityConfigIntegration()); err != nil {
		os.Stderr = oldStderr
		w.Close()
		t.Fatalf("expected sync success with warning, got error: %v", err)
	}
	w.Close()
	os.Stderr = oldStderr
	output, _ := io.ReadAll(r)
	if !strings.Contains(string(output), "[duck][warn] template config (repo/ref/path/vars) changed but checksum is unchanged") {
		t.Fatalf("expected warning in sync output, got: %s", string(output))
	}
}
