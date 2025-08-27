//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to isolate working directory and restore global seams
func withTempWD(t *testing.T, fn func()) {
	t.Helper()
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	fn()
}

// build a minimal target + template file fixture
func makeRepoWithTemplate(t *testing.T, fileRelPath, contents string) (repoDir string) {
	t.Helper()
	repoDir = t.TempDir()
	full := filepath.Join(repoDir, fileRelPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	return repoDir
}

// reset seam globals for each test
func resetSeams(t *testing.T) (restore func()) {
	t.Helper()
	origClone := cloneFunc
	origGetCurrent := getCurrentCommitFunc
	origGetRemote := getRemoteCommitFunc
	origExec := execCommand
	return func() {
		cloneFunc = origClone
		getCurrentCommitFunc = origGetCurrent
		getRemoteCommitFunc = origGetRemote
		execCommand = origExec
	}
}
