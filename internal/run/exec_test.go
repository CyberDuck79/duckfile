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

// TestExecuteTargetEnvironmentVariables validates that Duck environment variables
// are properly set during target execution
func TestExecuteTargetEnvironmentVariables(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		// Setup test repository and template
		repoDir := t.TempDir()
		templateFile := filepath.Join(repoDir, "test.tpl")
		os.WriteFile(templateFile, []byte("test content"), 0o644)

		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string) (string, error) {
			return repoDir, nil
		}
		defer func() { cloneFunc = origClone }()

		// Capture the command object that gets created
		var capturedCmd *exec.Cmd
		origExec := execCommand
		execCommand = func(name string, args ...string) *exec.Cmd {
			cmd := exec.Command("echo") // Benign command that will succeed
			capturedCmd = cmd
			return cmd
		}
		defer func() { execCommand = origExec }()

		// Create test target and execute
		target := config.Target{
			Binary:   "test-binary",
			FileFlag: "-f",
			Template: config.Template{
				Repo: "https://github.com/test/repo.git",
				Ref:  "main",
				Path: "test.tpl",
			},
		}

		cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"test": target}}
		err := Exec(cfg, "test", []string{}, defaultSecurityConfig(), nil, nil)
		if err != nil {
			t.Fatalf("unexpected execution error: %v", err)
		}

		// Verify Duck environment variables are set on the captured command
		if capturedCmd == nil {
			t.Fatalf("No command was captured")
		}

		envMap := make(map[string]string)
		for _, env := range capturedCmd.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}

		expectedEnvs := []string{
			"DUCK_REPO_PATH",
			"DUCK_REPO_URL",
			"DUCK_REPO_REF",
			"DUCK_TEMPLATE_PATH",
			"DUCK_RENDERED_PATH",
			"DUCK_SYMLINK_PATH",
			"DUCK_TARGET_NAME",
			"DUCK_CACHE_DIR",
		}

		for _, envVar := range expectedEnvs {
			if _, exists := envMap[envVar]; !exists {
				t.Errorf("Expected environment variable %s not found", envVar)
			}
		}

		// Verify specific values
		if envMap["DUCK_REPO_URL"] != "https://github.com/test/repo.git" {
			t.Errorf("Expected DUCK_REPO_URL=https://github.com/test/repo.git, got %s", envMap["DUCK_REPO_URL"])
		}
		if envMap["DUCK_REPO_REF"] != "main" {
			t.Errorf("Expected DUCK_REPO_REF=main, got %s", envMap["DUCK_REPO_REF"])
		}
		if envMap["DUCK_TARGET_NAME"] != "test" {
			t.Errorf("Expected DUCK_TARGET_NAME=test, got %s", envMap["DUCK_TARGET_NAME"])
		}
	})
}

// TestBuildDuckEnvironment tests the environment variable builder function
func TestBuildDuckEnvironment(t *testing.T) {
	result := &PrepareTemplateResult{
		ObjFile:      "/path/to/rendered",
		LinkPath:     "/path/to/symlink",
		RepoPath:     "/path/to/repo",
		RepoURL:      "https://github.com/test/repo.git",
		RepoRef:      "main",
		TemplatePath: "/path/to/template.tpl",
		TargetName:   "build",
		CacheDir:     "/path/to/cache",
	}

	env := buildDuckEnvironment(result)

	// Check that all expected environment variables are present
	expectedVars := map[string]string{
		"DUCK_REPO_PATH":     "/path/to/repo",
		"DUCK_REPO_URL":      "https://github.com/test/repo.git",
		"DUCK_REPO_REF":      "main",
		"DUCK_TEMPLATE_PATH": "/path/to/template.tpl",
		"DUCK_RENDERED_PATH": "/path/to/rendered",
		"DUCK_SYMLINK_PATH":  "/path/to/symlink",
		"DUCK_TARGET_NAME":   "build",
		"DUCK_CACHE_DIR":     "/path/to/cache",
	}

	found := make(map[string]bool)
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			key, value := parts[0], parts[1]
			if expectedValue, expected := expectedVars[key]; expected {
				found[key] = true
				if value != expectedValue {
					t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, value)
				}
			}
		}
	}

	// Ensure all expected variables were found
	for key := range expectedVars {
		if !found[key] {
			t.Errorf("Expected environment variable %s not found", key)
		}
	}

	// Verify that original environment is preserved
	originalEnvLen := len(os.Environ())
	if len(env) < originalEnvLen+len(expectedVars) {
		t.Errorf("Expected at least %d environment variables, got %d", originalEnvLen+len(expectedVars), len(env))
	}
}

// TestEnvironmentVariablesWithDefaultRef tests environment variables when ref is empty (defaults to main)
func TestEnvironmentVariablesWithDefaultRef(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		repoDir := t.TempDir()
		templateFile := filepath.Join(repoDir, "test.tpl")
		os.WriteFile(templateFile, []byte("test content"), 0o644)

		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string) (string, error) {
			return repoDir, nil
		}
		defer func() { cloneFunc = origClone }()

		var capturedCmd *exec.Cmd
		origExec := execCommand
		execCommand = func(name string, args ...string) *exec.Cmd {
			cmd := exec.Command("echo")
			capturedCmd = cmd
			return cmd
		}
		defer func() { execCommand = origExec }()

		// Create target with empty ref (should default to "main")
		target := config.Target{
			Binary:   "test-binary",
			FileFlag: "-f",
			Template: config.Template{
				Repo: "https://github.com/test/repo.git",
				Ref:  "", // Empty ref should default to "main"
				Path: "test.tpl",
			},
		}

		cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"test": target}}
		err := Exec(cfg, "test", []string{}, defaultSecurityConfig(), nil, nil)
		if err != nil {
			t.Fatalf("unexpected execution error: %v", err)
		}

		// Verify DUCK_REPO_REF is empty (as per config)
		if capturedCmd == nil {
			t.Fatalf("No command was captured")
		}

		envMap := make(map[string]string)
		for _, env := range capturedCmd.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}

		if envMap["DUCK_REPO_REF"] != "" {
			t.Errorf("Expected DUCK_REPO_REF to be empty, got %s", envMap["DUCK_REPO_REF"])
		}
	})
}
