package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// copyRendered copies the rendered file to the target path, replacing any existing file or symlink.
func copyRendered(src, dst string) error {
	// Ensure parent dir of dst exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// Remove existing file/symlink
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	// Copy file contents
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		cerr := in.Close()
		if cerr != nil {
			log.Warnf("error closing input file: %v", cerr)
		}
	}()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if cerr != nil {
			log.Warnf("error closing output file: %v", cerr)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// ...existing code...

// PrepareTemplateResult holds the results of template preparation
// shared between Exec and Sync operations
type PrepareTemplateResult struct {
	ObjFile        string // Path to rendered object file
	LinkPath       string // Path where symlink should point
	OldRenderedKey string // Previous rendered cache key (for cleanup)
	RenderedKey    string // Current rendered cache key
	RemoteKey      string // Remote cache key (stable for repo/ref/path)

	// New fields for environment variables
	RepoPath     string // Path to cloned repository
	RepoURL      string // Repository URL
	RepoRef      string // Git reference used
	TemplatePath string // Source template file path
	TargetName   string // Target name being executed
	CacheDir     string // Per-target cache directory
}

// prepareAndRenderTemplate handles the complete template preparation workflow
// shared between Exec and Sync operations. It includes variable resolution,
// cache computation, repository cloning, checksum validation, template rendering,
// symlink management, and old cache cleanup.
func prepareAndRenderTemplate(targetName string, target config.Target, cfg *config.DuckConf, force bool, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) (*PrepareTemplateResult, error) {
	log.Infof("🎯 prepare template for target %q", targetName)

	// Validate strict policy mode requirements first
	if err := config.ValidateStrictPolicyMode(securityCfg); err != nil {
		return nil, fmt.Errorf("strict policy mode validation failed: %w", err)
	}

	// Enforce security policies
	policyResult := config.EnforceSecurityPolicies(targetName, &target, securityCfg)
	if !policyResult.Allowed {
		violationMsg := config.FormatPolicyViolations(policyResult)
		return nil, fmt.Errorf("security policy violations prevent execution:\n%s", violationMsg)
	}

	// Log policy warnings if any
	if len(policyResult.Warnings) > 0 {
		warningMsg := config.FormatPolicyViolations(policyResult)
		log.Warnf("Security policy warnings:\n%s", warningMsg)
	}

	// Apply policy overrides to target configuration
	modifiedTarget := config.ApplyPolicyOverrides(&target, securityCfg)
	target = *modifiedTarget

	// Resolve template (either inline or from remote reference)
	template, err := config.ResolveTemplate(target, cfg.Remotes)
	if err != nil {
		return nil, fmt.Errorf("template resolution failed: %w", err)
	}

	// Existing repository access validation (kept for backward compatibility)
	if err := config.ValidateRepoAccess(template.Repo, securityCfg); err != nil {
		return nil, fmt.Errorf("repository access denied: %w", err)
	}

	vars, err := resolveAndLogVariables(target)
	if err != nil {
		return nil, err
	}

	paths, err := computeTemplatePaths(targetName, target, template, vars)
	if err != nil {
		return nil, err
	}

	trackCommitHash := config.ResolveTrackCommitHash(trackCommitHashFlag, template, cfg)

	needRemote, err := decideRemoteFetch(force, trackCommitHash, target, template, cfg, autoUpdateOnChangeFlag, paths)
	if err != nil {
		return nil, err
	}

	if needRemote {
		if err := fetchRemote(force, target, template, paths); err != nil {
			return nil, err
		}
	}

	if err := ensureRendered(force, needRemote, target, template, vars, paths); err != nil {
		return nil, err
	}

	// Determine if we should copy instead of symlink
	isSelf := targetName == "self"
	if isSelf || target.CopyRendered {
		// For self, always copy to config file path
		dst := paths.linkPath
		// Removed empty if isSelf branch (was linter warning)
		if err := copyRendered(paths.renderedFile, dst); err != nil {
			return nil, err
		}
		log.Infof("📄 copied rendered file to %s", dst)
		// No symlink, so oldRenderedKey is empty
		pruneOldRendered("", paths.renderedKey)
		return &PrepareTemplateResult{
			ObjFile:        paths.renderedFile,
			LinkPath:       dst,
			OldRenderedKey: "",
			RenderedKey:    paths.renderedKey,
			RemoteKey:      paths.remoteKey,

			// New fields for environment variables
			RepoPath:     paths.remoteDir,
			RepoURL:      template.Repo,
			RepoRef:      template.Ref,
			TemplatePath: paths.remoteTemplateFile,
			TargetName:   targetName,
			CacheDir:     filepath.Join(".duck", targetName),
		}, nil
	}

	oldRenderedKey, err := linkRendered(paths)
	if err != nil {
		return nil, err
	}
	pruneOldRendered(oldRenderedKey, paths.renderedKey)
	return &PrepareTemplateResult{
		ObjFile:        paths.renderedFile,
		LinkPath:       paths.linkPath,
		OldRenderedKey: oldRenderedKey,
		RenderedKey:    paths.renderedKey,
		RemoteKey:      paths.remoteKey,

		// New fields for environment variables
		RepoPath:     paths.remoteDir,
		RepoURL:      template.Repo,
		RepoRef:      template.Ref,
		TemplatePath: paths.remoteTemplateFile,
		TargetName:   targetName,
		CacheDir:     filepath.Join(".duck", targetName),
	}, nil
}

// Sync renders templates into the cache without executing the target.
// If targetName is empty, all targets (default + named) are synced.
// If force is true, re-render regardless of existing cache.
//
// This function now shares the same template preparation logic as Exec,
// including checksum validation, through the prepareAndRenderTemplate function.
func Sync(cfg *config.DuckConf, targetName string, force bool, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
	// Log policy enforcement summary if security config is present
	if securityCfg != nil {
		policySummary := config.GetPolicyEnforcementSummary(securityCfg)
		log.Infof("🔒 Security: %s", policySummary)
	}

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
