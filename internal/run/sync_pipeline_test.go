//nolint:errcheck
package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestSyncPipelineHappyPath performs a concise end-to-end Exec flow (clone -> render -> link -> exec).
// It validates the rendered file content, symlink existence, and exec invocation argument ordering.
func TestSyncPipelineHappyPath(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		// Prepare fake repo template
		repoDir := filepath.Join("repo")
		os.MkdirAll(repoDir, 0o755)
		os.WriteFile(filepath.Join(repoDir, "t.tpl"), []byte("hello {{ .NAME }}"), 0o644)
		// Stub clone
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) { return repoDir, nil }
		defer func() { cloneFunc = origClone }()
		// Capture exec invocation
		var capturedArgs []string
		origExec := execCommand
		execCommand = func(name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			// benign command succeeds
			return exec.Command("echo")
		}
		defer func() { execCommand = origExec }()
		// Config
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{
			"build": {Binary: "echo", FileFlag: "-f", Args: []string{"--mode", "run"}, Template: &config.Template{Repo: "stub", Path: "t.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("world")}},
		}}
		// Run exec (passes through additional args)
		if err := Exec(cfg, "default", []string{"--extra"}, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("exec pipeline: %v", err)
		}
		// Validate symlink exists
		linkPath := filepath.Join(".duck", "build", "t")
		if fi, err := os.Lstat(linkPath); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink at %s: %v", linkPath, err)
		}
		target, _ := os.Readlink(linkPath)
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(linkPath), target)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read rendered: %v", err)
		}
		if !strings.Contains(string(data), "world") {
			t.Fatalf("rendered content missing variable expansion: %q", string(data))
		}
		// Validate captured args: expect echo -f <file> --mode run --extra
		if len(capturedArgs) < 5 {
			t.Fatalf("unexpected captured args %v", capturedArgs)
		}
		if capturedArgs[0] != "echo" || capturedArgs[1] != "-f" {
			t.Fatalf("unexpected start args %v", capturedArgs)
		}
		renderedFile := capturedArgs[2]
		if _, err := os.Stat(renderedFile); err != nil {
			t.Fatalf("referenced rendered file missing: %v", err)
		}
		tail := strings.Join(capturedArgs[3:], " ")
		if !strings.HasPrefix(tail, "--mode run") || !strings.HasSuffix(tail, "--extra") {
			t.Fatalf("unexpected arg tail %q", tail)
		}
	})
}
