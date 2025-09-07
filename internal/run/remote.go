package run

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// fetchRemote clones (or reclones into) the remote cache directory, validates
// (optional) checksum, caches the raw template file, and captures the commit
// hash for future validation.
func fetchRemote(force bool, target config.Target, paths *templatePaths) error {
	if force {
		log.Infof("🔄 force fetching remote: %s@%s", target.Template.Repo, target.Template.Ref)
	} else {
		log.Infof("🔄 fetch remote: %s@%s", target.Template.Repo, target.Template.Ref)
	}
	if err := os.MkdirAll(paths.remoteDir, 0o755); err != nil {
		return err
	}
	repoDir, err := cloneFunc(target.Template.Repo, target.Template.Ref, paths.remoteDir, target.Template.Submodules)
	if err != nil {
		return err
	}
	src := filepath.Join(repoDir, target.Template.Path)
	if err := validateAndCacheTemplateChecksum(target, src, paths.remoteDir); err != nil { // includes checksum logic
		return err
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read remote template: %w", err)
	}
	if err := os.WriteFile(paths.remoteTemplateFile, raw, 0o644); err != nil {
		return fmt.Errorf("cache remote template: %w", err)
	}
	// capture commit hash (best-effort)
	if commitHash, err := getCurrentCommitFunc(repoDir); err != nil {
		log.Debugf("skip commit hash capture (unavailable): %v", err)
	} else if err := writeCommitHashMetadata(paths.remoteDir, commitHash); err != nil {
		log.Warnf("failed to write commit hash metadata: %v", err)
	}
	return nil
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
