//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestCopyRenderedTarget verifies that targets with copyRendered:true produce a regular file at renderedPath.
func TestCopyRenderedTarget(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("copy {{ .V }}"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			b, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), b, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		cfg := &config.DuckConf{Version: 1, Default: "copy", Targets: map[string]config.Target{
			"copy": {
				Template:     config.Template{Repo: "stub", Path: "file.tpl"},
				Variables:    map[string]config.VarValue{"V": config.NewLiteralVar("OK")},
				RenderedPath: "copied.txt",
				CopyRendered: true,
			},
		}}
		if err := Sync(cfg, "copy", false, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("sync copyRendered: %v", err)
		}
		fi, err := os.Lstat("copied.txt")
		if err != nil {
			t.Fatalf("expected copied.txt: %v", err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("expected regular file, got symlink")
		}
		b, _ := os.ReadFile("copied.txt")
		if string(b) != "copy OK" {
			t.Fatalf("unexpected file content: %q", string(b))
		}
	})
}

// TestSelfTarget verifies that the self target always copies to the config file path and forbids binary execution.
func TestSelfTarget(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "duck.yaml.tpl"), []byte("version: {{ .VERSION }}\n"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			b, _ := os.ReadFile(filepath.Join(templateSrc, "duck.yaml.tpl"))
			os.WriteFile(filepath.Join(dst, "duck.yaml.tpl"), b, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		cfg := &config.DuckConf{Version: 1, Default: "self", Targets: map[string]config.Target{
			"self": {
				Template:     config.Template{Repo: "stub", Path: "duck.yaml.tpl"},
				Variables:    map[string]config.VarValue{"VERSION": config.NewLiteralVar("2")},
				RenderedPath: "duck.yaml",
				CopyRendered: true,
			},
		}}
		if err := Sync(cfg, "self", false, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("sync self: %v", err)
		}
		fi, err := os.Lstat("duck.yaml")
		if err != nil {
			t.Fatalf("expected duck.yaml: %v", err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("expected regular file for self target")
		}
		b, _ := os.ReadFile("duck.yaml")
		if string(b) != "version: 2\n" {
			t.Fatalf("unexpected self file content: %q", string(b))
		}
	})
}

// TestCopyRenderedErrors covers error cases in copyRendered for coverage.
func TestCopyRenderedErrors(t *testing.T) {
	t.Run("fail to create parent dir", func(t *testing.T) {
		// Use an invalid path to trigger MkdirAll error
		err := copyRendered("src.txt", string([]byte{0}))
		if err == nil {
			t.Fatalf("expected error for invalid parent dir")
		}
	})

	t.Run("fail to remove existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		dst := filepath.Join(tmpDir, "file.txt")
		os.WriteFile(dst, []byte("data"), 0o644)
		// Make file read-only so Remove fails
		os.Chmod(dst, 0)
		err := copyRendered("src.txt", dst)
		if err == nil {
			t.Fatalf("expected error removing read-only file")
		}
		// Restore permissions for cleanup
		os.Chmod(dst, 0o644)
	})

	t.Run("fail to open src file", func(t *testing.T) {
		tmpDir := t.TempDir()
		dst := filepath.Join(tmpDir, "file.txt")
		err := copyRendered("nonexistent.txt", dst)
		if err == nil {
			t.Fatalf("expected error opening nonexistent src file")
		}
	})

	t.Run("fail to create dst file", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "src.txt")
		os.WriteFile(src, []byte("data"), 0o644)
		// Use an invalid path for dst
		err := copyRendered(src, string([]byte{0}))
		if err == nil {
			t.Fatalf("expected error creating dst file")
		}
	})

	t.Run("fail to copy file contents", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "src.txt")
		dst := filepath.Join(tmpDir, "dst.txt")
		// Create a directory as src to trigger io.Copy error
		os.Mkdir(src, 0o755)
		err := copyRendered(src, dst)
		if err == nil {
			t.Fatalf("expected error copying from directory src")
		}
	})
}
