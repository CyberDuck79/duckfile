//nolint:errcheck
package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CyberDuck79/duckfile/internal/config"
)

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
	if err := Sync(cfg, "", false); err != nil {
		t.Fatalf("sync1: %v", err)
	}
	keys1 := listObjectKeys(t)
	if len(keys1) != 1 {
		t.Fatalf("expected 1 key got %v", keys1)
	}
	// change variable => new key
	cfg.Targets[cfg.Default].Variables["NAME"] = config.NewLiteralVar("two")
	if err := Sync(cfg, "", false); err != nil {
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
	if err := Sync(cfg, "", false); err != nil {
		t.Fatalf("sync1: %v", err)
	}
	// modify source template, but don't force
	os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("v2 {{ .NAME }}"), 0o644)
	if err := Sync(cfg, "", false); err != nil {
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
	if err := Sync(cfg, "", false); err != nil {
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
	if err := Sync(cfg, "", true); err != nil {
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
	if err := Exec(cfg, "default", nil); err == nil || !strings.Contains(err.Error(), "no binary configured") {
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
	if err := Exec(cfg, "default", nil); err == nil {
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
	err := Sync(cfg, "default", false)
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
	if err := Sync(cfg, "build", false); err != nil {
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
	if err := Sync(cfg, "build", false); err != nil {
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
	if err := Sync(cfg, "", false); err != nil {
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

// end
