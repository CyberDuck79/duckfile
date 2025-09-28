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
	base := filepath.Join(".duck", "objects", "rendered")
	entries, err := os.ReadDir(base)
	if err != nil {
		return []string{}
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestSyncVariableChangePrunesOldKey verifies variable change produces new rendered key while remote reused.
func TestSyncVariableChangePrunesOldKey(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		// source template (relative inside temp wd)
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hello {{ .NAME }}"), 0o644)
		// stub clone copies current template file
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: &config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("one")}}}}
		if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("sync1: %v", err)
		}
		keys1 := listObjectKeys(t)
		if len(keys1) != 1 {
			t.Fatalf("expected 1 rendered key got %v", keys1)
		}
		// change variable => new key
		cfg.Targets[cfg.Default].Variables["NAME"] = config.NewLiteralVar("two")
		if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("sync2: %v", err)
		}
		keys2 := listObjectKeys(t)
		if len(keys2) != 1 {
			t.Fatalf("expected 1 rendered key after change got %v", keys2)
		}
		if keys1[0] == keys2[0] {
			t.Fatalf("expected different rendered key after var change")
		}
	})
}

// TestSyncIdempotentWithoutForce confirms that re-running sync with unchanged
// variables and template does not overwrite the existing rendered object.
func TestSyncIdempotentWithoutForce(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("v1 {{ .NAME }}"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: &config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("X")}}}}
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
	})
}

// TestSyncForceReRendersSameKey checks that the force flag triggers a re-render
// (new content) even when the cache key (inputs) are unchanged.
func TestSyncForceReRendersSameKey(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("force1 {{ .NAME }}"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: &config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("X")}}}}
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
	})
}

// TestExecMissingBinaryError ensures executing a target lacking a binary returns
// a helpful guidance error instead of proceeding.

// TestExecUnderlyingBinaryFailure stubs the underlying process to exit non-zero
// and asserts Exec surfaces a failure.

// TestCleanRemovesOnlyTargetArtifacts verifies cleaning a single target removes
// its symlink and object while preserving others.

// TestCommitHashMetadataStorageBasic tests basic commit hash metadata storage and retrieval
func TestCommitHashMetadataStorageBasic(t *testing.T) {
	withTempWD(t, func() {

		// Create test object directory (relative inside temp wd)
		objDir := filepath.Join("test-object")
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
		objDir2 := filepath.Join("test-object2")
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
	})
}

// TestRenderedKeyOnlyDependsOnVariables validates rendered key stability & variable sensitivity.
func TestRenderedKeyOnlyDependsOnVariables(t *testing.T) {
	vars1 := map[string]any{"A": 1, "B": "x"}
	vars2 := map[string]any{"A": 1, "B": "y"}
	key1, err := computeRenderedCacheKey(vars1)
	if err != nil {
		t.Fatalf("rk1: %v", err)
	}
	key1b, _ := computeRenderedCacheKey(vars1)
	if key1 != key1b {
		t.Fatalf("rendered key not deterministic")
	}
	key2, _ := computeRenderedCacheKey(vars2)
	if key1 == key2 {
		t.Fatalf("rendered key should differ when variables differ")
	}
}

// TestCommitHashValidationEdgeCases tests edge cases in commit hash validation
func TestCommitHashValidationEdgeCases(t *testing.T) {
	withTempWD(t, func() {

		// Create test object directory (relative)
		objDir := filepath.Join("test-object")
		os.MkdirAll(objDir, 0o755)

		// Mock getRemoteCommitFunc for testing
		origGetRemoteCommitFunc := getRemoteCommitFunc
		defer func() { getRemoteCommitFunc = origGetRemoteCommitFunc }()

		// Test 1: Network failure during remote hash retrieval
		writeCommitHashMetadata(objDir, "a1b2c3d4e5f6789012345678901234567890abcd")

		getRemoteCommitFunc = func(repo, ref string) (string, error) {
			return "", fmt.Errorf("network error: connection timeout")
		}

		valid, err := validateCachedCommitHash("https://github.com/test/repo.git", "main", objDir)
		if err != nil {
			t.Fatalf("validation should not fail on network error: %v", err)
		}
		if !valid {
			t.Fatal("validation should pass gracefully on network error")
		}

		// Test 2: Hash mismatch (should return false, no error)
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

		// Test 3: Hash match (should return true, no error)
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
	})
}

// TestPrepareTemplatePrunesOldRenderedCache verifies that prepareAndRenderTemplate removes
// the previous rendered cache directory when variables change (rendered key changes).
func TestPrepareTemplatePrunesOldRenderedCache(t *testing.T) {
	withTempWD(t, func() {

		// Fake repo with template
		repoDir := filepath.Join("repo")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("mkdir repo: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "t.tpl"), []byte("hi {{.NAME}}"), 0o644); err != nil {
			t.Fatalf("write tpl: %v", err)
		}

		// Stub clone
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		defer func() { cloneFunc = origClone }()

		// Initial target/config
		target := config.Target{Template: &config.Template{Repo: "stub", Path: "t.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("one")}, RenderedPath: "out.txt"}
		cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"t": target}}

		res1, err := prepareAndRenderTemplate("t", target, cfg, false, &config.SecurityConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("first prepare: %v", err)
		}
		oldRenderedKey := res1.RenderedKey
		oldDir := filepath.Join(".duck", "objects", "rendered", oldRenderedKey)
		if _, err := os.Stat(oldDir); err != nil {
			t.Fatalf("old rendered dir missing: %v", err)
		}

		// Change variable to force new rendered key
		target2 := target
		target2.Variables = map[string]config.VarValue{"NAME": config.NewLiteralVar("two")}
		cfg.Targets["t"] = target2
		res2, err := prepareAndRenderTemplate("t", target2, cfg, false, &config.SecurityConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("second prepare: %v", err)
		}
		if res2.RenderedKey == oldRenderedKey {
			t.Fatalf("rendered key did not change")
		}

		// Old directory should be pruned
		if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
			t.Fatalf("old rendered cache directory was not pruned: %v", err)
		}
		// New dir should exist
		newDir := filepath.Join(".duck", "objects", "rendered", res2.RenderedKey)
		if _, err := os.Stat(newDir); err != nil {
			t.Fatalf("new rendered dir missing: %v", err)
		}
		// Symlink should point to new rendered file
		data, err := os.ReadFile("out.txt")
		if err != nil {
			t.Fatalf("read out: %v", err)
		}
		if !strings.Contains(string(data), "two") {
			t.Fatalf("symlink not updated to new rendered content: %s", string(data))
		}
	})
}

// TestVariableChangeDoesNotRefetchRemote ensures variable-only changes do not cause a new remote fetch.
func TestVariableChangeDoesNotRefetchRemote(t *testing.T) {
	withTempWD(t, func() {

		// Fake repo with template
		repoDir := filepath.Join("repo")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("mkdir repo: %v", err)
		}
		content := "hello {{.NAME}}"
		if err := os.WriteFile(filepath.Join(repoDir, "t.tpl"), []byte(content), 0o644); err != nil {
			t.Fatalf("write tpl: %v", err)
		}

		// Mock clone
		origClone := cloneFunc
		cloneCalls := 0
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { cloneCalls++; return repoDir, nil }
		defer func() { cloneFunc = origClone }()

		target := config.Target{Template: &config.Template{Repo: "https://example/repo.git", Ref: "main", Path: "t.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("Alice")}, RenderedPath: "out.txt"}
		cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"t": target}}

		// First render
		if _, err := prepareAndRenderTemplate("t", target, cfg, false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Fatalf("first render: %v", err)
		}
		remoteKey, err := computeRemoteCacheKey(target.Template.Repo, target.Template.Ref, target.Template.Path)
		if err != nil {
			t.Fatalf("remote key: %v", err)
		}
		remoteDir := filepath.Join(".duck", "objects", "remote", remoteKey)
		if _, err := os.Stat(remoteDir); err != nil {
			t.Fatalf("remote dir missing: %v", err)
		}

		// Change only variable
		target2 := target
		target2.Variables = map[string]config.VarValue{"NAME": config.NewLiteralVar("Bob")}
		cfg.Targets["t"] = target2
		if _, err := prepareAndRenderTemplate("t", target2, cfg, false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Fatalf("second render: %v", err)
		}

		if cloneCalls != 1 {
			t.Fatalf("expected exactly one remote fetch, got %d", cloneCalls)
		}

		b, err := os.ReadFile("out.txt")
		if err != nil {
			t.Fatalf("read rendered: %v", err)
		}
		if !strings.Contains(string(b), "Bob") {
			t.Fatalf("render did not reflect variable change: %s", string(b))
		}
	})
}

// end
