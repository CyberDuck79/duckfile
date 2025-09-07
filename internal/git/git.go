package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/log"
)

// CloneInto clones/fetches repo@ref into cacheDir/repo and checks out the ref in the workdir.
// Returns the workdir path with the working tree set to the requested ref (detached HEAD).
func CloneInto(repo, ref, cacheDir string, submodules bool) (string, error) {
	workdir := filepath.Join(cacheDir, "repo") // 1-repo MVP, improve later

	// Already cloned?
	if _, err := exec.Command("test", "-d", filepath.Join(workdir, ".git")).CombinedOutput(); err == nil {
		log.Infof("Repository already exists at %s, updating...", workdir)
		// Fetch the desired ref and checkout FETCH_HEAD (detached)
		log.Debugf("Fetching ref %s from %s", ref, repo)
		if out, err := exec.Command("git", "-C", workdir, "fetch", "--depth", "1", "origin", ref).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch failed: %v: %s", err, string(out))
		}
		log.Debugf("Checking out ref %s", ref)
		if out, err := exec.Command("git", "-C", workdir, "checkout", "--force", "--detach", "FETCH_HEAD").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git checkout failed: %v: %s", err, string(out))
		}
		if submodules {
			log.Debugf("Updating submodules for %s", workdir)
			if out, err := exec.Command("git", "-C", workdir, "submodule", "update", "--init", "--recursive").CombinedOutput(); err != nil {
				return "", fmt.Errorf("git submodule update failed: %v: %s", err, string(out))
			}
		}
		log.Infof("Successfully updated repository to %s", ref)
	} else {
		log.Infof("Cloning repository %s to %s", repo, workdir)
		// Fresh clone, then force checkout the ref (supports branch, tag, or commit)
		cloneArgs := []string{"clone", "--depth", "1"}
		if submodules {
			cloneArgs = append(cloneArgs, "--recurse-submodules")
		}
		cloneArgs = append(cloneArgs, repo, workdir)
		if out, err := exec.Command("git", cloneArgs...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone failed: %v: %s", err, string(out))
		}
		// Ensure we have the ref and check it out detached
		log.Debugf("Fetching ref %s", ref)
		if out, err := exec.Command("git", "-C", workdir, "fetch", "--depth", "1", "origin", ref).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch failed: %v: %s", err, string(out))
		}
		log.Debugf("Checking out ref %s", ref)
		if out, err := exec.Command("git", "-C", workdir, "checkout", "--force", "--detach", "FETCH_HEAD").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git checkout failed: %v: %s", err, string(out))
		}
		if submodules {
			log.Debugf("Updating submodules for %s", workdir)
			if out, err := exec.Command("git", "-C", workdir, "submodule", "update", "--init", "--recursive").CombinedOutput(); err != nil {
				return "", fmt.Errorf("git submodule update failed: %v: %s", err, string(out))
			}
		}
		log.Infof("Successfully cloned and checked out %s", ref)
	}
	return workdir, nil
}

// GetCurrentCommitHash returns the commit hash of the currently checked out ref in the given directory.
// Returns the full 40-character SHA-1 hash.
func GetCurrentCommitHash(workdir string) (string, error) {
	out, err := exec.Command("git", "-C", workdir, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %v: %s", err, string(out))
	}
	hash := strings.TrimSpace(string(out))
	if len(hash) != 40 {
		return "", fmt.Errorf("invalid commit hash length: got %d characters, expected 40", len(hash))
	}
	return hash, nil
}

// GetRemoteCommitHash fetches the remote ref and returns its commit hash without checking it out.
// This function is used to check if the remote has changed since the last cache.
// If network fails, returns an error that can be handled gracefully by the caller.
func GetRemoteCommitHash(repo, ref string) (string, error) {
	log.Debugf("Checking remote commit hash for %s@%s", repo, ref)
	// Use ls-remote to get the commit hash without cloning/fetching
	out, err := exec.Command("git", "ls-remote", repo, ref).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git ls-remote failed (network or repository error): %v: %s", err, string(out))
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return "", fmt.Errorf("ref %q not found in repository %q", ref, repo)
	}

	// Parse output: "commit_hash\trefs/heads/branch" or "commit_hash\tHEAD"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			hash := strings.TrimSpace(parts[0])
			if len(hash) == 40 && isValidCommitHash(hash) {
				log.Debugf("Remote commit hash for %s@%s: %s", repo, ref, hash[:8])
				return hash, nil
			}
		}
	}

	return "", fmt.Errorf("could not parse commit hash from ls-remote output: %s", output)
}

// IsCommitHash checks if the given ref is already a commit hash (40-character hex string).
// This is used to validate configuration - if ref is already a commit hash,
// commit hash tracking doesn't make sense since commit hashes don't change.
func IsCommitHash(ref string) bool {
	return len(ref) == 40 && isValidCommitHash(ref)
}

// isValidCommitHash checks if a string is a valid 40-character hexadecimal hash
func isValidCommitHash(hash string) bool {
	if len(hash) != 40 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-fA-F0-9]{40}$", hash)
	return matched
}
