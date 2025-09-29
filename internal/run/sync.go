package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// getTargetCacheDir returns the cache directory path for a target
func getTargetCacheDir(targetName string) string {
	return filepath.Join(".duck", targetName)
}

// getBaseTemplateName extracts the base name from a template path
func getBaseTemplateName(templatePath string) string {
	return strings.TrimSuffix(filepath.Base(templatePath), ".tpl")
}

// createPrepareTemplateResult creates a PrepareTemplateResult with common fields populated
func createPrepareTemplateResult(paths *templatePaths, resolved config.ResolvedTemplate, targetName, linkPath, oldRenderedKey string) *PrepareTemplateResult {
	return &PrepareTemplateResult{
		ObjFile:        paths.renderedFile,
		LinkPath:       linkPath,
		OldRenderedKey: oldRenderedKey,
		RenderedKey:    paths.renderedKey,
		RemoteKey:      paths.remoteKey,

		// Environment variables
		RepoPath:     filepath.Join(paths.remoteDir, "repo"),
		RepoURL:      resolved.Repo,
		RepoRef:      resolved.Ref,
		TemplatePath: paths.remoteTemplateFile,
		TargetName:   targetName,
		CacheDir:     getTargetCacheDir(targetName),
	}
}

// prepareAndRenderTemplate handles the complete template preparation workflow
// shared between Exec and Sync operations. It includes variable resolution,
// cache computation, repository cloning, checksum validation, template rendering,
// symlink management, and old cache cleanup.
// validateSecurityAndResolveTemplate handles security validation and template resolution
func validateSecurityAndResolveTemplate(targetName string, target *config.Target, cfg *config.DuckConf, securityCfg *config.SecurityConfig) (config.ResolvedTemplate, error) {
	// Validate strict policy mode requirements first
	if err := config.ValidateStrictPolicyMode(securityCfg); err != nil {
		return config.ResolvedTemplate{}, fmt.Errorf("strict policy mode validation failed: %w", err)
	}

	// Enforce security policies
	policyResult := config.EnforceSecurityPolicies(targetName, target, securityCfg)
	if !policyResult.Allowed {
		violationMsg := config.FormatPolicyViolations(policyResult)
		return config.ResolvedTemplate{}, fmt.Errorf("security policy violations prevent execution:\n%s", violationMsg)
	}

	// Log policy warnings if any
	if len(policyResult.Warnings) > 0 {
		warningMsg := config.FormatPolicyViolations(policyResult)
		log.Warnf("Security policy warnings:\n%s", warningMsg)
	}

	// Apply policy overrides to target configuration
	modifiedTarget := config.ApplyPolicyOverrides(target, securityCfg)
	*target = *modifiedTarget

	// Resolve template configuration (handles both remote references and inline configs)
	resolved, err := config.ResolveTemplateConfig(target.Template, cfg.Remotes, cfg.Settings)
	if err != nil {
		return config.ResolvedTemplate{}, fmt.Errorf("failed to resolve template config: %w", err)
	}

	// Repository access validation using resolved config
	if err := config.ValidateRepoAccess(resolved.Repo, securityCfg); err != nil {
		return config.ResolvedTemplate{}, fmt.Errorf("repository access denied: %w", err)
	}

	return resolved, nil
}

// prepareTemplateWorkflow handles the template preparation workflow including fetching and rendering
func prepareTemplateWorkflow(targetName string, target config.Target, resolved config.ResolvedTemplate, cfg *config.DuckConf, force bool, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) (*templatePaths, error) {
	vars, err := resolveAndLogVariables(target)
	if err != nil {
		return nil, err
	}

	paths, err := computeTemplatePathsResolved(targetName, resolved, vars, target.RenderedPath)
	if err != nil {
		return nil, err
	}

	trackCommitHash := config.ResolveTrackCommitHash(trackCommitHashFlag, &target.Template, cfg)

	needRemote, err := decideRemoteFetchResolved(force, trackCommitHash, resolved, cfg, autoUpdateOnChangeFlag, paths)
	if err != nil {
		return nil, err
	}

	if needRemote {
		if err := fetchRemote(force, resolved, paths); err != nil {
			return nil, err
		}
	}

	// Extract template file from remote cache to template cache
	needTemplate, err := decideTemplateFetch(force, needRemote, resolved, paths)
	if err != nil {
		return nil, err
	}

	if needTemplate {
		if err := extractTemplate(resolved, paths); err != nil {
			return nil, err
		}
	}

	if err := ensureRendered(force, needRemote, target, vars, paths); err != nil {
		return nil, err
	}

	return paths, nil
}

func prepareAndRenderTemplate(targetName string, target config.Target, cfg *config.DuckConf, force bool, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) (*PrepareTemplateResult, error) {
	log.Infof("🎯 prepare template for target %q", targetName)

	// Validate security and resolve template configuration
	resolved, err := validateSecurityAndResolveTemplate(targetName, &target, cfg, securityCfg)
	if err != nil {
		return nil, err
	}

	// Prepare and render template
	paths, err := prepareTemplateWorkflow(targetName, target, resolved, cfg, force, trackCommitHashFlag, autoUpdateOnChangeFlag)
	if err != nil {
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
		return createPrepareTemplateResult(paths, resolved, targetName, dst, ""), nil
	}

	oldRenderedKey, err := linkRendered(paths)
	if err != nil {
		return nil, err
	}
	pruneOldRendered(oldRenderedKey, paths.renderedKey)
	return createPrepareTemplateResult(paths, resolved, targetName, paths.linkPath, oldRenderedKey), nil
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
