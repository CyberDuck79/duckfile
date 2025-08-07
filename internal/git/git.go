package git

import (
	"os/exec"
	"path/filepath"
)

// CloneInto clones/fetches repo@ref into cacheDir/repoHash and returns the workdir path.
func CloneInto(repo, ref, cacheDir string) (string, error) {
	workdir := filepath.Join(cacheDir, "repo") // 1-repo MVP, improve later
	// already cloned?
	if _, err := exec.Command("test", "-d", filepath.Join(workdir, ".git")).CombinedOutput(); err == nil {
		cmd := exec.Command("git", "-C", workdir, "fetch", "--depth", "1", "origin", ref)
		if err := cmd.Run(); err != nil {
			return "", err
		}
	} else {
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repo, workdir)
		if err := cmd.Run(); err != nil {
			return "", err
		}
	}
	return workdir, nil
}
