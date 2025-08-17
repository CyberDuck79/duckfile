//nolint:errcheck
package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// defaultSecurityConfig creates a permissive security config for testing
func defaultSecurityConfig() *config.SecurityConfig {
	return &config.SecurityConfig{
		AllowedHosts: nil, // Allow all hosts in tests
		DeniedHosts:  nil,
		StrictMode:   false,
		Source:       "test",
	}
}

// helper to list object key dirs
func listObjectKeys(t *testing.T) []string {
	t.Helper()
	base := filepath.Join(".duck", "objects")
	entries, err := os.ReadDir(base)
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestSyncVariableChangePrunesOldKey verifies that changing a variable value
// generates a new cache key and prunes the previous object directory.
func TestSyncVariableChangePrunesOldKey(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	// source template
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)
	// stub clone copies current template file
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("one")}}}}
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync1: %v", err)
	}
	keys1 := listObjectKeys(t)
	if len(keys1) != 1 {
		t.Fatalf("expected 1 key got %v", keys1)
	}
	// change variable => new key
	cfg.Targets[cfg.Default].Variables["NAME"] = config.NewLiteralVar("two")
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync2: %v", err)
	}
	keys2 := listObjectKeys(t)
	if len(keys2) != 1 {
		t.Fatalf("expected 1 key after change got %v", keys2)
	}
	if keys1[0] == keys2[0] {
		t.Fatalf("expected different key after var change")
	}
}

// TestSyncIdempotentWithoutForce confirms that re-running sync with unchanged
// variables and template does not overwrite the existing rendered object.
func TestSyncIdempotentWithoutForce(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("v1 {{ .NAME }}"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("X")}}}}
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync1: %v", err)
	}
	// modify source template, but don't force
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("v2 {{ .NAME }}"), 0o644)
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync2: %v", err)
	}
	// object content should still be v1 because key unchanged and not forced
	link := filepath.Join(".duck", "build", "file")
	target, _ := os.Readlink(link)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	b, _ := os.ReadFile(target)
	if strings.HasPrefix(string(b), "v2") {
		t.Fatalf("unexpected re-render without force: %q", string(b))
	}
}

// TestSyncForceReRendersSameKey checks that the force flag triggers a re-render
// (new content) even when the cache key (inputs) are unchanged.
func TestSyncForceReRendersSameKey(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("force1 {{ .NAME }}"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("X")}}}}
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync1: %v", err)
	}
	link := filepath.Join(".duck", "build", "file")
	target, _ := os.Readlink(link)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	before, _ := os.ReadFile(target)
	// update source, force sync
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("force2 {{ .NAME }}"), 0o644)
	// small sleep to ensure mtime difference if needed
	time.Sleep(10 * time.Millisecond)
	if err := Sync(cfg, "", true, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync force: %v", err)
	}
	after, _ := os.ReadFile(target)
	if strings.EqualFold(string(before), string(after)) || !strings.HasPrefix(string(after), "force2") {
		t.Fatalf("expected forced re-render, got %q -> %q", string(before), string(after))
	}
}

// TestExecMissingBinaryError ensures executing a target lacking a binary returns
// a helpful guidance error instead of proceeding.
func TestExecMissingBinaryError(t *testing.T) {
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Template: config.Template{Repo: "r", Path: "file.tpl"}}}}
	if err := Exec(cfg, "default", nil, defaultSecurityConfig(), nil, nil); err == nil || !strings.Contains(err.Error(), "no binary configured") {
		t.Fatalf("expected missing binary error, got %v", err)
	}
}

// TestExecUnderlyingBinaryFailure stubs the underlying process to exit non-zero
// and asserts Exec surfaces a failure.
func TestExecUnderlyingBinaryFailure(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hi"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		data, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), data, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	// execCommand stub returns failing command (non-zero exit)
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("sh", "-c", "exit 5") }
	defer func() { execCommand = origExec }()
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "dummy", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}}}}
	if err := Exec(cfg, "default", nil, defaultSecurityConfig(), nil, nil); err == nil {
		t.Fatalf("expected failure from underlying binary")
	}
}

// TestRenderMissingVariableStrict validates that missing variables cause an
// error when AllowMissing is false.
func TestRenderMissingVariableStrict(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hello {{ .NAME }} {{ .OTHER }}"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		data, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), data, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("world")}}}}
	err := Sync(cfg, "default", false, defaultSecurityConfig(), nil, nil)
	if err == nil {
		t.Fatalf("expected render error for missing var")
	}
	if !strings.Contains(err.Error(), "missing") && !strings.Contains(err.Error(), "map has no entry") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnsureSymlinkReplacesFile ensures an existing regular file at the symlink
// location is replaced by the correct symlink.
func TestEnsureSymlinkReplacesFile(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("content"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		data, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), data, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	// Pre-create regular file at link path
	os.MkdirAll(filepath.Join(".duck", "build"), 0o755)
	os.WriteFile(filepath.Join(".duck", "build", "file"), []byte("old"), 0o644)
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}}}}
	if err := Sync(cfg, "build", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync: %v", err)
	}
	link := filepath.Join(".duck", "build", "file")
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("stat link: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink replacing file")
	}
}

// TestBrokenSymlinkUpdated creates a dangling symlink at the linkPath and
// confirms Sync replaces it with a valid one pointing to a freshly rendered object.
func TestBrokenSymlinkUpdated(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hello"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	// create broken symlink
	os.MkdirAll(filepath.Join(".duck", "build"), 0o755)
	link := filepath.Join(".duck", "build", "file")
	os.Symlink("../objects/missing-key/file", link)
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}}}}
	if err := Sync(cfg, "build", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync err: %v", err)
	}
	// link should now point to an existing object file
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(link), dest)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected repaired symlink target exists: %v", err)
	}
}

// TestCleanRemovesOnlyTargetArtifacts verifies cleaning a single target removes
// its symlink and object while preserving others.
func TestCleanRemovesOnlyTargetArtifacts(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	templateSrc := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("content {{ .V }}"), 0o644)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
		os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()

	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"V": config.NewLiteralVar("ONE")}}, "other": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"V": config.NewLiteralVar("TWO")}}}}
	if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("sync all: %v", err)
	}
	// capture keys
	keysBefore := listObjectKeys(t)
	if len(keysBefore) != 2 {
		t.Fatalf("expected 2 keys, got %v", keysBefore)
	}
	if err := Clean(cfg, "default"); err != nil {
		t.Fatalf("clean default: %v", err)
	}
	// default symlink gone
	if _, err := os.Lstat(filepath.Join(".duck", "build", "file")); err == nil {
		t.Fatalf("expected default symlink removed")
	}
	// other symlink still present
	if _, err := os.Lstat(filepath.Join(".duck", "other", "file")); err != nil {
		t.Fatalf("other symlink missing after clean: %v", err)
	}
	// objects count should be 1 now
	if keys := listObjectKeys(t); len(keys) != 1 {
		t.Fatalf("expected 1 remaining object, got %v", keys)
	}
}

// TestCacheInvalidationWithCommitHashTracking tests cache invalidation when commit hash tracking settings change
func TestCacheInvalidationWithCommitHashTracking(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create fake template repo structure
	templateDir := filepath.Join(tmp, "templateSrc")
	os.MkdirAll(templateDir, 0o755)
	os.WriteFile(filepath.Join(templateDir, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)

	// Mock git dir for commit hash
	gitDir := filepath.Join(templateDir, ".git")
	os.MkdirAll(gitDir, 0o755)
	testCommitHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	refsDir := filepath.Join(gitDir, "refs", "heads")
	os.MkdirAll(refsDir, 0o755)
	os.WriteFile(filepath.Join(refsDir, "main"), []byte(testCommitHash+"\n"), 0o644)

	// Stub cloneFunc to return our test repo
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		repoDir := filepath.Join(cacheDir, "repo")
		return repoDir, copyDirCache(templateDir, repoDir)
	}
	defer func() { cloneFunc = origClone }()

	// Mock commit hash functions for proper testing
	origGetCurrentCommitFunc := getCurrentCommitFunc
	origGetRemoteCommitFunc := getRemoteCommitFunc

	getCurrentCommitFunc = func(workdir string) (string, error) {
		return testCommitHash, nil
	}
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return testCommitHash, nil
	}

	defer func() {
		getCurrentCommitFunc = origGetCurrentCommitFunc
		getRemoteCommitFunc = origGetRemoteCommitFunc
	}()

	// Create target configuration
	target := config.Target{
		Template: config.Template{
			Repo: "https://github.com/test/repo.git",
			Ref:  "main",
			Path: "file.tpl",
		},
		Variables: map[string]config.VarValue{
			"NAME": config.NewLiteralVar("world"),
		},
	}

	// Config WITHOUT commit hash tracking
	cfg1 := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: false,
		},
		Targets: map[string]config.Target{
			"test": target,
		},
	}

	// First sync without commit hash tracking
	if err := Sync(cfg1, "test", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Get initial cache key
	vars1, err := resolveVariables(target.Variables)
	if err != nil {
		t.Fatalf("failed to resolve variables: %v", err)
	}
	cacheKey1, err := computeCacheKey(target.Template.Repo, target.Template.Ref, target.Template.Path, vars1, false)
	if err != nil {
		t.Fatalf("failed to compute cache key 1: %v", err)
	}

	objDir1 := filepath.Join(".duck", "objects", cacheKey1)
	if _, err := os.Stat(objDir1); err != nil {
		t.Fatalf("expected cache directory to exist: %v", err)
	}

	// Verify no commit hash metadata exists
	metadataPath1 := filepath.Join(objDir1, "commit.hash")
	if _, err := os.Stat(metadataPath1); err == nil {
		t.Fatal("expected no commit hash metadata without tracking")
	}

	// Config WITH commit hash tracking (same everything else)
	cfg2 := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: true,
		},
		Targets: map[string]config.Target{
			"test": target,
		},
	}

	// Second sync with commit hash tracking - should create new cache
	if err := Sync(cfg2, "test", false, defaultSecurityConfig(), nil, nil); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Get new cache key (should be different due to commit hash tracking)
	vars2, err := resolveVariables(target.Variables)
	if err != nil {
		t.Fatalf("failed to resolve variables: %v", err)
	}
	cacheKey2, err := computeCacheKey(target.Template.Repo, target.Template.Ref, target.Template.Path, vars2, true)
	if err != nil {
		t.Fatalf("failed to compute cache key 2: %v", err)
	}

	if cacheKey1 == cacheKey2 {
		t.Fatal("expected different cache keys when commit hash tracking changes")
	}

	objDir2 := filepath.Join(".duck", "objects", cacheKey2)
	if _, err := os.Stat(objDir2); err != nil {
		t.Fatalf("expected new cache directory to exist: %v", err)
	}

	// Verify commit hash metadata exists in new cache
	metadataPath2 := filepath.Join(objDir2, "commit.hash")
	if _, err := os.Stat(metadataPath2); err != nil {
		t.Fatalf("expected commit hash metadata with tracking: %v", err)
	}

	// Verify both cache directories exist (old cache not cleaned up immediately)
	// Note: This behavior depends on cache cleanup strategy, so we'll just check the new one exists
	if _, err := os.Stat(objDir2); err != nil {
		t.Fatalf("new cache directory should exist: %v", err)
	}
}

// TestCommitHashMetadataStorageBasic tests basic commit hash metadata storage and retrieval
func TestCommitHashMetadataStorageBasic(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create test object directory
	objDir := filepath.Join(tmp, "test-object")
	os.MkdirAll(objDir, 0o755)

	// Test writing and reading commit hash metadata
	testHash := "a1b2c3d4e5f6789012345678901234567890abcd"

	if err := writeCommitHashMetadata(objDir, testHash); err != nil {
		t.Fatalf("failed to write commit hash metadata: %v", err)
	}

	// Verify file was created
	metadataPath := filepath.Join(objDir, "commit.hash")
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("expected metadata file to exist: %v", err)
	}

	// Read back the metadata
	storedHash, err := readCommitHashMetadata(objDir)
	if err != nil {
		t.Fatalf("failed to read commit hash metadata: %v", err)
	}

	if storedHash != testHash {
		t.Fatalf("expected stored hash %s, got %s", testHash, storedHash)
	}

	// Test empty hash (should not create file)
	objDir2 := filepath.Join(tmp, "test-object2")
	os.MkdirAll(objDir2, 0o755)

	if err := writeCommitHashMetadata(objDir2, ""); err != nil {
		t.Fatalf("failed to write empty commit hash metadata: %v", err)
	}

	metadataPath2 := filepath.Join(objDir2, "commit.hash")
	if _, err := os.Stat(metadataPath2); err == nil {
		t.Fatal("expected no metadata file for empty hash")
	}

	// Test reading from directory without metadata
	emptyHash, err := readCommitHashMetadata(objDir2)
	if err != nil {
		t.Fatalf("failed to read from directory without metadata: %v", err)
	}

	if emptyHash != "" {
		t.Fatalf("expected empty hash from directory without metadata, got %s", emptyHash)
	}
}

// TestCacheKeyComputationWithCommitHashTracking tests that cache keys change when commit hash tracking is enabled/disabled
func TestCacheKeyComputationWithCommitHashTracking(t *testing.T) {
	// Test data
	repo := "https://github.com/test/repo.git"
	ref := "main"
	path := "test.tpl"
	vars := map[string]any{
		"NAME":    "world",
		"VERSION": "1.0.0",
	}

	// Compute cache key without commit hash tracking
	key1, err := computeCacheKey(repo, ref, path, vars, false)
	if err != nil {
		t.Fatalf("failed to compute cache key without tracking: %v", err)
	}

	// Compute cache key with commit hash tracking
	key2, err := computeCacheKey(repo, ref, path, vars, true)
	if err != nil {
		t.Fatalf("failed to compute cache key with tracking: %v", err)
	}

	// Keys should be different
	if key1 == key2 {
		t.Fatal("expected different cache keys when commit hash tracking changes")
	}

	// Keys should be consistent when called multiple times with same parameters
	key1Again, err := computeCacheKey(repo, ref, path, vars, false)
	if err != nil {
		t.Fatalf("failed to recompute cache key without tracking: %v", err)
	}

	key2Again, err := computeCacheKey(repo, ref, path, vars, true)
	if err != nil {
		t.Fatalf("failed to recompute cache key with tracking: %v", err)
	}

	if key1 != key1Again {
		t.Fatal("cache key without tracking should be consistent")
	}

	if key2 != key2Again {
		t.Fatal("cache key with tracking should be consistent")
	}

	// Verify keys are valid hex strings
	if len(key1) != 40 {
		t.Fatalf("expected 40-character hex key, got %d characters", len(key1))
	}

	if len(key2) != 40 {
		t.Fatalf("expected 40-character hex key, got %d characters", len(key2))
	}

	// Test with different variables
	vars2 := map[string]any{
		"NAME":    "universe",
		"VERSION": "2.0.0",
	}

	key3, err := computeCacheKey(repo, ref, path, vars2, false)
	if err != nil {
		t.Fatalf("failed to compute cache key with different vars: %v", err)
	}

	if key1 == key3 {
		t.Fatal("expected different cache keys with different variables")
	}
}

// TestCommitHashValidationEdgeCases tests edge cases in commit hash validation
func TestCommitHashValidationEdgeCases(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// Create test object directory
	objDir := filepath.Join(tmp, "test-object")
	os.MkdirAll(objDir, 0o755)

	// Mock getRemoteCommitFunc for testing
	origGetRemoteCommitFunc := getRemoteCommitFunc
	defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

	// Test 1: No stored hash (cache created without tracking)
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return "b2c3d4e5f6789012345678901234567890abcdef", nil
	}

	valid, err := validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not fail with no stored hash: %v", err)
	}
	if !valid {
		t.Fatal("validation should pass when no stored hash exists")
	}

	// Test 2: Network failure during remote hash retrieval
	writeCommitHashMetadata(objDir, "a1b2c3d4e5f6789012345678901234567890abcd")

	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return "", fmt.Errorf("network error: connection timeout")
	}

	valid, err = validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not fail on network error: %v", err)
	}
	if !valid {
		t.Fatal("validation should pass gracefully on network error")
	}

	// Test 3: Hash mismatch (should return false, no error)
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return "b2c3d4e5f6789012345678901234567890abcdef", nil
	}

	valid, err = validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not error on hash mismatch: %v", err)
	}
	if valid {
		t.Fatal("validation should return false when hashes don't match")
	}

	// Test 4: Hash match (should return true, no error)
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return "a1b2c3d4e5f6789012345678901234567890abcd", nil
	}

	valid, err = validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not error on hash match: %v", err)
	}
	if !valid {
		t.Fatal("validation should return true when hashes match")
	}
}

// copyDirCache recursively copies a directory for cache tests
func copyDirCache(src, dst string) error {
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

// end
