//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestLinkAndPruneRendered focuses on extracting old key and pruning logic.
func TestLinkAndPruneRendered(t *testing.T) {
	withTempWD(t, func() {
		vars := map[string]any{"A": 1}
		p1, _ := computeTemplatePaths("t", config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}, vars)
		os.MkdirAll(p1.renderedDir, 0o755)
		os.WriteFile(p1.renderedFile, []byte("one"), 0o644)
		oldKey, err := linkRendered(p1)
		if err != nil {
			t.Fatalf("link first: %v", err)
		}
		if oldKey != "" {
			t.Fatalf("expected no old key first link, got %s", oldKey)
		}
		vars2 := map[string]any{"A": 2}
		p2, _ := computeTemplatePaths("t", config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}, vars2)
		os.MkdirAll(p2.renderedDir, 0o755)
		os.WriteFile(p2.renderedFile, []byte("two"), 0o644)
		oldKey2, err := linkRendered(p2)
		if err != nil {
			t.Fatalf("link second: %v", err)
		}
		if oldKey2 != p1.renderedKey {
			t.Fatalf("expected old key %s got %s", p1.renderedKey, oldKey2)
		}
		if _, err := os.Stat(filepath.Join(".duck", "objects", "rendered", p1.renderedKey)); err != nil {
			t.Fatalf("pre prune missing old rendered: %v", err)
		}
		pruneOldRendered(oldKey2, p2.renderedKey)
		if _, err := os.Stat(filepath.Join(".duck", "objects", "rendered", p1.renderedKey)); !os.IsNotExist(err) {
			t.Fatalf("old rendered not pruned")
		}
	})
}

// Early return when symlink already points correctly.
func TestEnsureSymlinkAlreadyCorrect(t *testing.T) {
	withTempWD(t, func() {
		file := "f.txt"
		os.WriteFile(file, []byte("x"), 0o644)
		link := "link.txt"
		if err := ensureSymlink(file, link); err != nil {
			t.Fatalf("create link: %v", err)
		}
		if err := ensureSymlink(file, link); err != nil {
			t.Fatalf("ensure again: %v", err)
		}
	})
}

// Replace existing regular file with correct symlink.
func TestEnsureSymlinkReplacesFile(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("content"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), data, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
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
	})
}

// Repair dangling symlink.
func TestBrokenSymlinkUpdated(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hello"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		os.MkdirAll(filepath.Join(".duck", "build"), 0o755)
		link := filepath.Join(".duck", "build", "file")
		os.Symlink("../objects/missing-key/file", link)
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}}}}
		if err := Sync(cfg, "build", false, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("sync err: %v", err)
		}
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
	})
}
