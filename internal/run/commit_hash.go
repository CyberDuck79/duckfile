package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/log"
)

// writeCommitHashMetadata stores the commit hash in the cache directory
func writeCommitHashMetadata(objDir, commitHash string) error {
	if commitHash == "" {
		return nil // Nothing to write
	}

	metadataFile := filepath.Join(objDir, "commit.hash")
	return os.WriteFile(metadataFile, []byte(commitHash), 0o644)
}

// readCommitHashMetadata reads the commit hash from the cache directory
func readCommitHashMetadata(objDir string) (string, error) {
	metadataFile := filepath.Join(objDir, "commit.hash")

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No metadata file exists
		}
		return "", fmt.Errorf("failed to read commit hash metadata: %w", err)
	}

	commitHash := strings.TrimSpace(string(data))
	return commitHash, nil
}

// hasCommitHashMetadata checks if commit hash metadata exists in the cache directory
func hasCommitHashMetadata(objDir string) bool {
	metadataFile := filepath.Join(objDir, "commit.hash")
	_, err := os.Stat(metadataFile)
	return err == nil
}

// validateCachedCommitHash checks if the cached commit hash is still valid by comparing with remote.
// Returns true if cache is valid, false if it should be invalidated, and an error for network issues.
func validateCachedCommitHash(repo, ref, objDir string) (bool, error) {
	// Read stored commit hash
	storedHash, err := readCommitHashMetadata(objDir)
	if err != nil {
		return false, fmt.Errorf("failed to read cached commit hash: %w", err)
	}

	if storedHash == "" {
		// No stored hash means cache was created without commit tracking
		log.Debugf("no stored commit hash found for %s@%s, cache validation skipped", repo, ref)
		return true, nil
	}

	log.Debugf("validating cached commit hash %s for %s@%s", truncateHash(storedHash), repo, ref)

	// Get current remote commit hash
	remoteHash, err := getRemoteCommitFunc(repo, ref)
	if err != nil {
		// Network failure or repository error - warn but don't fail
		log.Warnf("failed to fetch remote commit hash for %s@%s validation: %v", repo, ref, err)
		log.Warnf("continuing with cached template (network error)")
		return true, nil
	}

	if storedHash == remoteHash {
		log.Debugf("✅ commit hash unchanged for %s@%s: %s", repo, ref, truncateHash(storedHash))
		return true, nil
	}

	log.Infof("🔄 commit hash changed for %s@%s: %s -> %s", repo, ref, truncateHash(storedHash), truncateHash(remoteHash))
	return false, nil
}

// invalidateCache removes the cached object to force re-rendering
func invalidateCache(objDir string) error {
	if err := os.RemoveAll(objDir); err != nil {
		return fmt.Errorf("failed to invalidate cache directory %s: %w", objDir, err)
	}
	log.Infof("❌ cache invalidated: %s", objDir)
	return nil
}

// truncateHash returns a shortened version of a commit hash for display purposes
func truncateHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
