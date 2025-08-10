package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

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

	cfg := &config.DuckConf{Version: 1, Default: config.Target{Name: "build", Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("world")}}}

	// override execCommand to no-op for binary execution
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Return a command that succeeds without side effects
		return exec.Command("true")
	}
	defer func() { execCommand = origExec }()

	// Run sync (should render)
	if err := Sync(cfg, "", false); err != nil {
		t.Fatalf("sync error: %v", err)
	}
	// verify rendered artifact exists via symlink target
	base := "file"
	linkPath := filepath.Join(".duck", "default", base)
	fi, err := os.Lstat(linkPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", linkPath)
	}

	// Run Exec (should reuse cache)
	if err := Exec(cfg, "default", nil); err != nil {
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
