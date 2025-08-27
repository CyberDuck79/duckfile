//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestCleanRemovesOnlyTargetArtifacts verifies cleaning a single target removes
// its symlink and object while preserving others.
func TestCleanRemovesOnlyTargetArtifacts(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
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
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{
			"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"V": config.NewLiteralVar("ONE")}},
			"other": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"V": config.NewLiteralVar("TWO")}},
		}}
		if err := Sync(cfg, "", false, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("sync all: %v", err)
		}
		keysBefore := listObjectKeys(t)
		if len(keysBefore) != 2 {
			t.Fatalf("expected 2 keys, got %v", keysBefore)
		}
		if err := Clean(cfg, "default"); err != nil {
			t.Fatalf("clean default: %v", err)
		}
		if _, err := os.Lstat(filepath.Join(".duck", "build", "file")); err == nil {
			t.Fatalf("expected default symlink removed")
		}
		if _, err := os.Lstat(filepath.Join(".duck", "other", "file")); err != nil {
			t.Fatalf("other symlink missing after clean: %v", err)
		}
		if keys := listObjectKeys(t); len(keys) != 1 {
			t.Fatalf("expected 1 remaining object, got %v", keys)
		}
	})
}
