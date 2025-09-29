package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// fetchRemote clones (or reclones into) the remote cache directory at the repository level.
// This is now separated from template extraction to enable sharing across multiple templates.
func fetchRemote(force bool, resolved config.ResolvedTemplate, paths *templatePaths) error {
	if force {
		log.Infof("🔄 force fetching remote: %s@%s", resolved.Repo, resolved.Ref)
	} else {
		log.Infof("🔄 fetch remote: %s@%s", resolved.Repo, resolved.Ref)
	}
	if err := os.MkdirAll(paths.remoteDir, 0o755); err != nil {
		return err
	}

	// Clone the entire repository into remote cache (shared across templates)
	repoDir, err := cloneFunc(resolved.Repo, resolved.Ref, paths.remoteDir, resolved.Submodules)
	if err != nil {
		return err
	}

	// Store remote metadata for future reference
	metadata := map[string]string{
		"repo": resolved.Repo,
		"ref":  resolved.Ref,
	}
	metadataBytes, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(paths.remoteDir, "metadata.json"), metadataBytes, 0o644); err != nil {
		log.Warnf("failed to write remote metadata: %v", err)
	}

	// capture commit hash (best-effort) at remote level
	if commitHash, err := getCurrentCommitFunc(repoDir); err != nil {
		log.Debugf("skip commit hash capture (unavailable): %v", err)
	} else if err := writeCommitHashMetadata(paths.remoteDir, commitHash); err != nil {
		log.Warnf("failed to write commit hash metadata: %v", err)
	}
	return nil
}

// extractTemplate extracts a specific template file from the remote cache to the template cache.
// This allows multiple templates to share the same remote repository while caching individual files.
func extractTemplate(resolved config.ResolvedTemplate, paths *templatePaths) error {
	if err := os.MkdirAll(paths.templateDir, 0o755); err != nil {
		return err
	}

	// Find the repository directory within remote cache
	repoDir := filepath.Join(paths.remoteDir, "repo")
	src := filepath.Join(repoDir, resolved.Path)

	// Validate checksum if provided
	if resolved.Checksum != "" {
		if err := validateResolvedTemplateChecksum(src, resolved.Checksum); err != nil {
			return err
		}
	}

	// Read and cache the template file
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read template file %s: %w", resolved.Path, err)
	}

	if err := os.WriteFile(paths.remoteTemplateFile, raw, 0o644); err != nil {
		return fmt.Errorf("cache template file: %w", err)
	}

	// Store template metadata
	metadata := map[string]string{
		"remote": paths.remoteKey,
		"path":   resolved.Path,
	}
	if resolved.Checksum != "" {
		metadata["checksum"] = resolved.Checksum
	}
	metadataBytes, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(paths.templateDir, "metadata.json"), metadataBytes, 0o644); err != nil {
		log.Warnf("failed to write template metadata: %v", err)
	}

	return nil
}

// decideTemplateFetch determines whether the template file needs to be extracted from remote cache
func decideTemplateFetch(force, needRemote bool, resolved config.ResolvedTemplate, paths *templatePaths) (bool, error) {
	// If we just fetched the remote, we need to extract the template
	if needRemote {
		return true, nil
	}

	// Check if template cache exists
	if _, err := os.Stat(paths.remoteTemplateFile); os.IsNotExist(err) {
		return true, nil
	}

	return force, nil
}

// decideRemoteFetchResolved determines whether the remote repository needs to be fetched
// based on the resolved template configuration
// handleCommitHashValidation handles validation and auto-update logic for resolved templates
func handleCommitHashValidation(resolved config.ResolvedTemplate, autoUpdateOnChangeFlag *bool, paths *templatePaths) (bool, error) {
	log.Infof("🔍 checking remote updates: %s@%s", resolved.Repo, resolved.Ref)
	valid, err := validateCachedCommitHash(resolved.Repo, resolved.Ref, paths.remoteDir)
	if err != nil {
		return false, fmt.Errorf("commit hash validation failed: %w", err)
	}

	if valid {
		return false, nil // No update needed
	}

	// Template has changed - determine auto-update behavior
	autoUpdate := resolved.AutoUpdateOnChange
	if autoUpdateOnChangeFlag != nil {
		autoUpdate = *autoUpdateOnChangeFlag
	}

	if autoUpdate {
		log.Infof("📦 updating remote cache (commit changed)")
		if err := invalidateCache(paths.remoteDir); err != nil {
			return false, err
		}
		return true, nil // Need to fetch
	}

	// Auto-update disabled - return informative error
	storedHash, _ := readCommitHashMetadata(paths.remoteDir)
	remoteHash, _ := getRemoteCommitFunc(resolved.Repo, resolved.Ref)
	return false, fmt.Errorf("template has been updated remotely, but automatic updates are disabled.\n\nTemplate: %s@%s\nCached commit:  %s\nRemote commit:  %s\n\nEnable autoUpdateOnChange or re-run with --force", resolved.Repo, resolved.Ref, truncateHash(storedHash), truncateHash(remoteHash))
}

func decideRemoteFetchResolved(force, trackCommitHash bool, resolved config.ResolvedTemplate, cfg *config.DuckConf, autoUpdateOnChangeFlag *bool, paths *templatePaths) (bool, error) {
	needRemote := force

	// Check if remote repository cache exists
	repoDir := filepath.Join(paths.remoteDir, "repo")
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		needRemote = true
	}

	// Handle commit hash validation if enabled and cache exists
	if !needRemote && trackCommitHash {
		fetchNeeded, err := handleCommitHashValidation(resolved, autoUpdateOnChangeFlag, paths)
		if err != nil {
			return false, err
		}
		if fetchNeeded {
			needRemote = true
		}
	}

	return needRemote, nil
}

// decideRemoteFetch determines whether the remote template needs to be fetched
// again based on force flag, presence of cache, and (optionally) commit hash
// tracking with auto-update behavior.
func decideRemoteFetch(force, trackCommitHash bool, target config.Target, cfg *config.DuckConf, autoUpdateOnChangeFlag *bool, paths *templatePaths) (bool, error) {
	needRemote := force
	if _, err := os.Stat(paths.remoteTemplateFile); os.IsNotExist(err) {
		needRemote = true
	}
	if !needRemote && trackCommitHash { // commit hash validation path
		log.Infof("🔍 checking remote updates: %s@%s", target.Template.Repo, target.Template.Ref)
		valid, err := validateCachedCommitHash(target.Template.Repo, target.Template.Ref, paths.remoteDir)
		if err != nil {
			return false, fmt.Errorf("commit hash validation failed: %w", err)
		}
		if !valid {
			autoUpdate := config.ResolveAutoUpdateOnChange(autoUpdateOnChangeFlag, &target.Template, cfg)
			if autoUpdate {
				log.Infof("📦 updating remote cache (commit changed)")
				if err := invalidateCache(paths.remoteDir); err != nil {
					return false, err
				}
				needRemote = true
			} else {
				storedHash, _ := readCommitHashMetadata(paths.remoteDir)
				remoteHash, _ := getRemoteCommitFunc(target.Template.Repo, target.Template.Ref)
				return false, fmt.Errorf("template has been updated remotely, but automatic updates are disabled.\n\nTemplate: %s@%s\nCached commit:  %s\nRemote commit:  %s\n\nEnable autoUpdateOnChange or re-run with --force", target.Template.Repo, target.Template.Ref, truncateHash(storedHash), truncateHash(remoteHash))
			}
		}
	}
	return needRemote, nil
}
