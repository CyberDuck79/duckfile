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

		templateDir := makeRepoWithTemplate(t, "tpls/a.tpl", "hello")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(filepath.Dir(filepath.Join(repoDir, "tpls/a.tpl")), 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "tpls/a.tpl"))
			os.WriteFile(filepath.Join(repoDir, "tpls/a.tpl"), data, 0o644)
			return repoDir, nil
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "abcdef1234567890", nil }

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "tpls/a.tpl"}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// First fetch the remote repository
		if err := fetchRemote(false, resolved, paths); err != nil {
			t.Fatalf("fetchRemote: %v", err)
		}

		// Then extract the template
		if err := extractTemplate(resolved, paths); err != nil {
			t.Fatalf("extractTemplate: %v", err)
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

		templateDir := makeRepoWithTemplate(t, "f.tpl", "BODY")
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte("BODY")))
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "f.tpl"))
			os.WriteFile(filepath.Join(repoDir, "f.tpl"), data, 0o644)
			return repoDir, nil
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "x", Path: "f.tpl", Checksum: sum}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// First fetch the remote repository
		if err := fetchRemote(false, resolved, paths); err != nil {
			t.Fatalf("fetchRemote: %v", err)
		}

		// Then extract the template (this is where checksum validation happens)
		if err := extractTemplate(resolved, paths); err != nil {
			t.Fatalf("extractTemplate: %v", err)
		}

		// Check that template metadata includes checksum
		if _, err := os.Stat(filepath.Join(paths.templateDir, "metadata.json")); err != nil {
			t.Fatalf("template metadata missing: %v", err)
		}
	})
}

func TestFetchRemoteChecksumMismatch(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		templateDir := makeRepoWithTemplate(t, "f.tpl", "BODY")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "f.tpl"))
			os.WriteFile(filepath.Join(repoDir, "f.tpl"), data, 0o644)
			return repoDir, nil
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "x", Path: "f.tpl", Checksum: "deadbeef"}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// First fetch the remote repository (should succeed)
		if err := fetchRemote(false, resolved, paths); err != nil {
			t.Fatalf("fetchRemote should succeed: %v", err)
		}

		// Then extract the template (should fail on checksum)
		err = extractTemplate(resolved, paths)
		if err == nil {
			t.Fatalf("expected checksum mismatch error")
		}
		if _, statErr := os.Stat(paths.remoteTemplateFile); !os.IsNotExist(statErr) {
			t.Fatalf("remote template should not exist after checksum failure")
		}
		if _, statErr := os.Stat(filepath.Join(paths.templateDir, "metadata.json")); !os.IsNotExist(statErr) {
			t.Fatalf("template metadata should not exist after checksum failure")
		}
	})
}

func TestFetchRemoteForceReplacesContent(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		// First version
		templateDir1 := makeRepoWithTemplate(t, "f.tpl", "V1")
		templateDir2 := makeRepoWithTemplate(t, "f.tpl", "V2")

		useTemplateDir := templateDir1
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(useTemplateDir, "f.tpl"))
			os.WriteFile(filepath.Join(repoDir, "f.tpl"), data, 0o644)
			return repoDir, nil
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// First fetch and extract
		if err := fetchRemote(false, resolved, paths); err != nil {
			t.Fatalf("first fetch: %v", err)
		}
		if err := extractTemplate(resolved, paths); err != nil {
			t.Fatalf("first extract: %v", err)
		}
		b1, _ := os.ReadFile(paths.remoteTemplateFile)
		if string(b1) != "V1" {
			t.Fatalf("expected V1, got %s", string(b1))
		}

		// Switch repo content and fetch again with force
		useTemplateDir = templateDir2
		if err := fetchRemote(true, resolved, paths); err != nil {
			t.Fatalf("second fetch: %v", err)
		}
		if err := extractTemplate(resolved, paths); err != nil {
			t.Fatalf("second extract: %v", err)
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

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// fetchRemote should fail with clone error
		err = fetchRemote(false, resolved, paths)
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
		templateDir := makeRepoWithTemplate(t, "other.tpl", "X")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "other.tpl"))
			os.WriteFile(filepath.Join(repoDir, "other.tpl"), data, 0o644)
			return repoDir, nil
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "hash", nil }

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "missing.tpl"}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// fetchRemote should succeed (repository exists)
		if err := fetchRemote(false, resolved, paths); err != nil {
			t.Fatalf("fetchRemote should succeed: %v", err)
		}

		// extractTemplate should fail (missing template path)
		err = extractTemplate(resolved, paths)
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

		templateDir := makeRepoWithTemplate(t, "f.tpl", "hello")
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "f.tpl"))
			os.WriteFile(filepath.Join(repoDir, "f.tpl"), data, 0o644)
			return repoDir, nil
		}
		getCurrentCommitFunc = func(dir string) (string, error) { return "", fmt.Errorf("git error") }

		target := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		paths, _ := computeTemplatePaths("t", target, map[string]any{})

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// fetchRemote should succeed despite commit hash capture failure
		if err := fetchRemote(false, resolved, paths); err != nil {
			t.Fatalf("unexpected failure despite commit hash error: %v", err)
		}
		if hasCommitHashMetadata(paths.remoteDir) {
			t.Fatalf("commit hash metadata should not exist after capture failure")
		}
	})
}

// TestFetchRemoteEndToEnd ensures fetchRemote stores remote metadata and extractTemplate stores template.
func TestFetchRemoteEndToEnd(t *testing.T) {
	withTempWD(t, func() {
		templateDir := "template_source"
		os.MkdirAll(templateDir, 0o755)
		os.WriteFile(filepath.Join(templateDir, "f.tpl"), []byte("hello"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "f.tpl"))
			os.WriteFile(filepath.Join(repoDir, "f.tpl"), data, 0o644)
			return repoDir, nil
		}
		defer func() { cloneFunc = origClone }()
		origGetCurrent := getCurrentCommitFunc
		getCurrentCommitFunc = func(dir string) (string, error) { return "cafebabecafebabecafebabecafebabecafebabe", nil }
		defer func() { getCurrentCommitFunc = origGetCurrent }()

		vars := map[string]any{"A": 1}
		target := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		p, _ := computeTemplatePaths("t", target, vars)

		// Resolve template config to ResolvedTemplate
		resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
		if err != nil {
			t.Fatalf("ResolveTemplateConfig: %v", err)
		}

		// First fetch the remote
		if err := fetchRemote(false, resolved, p); err != nil {
			t.Fatalf("fetchRemote: %v", err)
		}
		if !hasCommitHashMetadata(p.remoteDir) {
			t.Fatalf("commit hash metadata not stored")
		}

		// Then extract the template
		if err := extractTemplate(resolved, p); err != nil {
			t.Fatalf("extractTemplate: %v", err)
		}
		if _, err := os.Stat(p.remoteTemplateFile); err != nil {
			t.Fatalf("remote template file missing: %v", err)
		}
	})
}
