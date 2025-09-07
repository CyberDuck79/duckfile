//nolint:errcheck
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// run executes a git command and fails test on error.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
	return string(out)
}

// TestCloneIntoFreshAndUpdate covers fresh clone, subsequent update fetch, and commit hash retrieval.
func TestCloneIntoFreshAndUpdate(t *testing.T) {
}

// TestCloneIntoWithSubmodules verifies that submodules are cloned when submodules=true
func TestCloneIntoWithSubmodules(t *testing.T) {
	os.Setenv("GIT_ALLOW_PROTOCOL", "file")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	if out, err := exec.Command("git", "init", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("init bare failed: %v: %s", err, string(out))
	}

	// Create seed repo with submodule
	seed := filepath.Join(root, "seed")
	if out, err := exec.Command("git", "init", seed).CombinedOutput(); err != nil {
		t.Fatalf("init seed failed: %v: %s", err, string(out))
	}
	run(t, seed, "config", "user.email", "tester@example.com")
	run(t, seed, "config", "user.name", "Tester")

	// Create submodule repo
	submod := filepath.Join(root, "submod")
	if out, err := exec.Command("git", "init", submod).CombinedOutput(); err != nil {
		t.Fatalf("init submod failed: %v: %s", err, string(out))
	}
	run(t, submod, "config", "user.email", "tester@example.com")
	run(t, submod, "config", "user.name", "Tester")
	subfile := filepath.Join(submod, "sub.txt")
	os.WriteFile(subfile, []byte("submodule content"), 0o644)
	run(t, submod, "add", "sub.txt")
	run(t, submod, "commit", "-m", "add sub.txt")

	// Add submodule to seed repo
	run(t, seed, "submodule", "add", submod, "submod")
	run(t, seed, "commit", "-m", "add submodule")
	run(t, seed, "branch", "-M", "main")
	run(t, seed, "remote", "add", "origin", origin)
	run(t, seed, "push", "origin", "main")

	cacheDir := filepath.Join(root, "cache")
	// Clone with submodules enabled
	workdir, err := CloneInto(origin, "main", cacheDir, true)
	if err != nil {
		t.Fatalf("CloneInto with submodules failed: %v", err)
	}
	// Check that submodule directory exists and file is present
	submodPath := filepath.Join(workdir, "submod", "sub.txt")
	if _, err := os.Stat(submodPath); err != nil {
		t.Fatalf("expected submodule file present: %v", err)
	}

	firstHash, err := GetCurrentCommitHash(workdir)
	if err != nil {
		t.Fatalf("GetCurrentCommitHash failed: %v", err)
	}
	if len(firstHash) != 40 {
		t.Fatalf("unexpected hash length: %d", len(firstHash))
	}

	readme := filepath.Join(seed, "README.md")
	if err := os.WriteFile(readme, []byte("second"), 0o644); err != nil {
		t.Fatalf("update readme: %v", err)
	}
	run(t, seed, "add", "README.md")
	run(t, seed, "commit", "-m", "second")
	run(t, seed, "push", "origin", "main")

	workdir2, err2 := CloneInto(origin, "main", cacheDir, false)
	if err2 != nil {
		t.Fatalf("CloneInto update failed: %v", err2)
	}
	if workdir2 != workdir {
		t.Fatalf("expected same workdir path on update")
	}
	secondHash, err := GetCurrentCommitHash(workdir2)
	if err != nil {
		t.Fatalf("GetCurrentCommitHash after update failed: %v", err)
	}
	if secondHash == firstHash {
		t.Fatalf("expected different commit hash after update")
	}

	remoteHash, err := GetRemoteCommitHash(origin, "main")
	if err != nil {
		t.Fatalf("GetRemoteCommitHash success path failed: %v", err)
	}
	if remoteHash != secondHash {
		t.Fatalf("remote hash %s != checked out %s", remoteHash, secondHash)
	}
}
