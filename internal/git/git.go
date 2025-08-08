package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// CloneInto clones/fetches repo@ref into cacheDir/repoHash and returns the workdir path.
func CloneInto(repo, ref, cacheDir string) (string, error) {
	workdir := filepath.Join(cacheDir, "repo") // 1-repo MVP, improve later
	// already cloned?
	if _, err := exec.Command("test", "-d", filepath.Join(workdir, ".git")).CombinedOutput(); err == nil {
		cmd := exec.Command("git", "-C", workdir, "fetch", "--depth", "1", "origin", ref)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch failed: %v: %s", err, string(out))
		}
	} else {
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repo, workdir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone failed: %v: %s", err, string(out))
		}
	}
	return workdir, nil
}
