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

// TestExecMissingBinaryError ensures executing a target lacking a binary returns
// a helpful guidance error instead of proceeding.
func TestExecMissingBinaryError(t *testing.T) {
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Template: config.Template{Repo: "r", Path: "file.tpl"}}}}
	if err := Exec(cfg, "default", nil, defaultSecurityConfig(), nil, nil); err == nil || !strings.Contains(err.Error(), "no binary configured") {
		t.Fatalf("expected missing binary error, got %v", err)
	}
}

// TestExecUnderlyingBinaryFailure stubs the underlying process to exit non-zero
// and asserts Exec surfaces a failure.
func TestExecUnderlyingBinaryFailure(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
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
		origExec := execCommand
		execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("sh", "-c", "exit 5") }
		defer func() { execCommand = origExec }()
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "dummy", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}}}}
		if err := Exec(cfg, "default", nil, defaultSecurityConfig(), nil, nil); err == nil {
			t.Fatalf("expected failure from underlying binary")
		}
	})
}

// TestExecuteTargetArgumentOrdering validates that executeTarget builds command args
// in the expected order: [fileFlag renderedFile] followed by configured Args and user passthrough args.
func TestExecuteTargetArgumentOrdering(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		// minimal rendered object setup
		tplDir := filepath.Join("repo")
		os.MkdirAll(tplDir, 0o755)
		os.WriteFile(filepath.Join(tplDir, "t.tpl"), []byte("hi"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string) (string, error) { return tplDir, nil }
		defer func() { cloneFunc = origClone }()
		// capture exec invocation
		var capturedName string
		var capturedArgs []string
		origExec := execCommand
		execCommand = func(name string, args ...string) *exec.Cmd {
			capturedName = name
			capturedArgs = append([]string{}, args...)
			// return benign command
			return exec.Command("echo")
		}
		defer func() { execCommand = origExec }()
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "mybin", FileFlag: "-f", Args: []string{"--opt", "123"}, Template: config.Template{Repo: "stub", Path: "t.tpl"}}}}
		if err := Exec(cfg, "default", []string{"--extra", "val"}, defaultSecurityConfig(), nil, nil); err != nil {
			t.Fatalf("exec: %v", err)
		}
		if capturedName != "mybin" {
			t.Fatalf("expected binary mybin, got %s", capturedName)
		}
		if len(capturedArgs) < 5 {
			t.Fatalf("unexpected args length: %v", capturedArgs)
		}
		// Expect ordering: -f <file> --opt 123 --extra val
		if capturedArgs[0] != "-f" {
			t.Fatalf("expected -f first got %v", capturedArgs)
		}
		renderedFile := capturedArgs[1]
		if _, err := os.Stat(renderedFile); err != nil {
			t.Fatalf("rendered file missing: %v", err)
		}
		expectedTail := []string{"--opt", "123", "--extra", "val"}
		if strings.Join(capturedArgs[2:], ":") != strings.Join(expectedTail, ":") {
			t.Fatalf("unexpected arg tail %v", capturedArgs)
		}
	})
}
