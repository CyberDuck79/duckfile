//nolint:errcheck
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestComputeCacheKeyWithCommitHashTracking verifies that the commit hash tracking setting
// affects cache key computation to ensure cache invalidation when tracking settings change.
func TestComputeCacheKeyWithCommitHashTracking(t *testing.T) {
	vars := map[string]any{"VAR": "value"}

	// Same inputs with different tracking settings should produce different keys
	keyWithTracking, err := computeCacheKey("repo", "main", "path.tpl", vars, true)
	if err != nil {
		t.Fatalf("failed to compute cache key with tracking: %v", err)
	}

	keyWithoutTracking, err := computeCacheKey("repo", "main", "path.tpl", vars, false)
	if err != nil {
		t.Fatalf("failed to compute cache key without tracking: %v", err)
	}

	if keyWithTracking == keyWithoutTracking {
		t.Fatalf("cache keys should differ when tracking settings change: %s vs %s", keyWithTracking, keyWithoutTracking)
	}

	// Same settings should produce same key
	keyWithTracking2, err := computeCacheKey("repo", "main", "path.tpl", vars, true)
	if err != nil {
		t.Fatalf("failed to compute cache key with tracking (second time): %v", err)
	}

	if keyWithTracking != keyWithTracking2 {
		t.Fatalf("cache keys should be deterministic: %s vs %s", keyWithTracking, keyWithTracking2)
	}
}

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
	key1, err := computeCacheKey(template.Repo, template.Ref, template.Path, vars, trackCommitHash)
	if err != nil {
		t.Fatalf("failed to compute cache key: %v", err)
	}

	// Change global setting and verify key changes
	cfg.Settings.TrackCommitHash = false
	trackCommitHash2 := config.ResolveTrackCommitHash(nil, template, cfg)
	if trackCommitHash2 {
		t.Fatal("expected trackCommitHash to be false from updated global settings")
	}

	key2, err := computeCacheKey(template.Repo, template.Ref, template.Path, vars, trackCommitHash2)
	if err != nil {
		t.Fatalf("failed to compute cache key with updated settings: %v", err)
	}

	if key1 == key2 {
		t.Fatalf("cache keys should differ when trackCommitHash setting changes")
	}
}

// TestCommitHashMetadataStorage tests that commit hash metadata is stored during template preparation.
func TestCommitHashMetadataStorage(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
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
	result, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to prepare template: %v", err)
	}

	// Check that commit hash metadata was stored
	objDir := filepath.Dir(result.ObjFile)
	if !hasCommitHashMetadata(objDir) {
		t.Fatal("commit hash metadata should have been stored")
	}

	// Verify the stored commit hash
	storedHash, err := readCommitHashMetadata(objDir)
	if err != nil {
		t.Fatalf("failed to read stored commit hash: %v", err)
	}

	if storedHash != testCommitHash {
		t.Fatalf("expected stored hash %s, got %s", testCommitHash, storedHash)
	}
}

// TestCommitHashMetadataNotStoredWhenDisabled verifies that metadata is not stored when tracking is disabled.
func TestCommitHashMetadataNotStoredWhenDisabled(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
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

	// Prepare template (this should NOT store commit hash metadata)
	result, err := prepareAndRenderTemplate("test", cfg.Targets["test"], cfg, false, &config.SecurityConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to prepare template: %v", err)
	}

	// Check that commit hash metadata was NOT stored
	objDir := filepath.Dir(result.ObjFile)
	if hasCommitHashMetadata(objDir) {
		t.Fatal("commit hash metadata should NOT have been stored when tracking is disabled")
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
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
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

	objDir1 := filepath.Dir(result1.ObjFile)
	if !hasCommitHashMetadata(objDir1) {
		t.Fatal("commit hash metadata should be stored")
	}

	// Verify stored hash
	storedHash, _ := readCommitHashMetadata(objDir1)
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
	objDir2 := filepath.Dir(result2.ObjFile)
	newStoredHash, _ := readCommitHashMetadata(objDir2)
	if newStoredHash != newHash {
		t.Fatalf("expected new stored hash %s, got %s", newHash, newStoredHash)
	}

	// Verify same cache key but content was re-rendered (auto-update invalidated and re-created)
	if result1.CacheKey != result2.CacheKey {
		t.Fatal("cache key should be the same (same configuration), but content was re-rendered")
	}

	// Verify the object was actually re-created by checking timestamps or ensuring file exists
	if _, err := os.Stat(result2.ObjFile); err != nil {
		t.Fatal("rendered object should exist after auto-update")
	}
}

// TestCommitHashValidationWithoutAutoUpdate tests behavior when auto-update is disabled.
func TestCommitHashValidationWithoutAutoUpdate(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
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
		"may be outdated",
		"autoUpdateOnChange: true",
		"--force flag",
		"duck clean",
	}

	errMsg := err.Error()
	for _, phrase := range expectedPhrases {
		if !contains(errMsg, phrase) {
			t.Errorf("error message should contain '%s', got: %s", phrase, errMsg)
		}
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
