//nolint:errcheck
package run

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

func TestFetchRemoteBasicSuccess(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		repoDir := makeRepoWithTemplate(t, "tpls/a.tpl", "hello")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		getCurrentCommitFunc = func(dir string) (string, error) { return "abcdef1234567890", nil }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "tpls/a.tpl"}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		if err := fetchRemote(false, target, target.Template, paths); err != nil {
			t.Fatalf("fetchRemote: %v", err)
		}
		content, err := os.ReadFile(paths.remoteTemplateFile)
		if err != nil {
			t.Fatalf("remote template missing: %v", err)
		}
		if string(content) != "hello" {
			t.Fatalf("unexpected content: %q", string(content))
		}
		if !hasCommitHashMetadata(paths.remoteDir) {
			t.Fatalf("expected commit hash metadata")
		}
	})
}

func TestFetchRemoteWithChecksumSuccess(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		repoDir := makeRepoWithTemplate(t, "f.tpl", "BODY")
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte("BODY")))
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "x", Path: "f.tpl", Checksum: sum}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		if err := fetchRemote(false, target, target.Template, paths); err != nil {
			t.Fatalf("fetchRemote: %v", err)
		}
		if _, err := os.Stat(filepath.Join(paths.remoteDir, "checksum.sha256")); err != nil {
			t.Fatalf("checksum file missing: %v", err)
		}
	})
}

func TestFetchRemoteChecksumMismatch(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		repoDir := makeRepoWithTemplate(t, "f.tpl", "BODY")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "x", Path: "f.tpl", Checksum: "deadbeef"}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		err := fetchRemote(false, target, target.Template, paths)
		if err == nil {
			t.Fatalf("expected checksum mismatch error")
		}
		if _, statErr := os.Stat(paths.remoteTemplateFile); !os.IsNotExist(statErr) {
			t.Fatalf("remote template should not exist after checksum failure")
		}
		if _, statErr := os.Stat(filepath.Join(paths.remoteDir, "checksum.sha256")); !os.IsNotExist(statErr) {
			t.Fatalf("checksum file should not exist after mismatch")
		}
	})
}

func TestFetchRemoteForceReplacesContent(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		// First version
		repoDir1 := makeRepoWithTemplate(t, "f.tpl", "V1")
		repoDir2 := makeRepoWithTemplate(t, "f.tpl", "V2")

		useRepo := repoDir1
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return useRepo, nil }
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		if err := fetchRemote(false, target, target.Template, paths); err != nil {
			t.Fatalf("first fetch: %v", err)
		}
		b1, _ := os.ReadFile(paths.remoteTemplateFile)
		if string(b1) != "V1" {
			t.Fatalf("expected V1, got %s", string(b1))
		}

		// Switch repo content and fetch again (simulate force path by just calling again)
		useRepo = repoDir2
		if err := fetchRemote(true, target, target.Template, paths); err != nil {
			t.Fatalf("second fetch: %v", err)
		}
		b2, _ := os.ReadFile(paths.remoteTemplateFile)
		if string(b2) != "V2" {
			t.Fatalf("expected V2 after force, got %s", string(b2))
		}
	})
}

func TestFetchRemoteCloneErrorPropagates(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			return "", fmt.Errorf("clone fail")
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		err := fetchRemote(false, target, target.Template, paths)
		if err == nil || !strings.Contains(err.Error(), "clone fail") {
			t.Fatalf("expected clone error, got %v", err)
		}
		if _, statErr := os.Stat(paths.remoteTemplateFile); !os.IsNotExist(statErr) {
			t.Fatalf("remote template should not exist after clone fail")
		}
	})
}

func TestFetchRemoteMissingTemplatePath(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		// Repo without the expected file
		repoDir := makeRepoWithTemplate(t, "other.tpl", "X")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "missing.tpl"}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		err := fetchRemote(false, target, target.Template, paths)
		if err == nil {
			t.Fatalf("expected error for missing template path")
		}
		if _, statErr := os.Stat(paths.remoteTemplateFile); !os.IsNotExist(statErr) {
			t.Fatalf("remote template should not exist when source missing")
		}
	})
}

func TestFetchRemoteCommitHashCaptureFailure(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		repoDir := makeRepoWithTemplate(t, "f.tpl", "hello")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		getCurrentCommitFunc = func(dir string) (string, error) { return "", fmt.Errorf("git error") }

		target := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		paths, _ := computeTemplatePaths("t", target, target.Template, map[string]any{})

		if err := fetchRemote(false, target, target.Template, paths); err != nil {
			t.Fatalf("unexpected failure despite commit hash error: %v", err)
		}
		if hasCommitHashMetadata(paths.remoteDir) {
			t.Fatalf("commit hash metadata should not exist after capture failure")
		}
	})
}

// TestFetchRemoteEndToEnd ensures fetchRemote stores raw template and commit hash metadata.
func TestFetchRemoteEndToEnd(t *testing.T) {
	withTempWD(t, func() {
		repoDir := "repo"
		os.MkdirAll(repoDir, 0o755)
		os.WriteFile(filepath.Join(repoDir, "f.tpl"), []byte("hello"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		defer func() { cloneFunc = origClone }()
		origGetCurrent := getCurrentCommitFunc
		getCurrentCommitFunc = func(dir string) (string, error) { return "cafebabecafebabecafebabecafebabecafebabe", nil }
		defer func() { getCurrentCommitFunc = origGetCurrent }()
		vars := map[string]any{"A": 1}
		testTarget := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		p, _ := computeTemplatePaths("t", testTarget, testTarget.Template, vars)
		if err := fetchRemote(false, testTarget, testTarget.Template, p); err != nil {
			t.Fatalf("fetchRemote: %v", err)
		}
		if _, err := os.Stat(p.remoteTemplateFile); err != nil {
			t.Fatalf("remote template file missing: %v", err)
		}
		if !hasCommitHashMetadata(p.remoteDir) {
			t.Fatalf("commit hash metadata not stored")
		}
	})
}
