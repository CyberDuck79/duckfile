package run

import (
	"fmt"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// PrepareTemplateResult holds the results of template preparation
// shared between Exec and Sync operations
type PrepareTemplateResult struct {
	ObjFile        string // Path to rendered object file
	LinkPath       string // Path where symlink should point
	OldRenderedKey string // Previous rendered cache key (for cleanup)
	RenderedKey    string // Current rendered cache key
	RemoteKey      string // Remote cache key (stable for repo/ref/path)
}

// prepareAndRenderTemplate handles the complete template preparation workflow
// shared between Exec and Sync operations. It includes variable resolution,
// cache computation, repository cloning, checksum validation, template rendering,
// symlink management, and old cache cleanup.
func prepareAndRenderTemplate(targetName string, target config.Target, cfg *config.DuckConf, force bool, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) (*PrepareTemplateResult, error) {
	log.Infof("🎯 prepare template for target %q", targetName)

	if err := config.ValidateRepoAccess(target.Template.Repo, securityCfg); err != nil {
		return nil, fmt.Errorf("repository access denied: %w", err)
	}

	vars, err := resolveAndLogVariables(target)
	if err != nil {
		return nil, err
	}

	paths, err := computeTemplatePaths(targetName, target, vars)
	if err != nil {
		return nil, err
	}

	trackCommitHash := config.ResolveTrackCommitHash(trackCommitHashFlag, &target.Template, cfg)

	needRemote, err := decideRemoteFetch(force, trackCommitHash, target, cfg, autoUpdateOnChangeFlag, paths)
	if err != nil {
		return nil, err
	}

	if needRemote {
		if err := fetchRemote(force, target, paths); err != nil {
			return nil, err
		}
	}

	if err := ensureRendered(force, needRemote, target, vars, paths); err != nil {
		return nil, err
	}

	oldRenderedKey, err := linkRendered(paths)
	if err != nil {
		return nil, err
	}

	pruneOldRendered(oldRenderedKey, paths.renderedKey)

	return &PrepareTemplateResult{ObjFile: paths.renderedFile, LinkPath: paths.linkPath, OldRenderedKey: oldRenderedKey, RenderedKey: paths.renderedKey, RemoteKey: paths.remoteKey}, nil
}

// Sync renders templates into the cache without executing the target.
// If targetName is empty, all targets (default + named) are synced.
// If force is true, re-render regardless of existing cache.
//
// This function now shares the same template preparation logic as Exec,
// including checksum validation, through the prepareAndRenderTemplate function.
func Sync(cfg *config.DuckConf, targetName string, force bool, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
	targets, err := collectTargets(cfg, targetName)
	if err != nil {
		return err
	}
	for name, t := range targets {
		if _, err := prepareAndRenderTemplate(name, t, cfg, force, securityCfg, trackCommitHashFlag, autoUpdateOnChangeFlag); err != nil {
			return err
		}
	}
	return nil
}
