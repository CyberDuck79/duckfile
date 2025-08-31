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
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	if out, err := exec.Command("git", "init", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("init bare failed: %v: %s", err, string(out))
	}

	seed := filepath.Join(root, "seed")
	if out, err := exec.Command("git", "init", seed).CombinedOutput(); err != nil {
		t.Fatalf("init seed failed: %v: %s", err, string(out))
	}
	// Configure identity
	run(t, seed, "config", "user.email", "tester@example.com")
	run(t, seed, "config", "user.name", "Tester")

	// Initial commit
	readme := filepath.Join(seed, "README.md")
	if err := os.WriteFile(readme, []byte("first"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	run(t, seed, "add", "README.md")
	run(t, seed, "commit", "-m", "initial")
	run(t, seed, "branch", "-M", "main")
	run(t, seed, "remote", "add", "origin", origin)
	run(t, seed, "push", "origin", "main")

	cacheDir := filepath.Join(root, "cache")
	workdir, err := CloneInto(origin, "main", cacheDir)
	if err != nil {
		t.Fatalf("CloneInto fresh failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		t.Fatalf("expected .git in workdir: %v", err)
	}

	firstHash, err := GetCurrentCommitHash(workdir)
	if err != nil {
		t.Fatalf("GetCurrentCommitHash failed: %v", err)
	}
	if len(firstHash) != 40 {
		t.Fatalf("unexpected hash length: %d", len(firstHash))
	}

	// Second commit pushed to origin, then re-run CloneInto to trigger update path
	if err := os.WriteFile(readme, []byte("second"), 0o644); err != nil {
		t.Fatalf("update readme: %v", err)
	}
	run(t, seed, "add", "README.md")
	run(t, seed, "commit", "-m", "second")
	run(t, seed, "push", "origin", "main")

	// Update call: repository already exists
	workdir2, err := CloneInto(origin, "main", cacheDir)
	if err != nil {
		t.Fatalf("CloneInto update failed: %v", err)
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

	// Remote commit hash should match current (ls-remote success path)
	remoteHash, err := GetRemoteCommitHash(origin, "main")
	if err != nil {
		t.Fatalf("GetRemoteCommitHash success path failed: %v", err)
	}
	if remoteHash != secondHash {
		t.Fatalf("remote hash %s != checked out %s", remoteHash, secondHash)
	}
}
