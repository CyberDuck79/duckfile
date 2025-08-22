//nolint:errcheck
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestCommitHashMetadataOperations tests reading, writing, and checking commit hash metadata.
func TestCommitHashMetadataOperations(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")

	// Create the object directory
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create object directory: %v", err)
	}

	// Test hasCommitHashMetadata when file doesn't exist
	if hasCommitHashMetadata(objDir) {
		t.Fatal("hasCommitHashMetadata should return false when file doesn't exist")
	}

	// Test reading when file doesn't exist
	commitHash, err := readCommitHashMetadata(objDir)
	if err != nil {
		t.Fatalf("readCommitHashMetadata should not error when file doesn't exist: %v", err)
	}
	if commitHash != "" {
		t.Fatalf("readCommitHashMetadata should return empty string when file doesn't exist, got: %s", commitHash)
	}

	// Test writing metadata
	expectedHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	if err := writeCommitHashMetadata(objDir, expectedHash); err != nil {
		t.Fatalf("failed to write commit hash metadata: %v", err)
	}

	// Test hasCommitHashMetadata when file exists
	if !hasCommitHashMetadata(objDir) {
		t.Fatal("hasCommitHashMetadata should return true when file exists")
	}

	// Test reading existing metadata
	actualHash, err := readCommitHashMetadata(objDir)
	if err != nil {
		t.Fatalf("failed to read commit hash metadata: %v", err)
	}
	if actualHash != expectedHash {
		t.Fatalf("read hash doesn't match written hash: expected %s, got %s", expectedHash, actualHash)
	}

	// Test writing empty hash (should not create file)
	emptyDir := filepath.Join(tmp, "empty_cache")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("failed to create empty directory: %v", err)
	}

	if err := writeCommitHashMetadata(emptyDir, ""); err != nil {
		t.Fatalf("writeCommitHashMetadata should not error with empty hash: %v", err)
	}

	if hasCommitHashMetadata(emptyDir) {
		t.Fatal("hasCommitHashMetadata should return false after writing empty hash")
	}
}

// TestCommitHashMetadataWithWhitespace tests that metadata correctly handles whitespace.
func TestCommitHashMetadataWithWhitespace(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")

	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create object directory: %v", err)
	}

	// Write metadata file with whitespace
	metadataFile := filepath.Join(objDir, "commit.hash")
	hashWithWhitespace := "  a1b2c3d4e5f6789012345678901234567890abcd\n\t  "
	if err := os.WriteFile(metadataFile, []byte(hashWithWhitespace), 0o644); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	// Reading should trim whitespace
	actualHash, err := readCommitHashMetadata(objDir)
	if err != nil {
		t.Fatalf("failed to read commit hash metadata: %v", err)
	}

	expectedHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	if actualHash != expectedHash {
		t.Fatalf("expected trimmed hash %s, got %s", expectedHash, actualHash)
	}
}

// TestCacheKeyIntegrationWithConfig tests cache key computation using actual config resolution.
func TestCacheKeyIntegrationWithConfig(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: true, // Global setting enabled
		},
	}

	template := &config.Template{
		Repo: "https://github.com/test/repo.git",
		Ref:  "main",
		Path: "template.tpl",
		// TrackCommitHash not set, should inherit from global
	}

	// Test resolution
	trackCommitHash := config.ResolveTrackCommitHash(nil, template, cfg)
	if !trackCommitHash {
		t.Fatal("expected trackCommitHash to be true from global settings")
	}

	// Test cache key computation
	vars := map[string]any{"TEST": "value"}
	remoteKey1, err := computeRemoteCacheKey(template.Repo, template.Ref, template.Path)
	if err != nil {
		t.Fatalf("remote key1: %v", err)
	}
	renderedKey1, err := computeRenderedCacheKey(vars)
	if err != nil {
		t.Fatalf("rendered key1: %v", err)
	}

	// Change global setting and verify key changes
	cfg.Settings.TrackCommitHash = false
	trackCommitHash2 := config.ResolveTrackCommitHash(nil, template, cfg)
	if trackCommitHash2 {
		t.Fatal("expected trackCommitHash to be false from updated global settings")
	}

	remoteKey2, _ := computeRemoteCacheKey(template.Repo, template.Ref, template.Path)
	renderedKey2, _ := computeRenderedCacheKey(vars)
	if remoteKey1 != remoteKey2 {
		t.Fatalf("remote key should not change when tracking setting changes")
	}
	if renderedKey1 != renderedKey2 {
		t.Fatalf("rendered key should not change when tracking setting changes")
	}
}

// TestCommitHashMetadataStorage tests that commit hash metadata is stored during template preparation.
func TestCommitHashMetadataStorage(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, fmt.Sprintf("wd-%d", time.Now().UnixNano()))
	os.MkdirAll(workDir, 0o755)
	oldWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldWd)

	// Create a mock git repository
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)

	// Create a simple template
	templateContent := "hello {{.NAME}}"
	os.WriteFile(filepath.Join(repoDir, "test.tpl"), []byte(templateContent), 0o644)

	// Create a mock git repository with a commit
	os.WriteFile(filepath.Join(repoDir, ".git"), []byte("gitdir: ../git"), 0o644)
	gitDir := filepath.Join(tmp, "git")
	os.MkdirAll(gitDir, 0o755)

	// Create mock git objects and HEAD
	objectsDir := filepath.Join(gitDir, "objects")
	os.MkdirAll(objectsDir, 0o755)

	// Mock commit hash
	testCommitHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)

	refsDir := filepath.Join(gitDir, "refs", "heads")
	os.MkdirAll(refsDir, 0o755)
	os.WriteFile(filepath.Join(refsDir, "main"), []byte(testCommitHash+"\n"), 0o644)

	// Set up mock clone function to return our test repo
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		return repoDir, nil
	}
	defer func() { cloneFunc = origClone }()

	// Create config with commit hash tracking enabled
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
					Path: "test.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// Prepare template (this should store commit hash metadata)
	if _, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil); err != nil {
		t.Fatalf("failed to prepare template: %v", err)
	}

	// Check that commit hash metadata was stored (in remote cache dir)
	remoteKey, err := computeRemoteCacheKey(cfg.Targets["test"].Template.Repo, cfg.Targets["test"].Template.Ref, cfg.Targets["test"].Template.Path)
	if err != nil {
		t.Fatalf("remote key: %v", err)
	}
	remoteDir := filepath.Join(".duck", "objects", "remote", remoteKey)
	if !hasCommitHashMetadata(remoteDir) {
		t.Fatal("commit hash metadata should have been stored in remote dir")
	}
	// Verify the stored commit hash
	storedHash, err := readCommitHashMetadata(remoteDir)
	if err != nil {
		t.Fatalf("failed to read stored commit hash: %v", err)
	}

	if storedHash != testCommitHash {
		t.Fatalf("expected stored hash %s, got %s", testCommitHash, storedHash)
	}
}

// TestCommitHashMetadataAlwaysStored verifies that commit hash metadata is written even when tracking is disabled.
func TestCommitHashMetadataAlwaysStored(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, fmt.Sprintf("wd-%d", time.Now().UnixNano()))
	os.MkdirAll(workDir, 0o755)
	oldWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldWd)

	// Create a mock git repository
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)

	// Create a simple template
	templateContent := "hello {{.NAME}}"
	os.WriteFile(filepath.Join(repoDir, "test.tpl"), []byte(templateContent), 0o644)

	// Set up mock clone function
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		return repoDir, nil
	}
	defer func() { cloneFunc = origClone }()

	// Stub current commit hash retrieval so metadata writing always succeeds
	origGetCurrentCommitFunc := getCurrentCommitFunc
	getCurrentCommitFunc = func(workdir string) (string, error) { return "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil }
	defer func() { getCurrentCommitFunc = origGetCurrentCommitFunc }()

	// Create config with commit hash tracking DISABLED
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: false, // Explicitly disabled
		},
		Targets: map[string]config.Target{
			"test": {
				Template: config.Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "test.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// Prepare template (always stores commit hash metadata now, even if tracking disabled)
	if _, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil); err != nil {
		t.Fatalf("failed to prepare template: %v", err)
	}

	// Check that commit hash metadata WAS stored (policy: always capture on fetch; tracking controls validation only)
	remoteKey, err := computeRemoteCacheKey(cfg.Targets["test"].Template.Repo, cfg.Targets["test"].Template.Ref, cfg.Targets["test"].Template.Path)
	if err != nil {
		t.Fatalf("remote key: %v", err)
	}
	remoteDir := filepath.Join(".duck", "objects", "remote", remoteKey)
	if !hasCommitHashMetadata(remoteDir) {
		t.Fatal("commit hash metadata should have been stored even when tracking is disabled")
	}
}

// TestValidateCachedCommitHashUnchanged tests validation when commit hash hasn't changed.
func TestValidateCachedCommitHashUnchanged(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")
	os.MkdirAll(objDir, 0o755)

	testHash := "a1b2c3d4e5f6789012345678901234567890abcd"

	// Store initial commit hash
	writeCommitHashMetadata(objDir, testHash)

	// Mock getRemoteCommitFunc to return the same hash
	origGetRemoteCommitFunc := getRemoteCommitFunc
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return testHash, nil
	}
	defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

	// Validate - should return true (cache is valid)
	valid, err := validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not error: %v", err)
	}
	if !valid {
		t.Fatal("cache should be valid when commit hash is unchanged")
	}
}

// TestValidateCachedCommitHashChanged tests validation when commit hash has changed.
func TestValidateCachedCommitHashChanged(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")
	os.MkdirAll(objDir, 0o755)

	oldHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	newHash := "b2c3d4e5f6789012345678901234567890abcdef"

	// Store old commit hash
	writeCommitHashMetadata(objDir, oldHash)

	// Mock getRemoteCommitFunc to return a different hash
	origGetRemoteCommitFunc := getRemoteCommitFunc
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return newHash, nil
	}
	defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

	// Validate - should return false (cache is invalid)
	valid, err := validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not error: %v", err)
	}
	if valid {
		t.Fatal("cache should be invalid when commit hash has changed")
	}
}

// TestValidateCachedCommitHashNetworkError tests validation with network errors.
func TestValidateCachedCommitHashNetworkError(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")
	os.MkdirAll(objDir, 0o755)

	testHash := "a1b2c3d4e5f6789012345678901234567890abcd"
	writeCommitHashMetadata(objDir, testHash)

	// Mock getRemoteCommitFunc to return network error
	origGetRemoteCommitFunc := getRemoteCommitFunc
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return "", fmt.Errorf("network error: connection timeout")
	}
	defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

	// Validate - should return true (continue with cache on network error)
	valid, err := validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not error on network failure: %v", err)
	}
	if !valid {
		t.Fatal("cache should remain valid on network error")
	}
}

// TestValidateCachedCommitHashNoMetadata tests validation when no metadata exists.
func TestValidateCachedCommitHashNoMetadata(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")
	os.MkdirAll(objDir, 0o755)

	// No metadata file exists

	// Validate - should return true (skip validation when no metadata)
	valid, err := validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
	if err != nil {
		t.Fatalf("validation should not error when no metadata: %v", err)
	}
	if !valid {
		t.Fatal("cache should be valid when no metadata exists")
	}
}

// TestInvalidateCache tests cache invalidation functionality.
func TestInvalidateCache(t *testing.T) {
	tmp := t.TempDir()
	objDir := filepath.Join(tmp, "cache_object")
	os.MkdirAll(objDir, 0o755)

	// Create some cache files
	os.WriteFile(filepath.Join(objDir, "template"), []byte("content"), 0o644)
	writeCommitHashMetadata(objDir, "a1b2c3d4e5f6789012345678901234567890abcd")

	// Verify files exist
	if _, err := os.Stat(filepath.Join(objDir, "template")); err != nil {
		t.Fatal("template file should exist before invalidation")
	}
	if !hasCommitHashMetadata(objDir) {
		t.Fatal("metadata should exist before invalidation")
	}

	// Invalidate cache
	if err := invalidateCache(objDir); err != nil {
		t.Fatalf("cache invalidation should not error: %v", err)
	}

	// Verify directory is removed
	if _, err := os.Stat(objDir); !os.IsNotExist(err) {
		t.Fatal("cache directory should be removed after invalidation")
	}
}

// TestCommitHashValidationWithAutoUpdate tests end-to-end behavior with auto-update enabled.
func TestCommitHashValidationWithAutoUpdate(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, fmt.Sprintf("wd-%d", time.Now().UnixNano()))
	os.MkdirAll(workDir, 0o755)
	oldWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldWd)

	// Create mock repo and template
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "test.tpl"), []byte("hello {{.NAME}}"), 0o644)

	// Mock clone function
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		return repoDir, nil
	}
	defer func() { cloneFunc = origClone }()

	// Create config with tracking and auto-update enabled
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
					Path: "test.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// Initial hash
	initialHash := "a1b2c3d4e5f6789012345678901234567890abcd"

	// Mock getCurrentCommitFunc for initial render
	origGetCurrentCommitFunc := getCurrentCommitFunc
	getCurrentCommitFunc = func(workdir string) (string, error) {
		return initialHash, nil
	}
	defer func() { getCurrentCommitFunc = origGetCurrentCommitFunc }()

	// First run - should render and store commit hash
	result1, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("first run should succeed: %v", err)
	}

	remoteDir1Auto := filepath.Join(".duck", "objects", "remote", result1.RemoteKey)
	if !hasCommitHashMetadata(remoteDir1Auto) {
		t.Fatal("commit hash metadata should be stored in remote dir")
	}
	storedHash, _ := readCommitHashMetadata(remoteDir1Auto)
	if storedHash != initialHash {
		t.Fatalf("expected stored hash %s, got %s", initialHash, storedHash)
	}

	// Now simulate commit hash change
	newHash := "b2c3d4e5f6789012345678901234567890abcdef"

	// Mock getRemoteCommitFunc to return new hash
	origGetRemoteCommitFunc := getRemoteCommitFunc
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return newHash, nil
	}
	defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

	// Update getCurrentCommitFunc for re-render
	getCurrentCommitFunc = func(workdir string) (string, error) {
		return newHash, nil
	}

	// Second run - should detect hash change and auto-update
	result2, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("second run should succeed with auto-update: %v", err)
	}

	// Verify new hash is stored
	// Metadata persists in same remote dir (remote key stable)
	newStoredHash, _ := readCommitHashMetadata(remoteDir1Auto)
	if newStoredHash != newHash {
		t.Fatalf("expected new stored hash %s, got %s", newHash, newStoredHash)
	}

	// Verify same cache key but content was re-rendered (auto-update invalidated and re-created)
	if result1.RemoteKey != result2.RemoteKey {
		t.Fatalf("remote key changed unexpectedly")
	}
	if result1.RenderedKey != result2.RenderedKey {
		t.Fatalf("rendered key changed unexpectedly")
	}

	// Verify the object was actually re-created by checking timestamps or ensuring file exists
	if _, err := os.Stat(result2.ObjFile); err != nil {
		t.Fatal("rendered object should exist after auto-update")
	}
}

// TestCommitHashValidationWithoutAutoUpdate tests behavior when auto-update is disabled.
func TestCommitHashValidationWithoutAutoUpdate(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, fmt.Sprintf("wd-%d", time.Now().UnixNano()))
	os.MkdirAll(workDir, 0o755)
	oldWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldWd)

	// Create mock repo and template
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "test.tpl"), []byte("hello {{.NAME}}"), 0o644)

	// Mock clone function
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		return repoDir, nil
	}
	defer func() { cloneFunc = origClone }()

	// Create config with tracking enabled but auto-update DISABLED
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash:    true,
			AutoUpdateOnChange: false, // Disabled
		},
		Targets: map[string]config.Target{
			"test": {
				Template: config.Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "test.tpl",
				},
				Variables: map[string]config.VarValue{
					"NAME": config.NewLiteralVar("world"),
				},
			},
		},
	}

	// Initial hash
	initialHash := "a1b2c3d4e5f6789012345678901234567890abcd"

	// Mock getCurrentCommitFunc for initial render
	origGetCurrentCommitFunc := getCurrentCommitFunc
	getCurrentCommitFunc = func(workdir string) (string, error) {
		return initialHash, nil
	}
	defer func() { getCurrentCommitFunc = origGetCurrentCommitFunc }()

	// First run - should render and store commit hash
	_, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("first run should succeed: %v", err)
	}

	// Now simulate commit hash change
	newHash := "b2c3d4e5f6789012345678901234567890abcdef"

	// Mock getRemoteCommitFunc to return new hash
	origGetRemoteCommitFunc := getRemoteCommitFunc
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		return newHash, nil
	}
	defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

	// Second run - should fail with descriptive error
	_, err = prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err == nil {
		t.Fatal("second run should fail when hash changes and auto-update is disabled")
	}

	// Verify error message mentions the solution
	expectedPhrases := []string{
		"template has been updated remotely",
		"automatic updates are disabled",
		"Enable autoUpdateOnChange or re-run with --force",
	}

	errMsg := err.Error()
	for _, phrase := range expectedPhrases {
		if !strings.Contains(errMsg, phrase) {
			t.Errorf("error message should contain '%s', got: %s", phrase, errMsg)
		}
	}
}

// TestRemoteCacheInvalidationGC ensures that when auto-update triggers, the remote cache directory
// is fully invalidated (stale files removed) before new content is fetched.
func TestRemoteCacheInvalidationGC(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, fmt.Sprintf("wd-%d", time.Now().UnixNano()))
	os.MkdirAll(workDir, 0o755)
	oldWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldWd)

	// Create mock repo and template
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(repoDir, 0o755)
	tplPath := filepath.Join(repoDir, "t.tpl")
	os.WriteFile(tplPath, []byte("v1 {{.NAME}}"), 0o644)

	// Clone stub (returns repoDir)
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) { return repoDir, nil }
	defer func() { cloneFunc = origClone }()

	cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"t": {Template: config.Template{Repo: "stub", Ref: "main", Path: "t.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("A")}, RenderedPath: "out.txt"}}, Settings: &config.Settings{TrackCommitHash: true, AutoUpdateOnChange: true}}

	// Stub commit hash progression: first run hash1, second run hash2 to force auto-update
	hash1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	call := 0
	origGetRemote := getRemoteCommitFunc
	origGetCurrent := getCurrentCommitFunc
	getRemoteCommitFunc = func(repo, ref string) (string, error) {
		if call == 0 {
			return hash1, nil
		}
		return hash2, nil
	}
	getCurrentCommitFunc = func(workdir string) (string, error) {
		if call == 0 {
			return hash1, nil
		}
		return hash2, nil
	}
	defer func() { getRemoteCommitFunc = origGetRemote; getCurrentCommitFunc = origGetCurrent }()

	// First prepare
	res1, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	remoteDir := filepath.Join(".duck", "objects", "remote", res1.RemoteKey)
	// Add an extra stale file that should vanish after invalidation
	staleFile := filepath.Join(remoteDir, "stale.tmp")
	if err := os.WriteFile(staleFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	if _, err := os.Stat(staleFile); err != nil {
		t.Fatalf("stale file missing pre-update: %v", err)
	}

	// Change template content to ensure we notice difference after update (though remote key constant)
	os.WriteFile(tplPath, []byte("v2 {{.NAME}}"), 0o644)
	call = 1 // subsequent remote/current commit calls return hash2

	// Second prepare triggers auto-update (hash change)
	res2, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if res1.RemoteKey != res2.RemoteKey {
		t.Fatalf("remote key changed unexpectedly (should remain stable)")
	}

	// Stale file should be gone
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Fatalf("stale file still present after auto-update")
	}
	// commit.hash should have new hash
	newHash, _ := readCommitHashMetadata(remoteDir)
	if newHash != hash2 {
		t.Fatalf("expected new commit hash %s got %s", hash2, newHash)
	}
	// Rendered file should reflect v2 content
	data, _ := os.ReadFile("out.txt")
	if !strings.Contains(string(data), "v2") {
		t.Fatalf("rendered output not updated to new template content: %s", string(data))
	}
}
