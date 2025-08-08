package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// CloneInto clones/fetches repo@ref into cacheDir/repo and checks out the ref in the workdir.
// Returns the workdir path with the working tree set to the requested ref (detached HEAD).
func CloneInto(repo, ref, cacheDir string) (string, error) {
	workdir := filepath.Join(cacheDir, "repo") // 1-repo MVP, improve later

	// Already cloned?
	if _, err := exec.Command("test", "-d", filepath.Join(workdir, ".git")).CombinedOutput(); err == nil {
		// Fetch the desired ref and checkout FETCH_HEAD (detached)
		if out, err := exec.Command("git", "-C", workdir, "fetch", "--depth", "1", "origin", ref).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch failed: %v: %s", err, string(out))
		}
		if out, err := exec.Command("git", "-C", workdir, "checkout", "--force", "--detach", "FETCH_HEAD").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git checkout failed: %v: %s", err, string(out))
		}
	} else {
		// Fresh clone, then force checkout the ref (supports branch, tag, or commit)
		if out, err := exec.Command("git", "clone", "--depth", "1", repo, workdir).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone failed: %v: %s", err, string(out))
		}
		// Ensure we have the ref and check it out detached
		if out, err := exec.Command("git", "-C", workdir, "fetch", "--depth", "1", "origin", ref).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch failed: %v: %s", err, string(out))
		}
		if out, err := exec.Command("git", "-C", workdir, "checkout", "--force", "--detach", "FETCH_HEAD").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git checkout failed: %v: %s", err, string(out))
		}
	}
	return workdir, nil
}
