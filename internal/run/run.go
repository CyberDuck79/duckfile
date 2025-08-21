// Package run provides the core execution logic for Duckfile operations.
//
// The main functions are:
//   - Exec: Renders a template and executes the target binary
//   - Sync: Renders templates without executing (useful for cache pre-population)
//   - Clean: Removes cached templates and artifacts
//
// The Exec and Sync functions share common template preparation logic through
// prepareAndRenderTemplate, which handles variable resolution, repository cloning,
// checksum validation, template rendering, and cache management. This ensures
// consistent behavior and reduces code duplication.
package run

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"os"
	"os/exec"
	"time"

	"text/template"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/git"
	"github.com/CyberDuck79/duckfile/internal/log"
	sprig "github.com/Masterminds/sprig/v3"
)

// Test seams (overridable in tests for determinism / stubbing)
var (
	nowFunc              = time.Now
	getenvFunc           = os.Getenv
	execCommand          = exec.Command
	cloneFunc            = git.CloneInto
	getRemoteCommitFunc  = git.GetRemoteCommitHash
	getCurrentCommitFunc = git.GetCurrentCommitHash
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

// Search target from configuration, return effective target name and configuration or error if unknown target.
func searchTarget(cfg *config.DuckConf, targetName string) (string, config.Target, error) {
	// Determine the effective target key
	key := targetName
	if strings.TrimSpace(key) == "" || key == "default" { // "default" still accepted for backwards CLI invocation
		key = cfg.Default
	}
	t, ok := cfg.Targets[key]
	if !ok {
		// Provide helpful list
		keys := make([]string, 0, len(cfg.Targets))
		for k := range cfg.Targets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", config.Target{}, fmt.Errorf("unknown target %q; available: %s", key, strings.Join(keys, ", "))
	}
	return key, t, nil
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

	// 1. Resolve variables
	vars, err := resolveVariables(target.Variables)
	if err != nil {
		return nil, err
	}
	log.Infof("🔧 resolved variables: %d", len(vars))
	if log.IsLevelEnabled(log.Debug) {
		for k, v := range vars {
			log.Debugf("var %s=%v", k, v)
		}
	}

	base := strings.TrimSuffix(filepath.Base(target.Template.Path), ".tpl")

	trackCommitHash := config.ResolveTrackCommitHash(trackCommitHashFlag, &target.Template, cfg)

	// 2. Compute two-tier cache keys
	remoteKey, err := computeRemoteCacheKey(target.Template.Repo, target.Template.Ref, target.Template.Path)
	if err != nil {
		return nil, err
	}
	renderedKey, err := computeRenderedCacheKey(vars) // ONLY variables drive rendered key
	if err != nil {
		return nil, err
	}

	remoteDir := filepath.Join(".duck", "objects", "remote", remoteKey)
	renderedDir := filepath.Join(".duck", "objects", "rendered", renderedKey)
	remoteTemplateFile := filepath.Join(remoteDir, filepath.Base(target.Template.Path)) // raw template copy
	renderedFile := filepath.Join(renderedDir, base)

	// per-target directory for symlink
	perTargetDir := filepath.Join(".duck", targetName)
	if err := os.MkdirAll(perTargetDir, 0o755); err != nil {
		return nil, err
	}

	linkPath := target.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(perTargetDir, base)
	}

	// Determine if remote fetch needed
	needRemote := force
	if _, err := os.Stat(remoteTemplateFile); os.IsNotExist(err) {
		needRemote = true
	}

	// Commit hash validation (only if tracking on and remote exists)
	if !needRemote && trackCommitHash {
		log.Infof("🔍 checking remote updates: %s@%s", target.Template.Repo, target.Template.Ref)
		valid, err := validateCachedCommitHash(target.Template.Repo, target.Template.Ref, remoteDir)
		if err != nil {
			return nil, fmt.Errorf("commit hash validation failed: %w", err)
		}
		if !valid {
			autoUpdate := config.ResolveAutoUpdateOnChange(autoUpdateOnChangeFlag, &target.Template, cfg)
			if autoUpdate {
				log.Infof("📦 updating remote cache (commit changed)")
				if err := invalidateCache(remoteDir); err != nil {
					return nil, err
				}
				needRemote = true
			} else {
				storedHash, _ := readCommitHashMetadata(remoteDir)
				remoteHash, _ := getRemoteCommitFunc(target.Template.Repo, target.Template.Ref)
				return nil, fmt.Errorf("template has been updated remotely, but automatic updates are disabled.\n\nTemplate: %s@%s\nCached commit:  %s\nRemote commit:  %s\n\nEnable autoUpdateOnChange or re-run with --force.", target.Template.Repo, target.Template.Ref, truncateHash(storedHash), truncateHash(remoteHash))
			}
		}
	}

	// 3. Fetch / update remote cache
	if needRemote {
		if force {
			log.Infof("🔄 force fetching remote: %s@%s", target.Template.Repo, target.Template.Ref)
		} else {
			log.Infof("🔄 fetch remote: %s@%s", target.Template.Repo, target.Template.Ref)
		}
		if err := os.MkdirAll(remoteDir, 0o755); err != nil {
			return nil, err
		}
		repoDir, err := cloneFunc(target.Template.Repo, target.Template.Ref, remoteDir)
		if err != nil {
			return nil, err
		}
		src := filepath.Join(repoDir, target.Template.Path)
		// checksum validation & warning logic
		if target.Template.Checksum != "" {
			sumFile := filepath.Join(remoteDir, "checksum.sha256")
			if _, err := os.Stat(sumFile); err == nil { // existing checksum
				if oldChecksum, err := os.ReadFile(sumFile); err == nil && string(oldChecksum) == target.Template.Checksum {
					log.Warnf("template config changed but checksum unchanged (repo/ref/path/vars)")
				}
			}
			b, err := os.ReadFile(src)
			if err != nil {
				return nil, fmt.Errorf("failed to read template for checksum validation: %w", err)
			}
			sum := fmt.Sprintf("%x", sha256.Sum256(b))
			if sum != target.Template.Checksum {
				return nil, fmt.Errorf("template checksum mismatch: expected %s, got %s", target.Template.Checksum, sum)
			}
			if err := os.WriteFile(sumFile, []byte(target.Template.Checksum), 0o644); err != nil {
				return nil, fmt.Errorf("failed to write checksum file: %w", err)
			}
		}
		// copy raw template into remoteDir for stable reuse
		raw, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("read remote template: %w", err)
		}
		if err := os.WriteFile(remoteTemplateFile, raw, 0o644); err != nil {
			return nil, fmt.Errorf("cache remote template: %w", err)
		}
		// Always attempt to capture commit hash on fetch; inexpensive and simplifies later enabling of tracking.
		commitHash, err := getCurrentCommitFunc(repoDir)
		if err != nil {
			log.Debugf("skip commit hash capture (unavailable): %v", err)
		} else if err := writeCommitHashMetadata(remoteDir, commitHash); err != nil {
			log.Warnf("failed to write commit hash metadata: %v", err)
		}
	}

	// 4. Determine if render needed
	needRender := force
	if _, err := os.Stat(renderedFile); os.IsNotExist(err) {
		needRender = true
	}
	if needRemote {
		needRender = true
	} // remote changed implies re-render

	if needRender {
		log.Infof("🎨 render template -> %s", renderedFile)
		if err := os.MkdirAll(renderedDir, 0o755); err != nil {
			return nil, err
		}
		if err := renderTemplate(remoteTemplateFile, renderedFile, target, vars); err != nil {
			return nil, err
		}
		// store linkage to remote key for potential future GC
		_ = os.WriteFile(filepath.Join(renderedDir, "remote.key"), []byte(remoteKey), 0o644)
	}

	// 5. Detect old rendered key from existing symlink
	oldRenderedKey := ""
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if dest, err := os.Readlink(linkPath); err == nil {
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(linkPath), dest)
			}
			if abs, err := filepath.Abs(dest); err == nil {
				objDirPrev := filepath.Dir(abs)
				// expect .../objects/rendered/<key>
				if filepath.Base(filepath.Dir(filepath.Dir(objDirPrev))) == "objects" && filepath.Base(filepath.Dir(objDirPrev)) == "rendered" {
					oldRenderedKey = filepath.Base(objDirPrev)
				}
			}
		}
	}

	// 6. Update symlink
	if err := ensureSymlink(renderedFile, linkPath); err != nil {
		return nil, err
	}
	log.Infof("🔗 symlink %s -> %s", linkPath, renderedFile)

	// 7. Prune old rendered cache if changed
	if oldRenderedKey != "" && oldRenderedKey != renderedKey {
		_ = os.RemoveAll(filepath.Join(".duck", "objects", "rendered", oldRenderedKey))
	}

	return &PrepareTemplateResult{ObjFile: renderedFile, LinkPath: linkPath, OldRenderedKey: oldRenderedKey, RenderedKey: renderedKey, RemoteKey: remoteKey}, nil
}

// executeTarget handles the binary execution portion of Exec.
// It takes the target configuration, rendered file path, and user arguments
// and executes the binary with proper argument ordering.
func executeTarget(target config.Target, linkPath string, passthrough []string) error {
	// Order: [fileFlag linkPath] + target default args + user passthrough args
	args := append([]string{target.FileFlag, linkPath}, []string(target.Args)...)
	args = append(args, passthrough...)
	cmd := execCommand(target.Binary, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	log.Infof("🚀 exec: %s %s", target.Binary, strings.Join(args, " "))
	return cmd.Run()
}

// Exec renders and executes one target.
//
// This function uses the shared prepareAndRenderTemplate function for template
// processing (variable resolution, caching, checksum validation, etc.) and then
// executes the target's binary with the rendered template.
func Exec(cfg *config.DuckConf, targetName string, passthrough []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
	// Determine the effective target key
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return err
	}

	log.Infof("▶️ exec target %q", key)

	// Ensure executable configuration is present
	if strings.TrimSpace(t.Binary) == "" {
		return fmt.Errorf("target %q has no binary configured; use 'duck sync %s' to render without executing", key, key)
	}

	// Prepare template (shared logic with Sync)
	result, err := prepareAndRenderTemplate(key, t, cfg, false, securityCfg, trackCommitHashFlag, autoUpdateOnChangeFlag)
	if err != nil {
		return err
	}

	// Execute underlying binary with the rendered template
	return executeTarget(t, result.LinkPath, passthrough)
}

func renderTemplate(src, dst string, targ config.Target, data map[string]any) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Build template with sprig functions and a small set of extras
	funcMap := sprig.TxtFuncMap()
	funcMap["now"] = nowFunc
	funcMap["env"] = getenvFunc

	// Delimiters: default {{ }}, overridable by config
	left, right := "{{", "}}"
	if targ.Template.Delims != nil {
		if l := strings.TrimSpace(targ.Template.Delims.Left); l != "" {
			left = l
		}
		if r := strings.TrimSpace(targ.Template.Delims.Right); r != "" {
			right = r
		}
	}

	tmpl := template.New(filepath.Base(src)).Funcs(funcMap).Delims(left, right)

	// Missing-key policy: allowMissing => zero (empty strings), else strict error
	if targ.Template.AllowMissing {
		tmpl = tmpl.Option("missingkey=zero")
	} else {
		tmpl = tmpl.Option("missingkey=error")
	}

	tpl, err := tmpl.Parse(string(raw))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

func resolveVariables(in map[string]config.VarValue) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch v.Kind {
		case config.VarLiteral:
			out[k] = v.Value
		case config.VarEnv:
			out[k] = os.Getenv(v.Arg)
		case config.VarFile:
			b, err := os.ReadFile(v.Arg)
			if err != nil {
				return nil, fmt.Errorf("read file for var %s: %w", k, err)
			}
			out[k] = string(b)
		case config.VarCmd:
			// Execute with /bin/sh -c to match spec
			cmd := exec.Command("/bin/sh", "-c", v.Arg)
			cmd.Env = os.Environ()
			outb, err := cmd.Output()
			if err != nil {
				// bubble up stderr if possible
				if ee, ok := err.(*exec.ExitError); ok {
					return nil, fmt.Errorf("cmd var %s failed: %v: %s", k, err, string(ee.Stderr))
				}
				return nil, fmt.Errorf("cmd var %s failed: %w", k, err)
			}
			// Trim trailing newline for typical CLI output
			out[k] = strings.TrimRight(string(outb), "\r\n")
		default:
			out[k] = v.Value
		}
	}
	return out, nil
}

// computeCacheKey builds a stable SHA1 over repo/ref/path, resolved vars, and commit tracking settings.
// computeRemoteCacheKey builds a stable SHA1 over repo/ref/path only.
func computeRemoteCacheKey(repo, ref, path string) (string, error) {
	payload := map[string]string{"repo": repo, "ref": ref, "path": path}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:]), nil
}

// computeRenderedCacheKey builds a stable SHA1 over resolved variables only (order independent).
func computeRenderedCacheKey(vars map[string]any) (string, error) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	type kv struct {
		K string `json:"k"`
		V any    `json:"v"`
	}
	pairs := make([]kv, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, kv{K: k, V: vars[k]})
	}
	b, err := json.Marshal(pairs)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:]), nil
}

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

func ensureSymlink(target, link string) error {
	// Ensure parent dir of link exists
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}

	// Resolve absolute target, then prefer a relative path from link dir
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	linkDir := filepath.Dir(link)
	relTarget, relErr := filepath.Rel(linkDir, absTarget)
	targetForLink := absTarget
	if relErr == nil && relTarget != "" && !strings.HasPrefix(relTarget, ".."+string(filepath.Separator)+"..") {
		// Use relative if it doesn’t escape too far up; keeps links portable inside .duck
		targetForLink = relTarget
	}

	// If a link/file exists, replace it unless it already matches
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if dest, err := os.Readlink(link); err == nil && dest == targetForLink {
				return nil // already correct
			}
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	}

	return os.Symlink(targetForLink, link)
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

// Clean removes cached objects and per-target working dirs.
// If targetName is empty, purge everything. Otherwise, clean only that target’s cache
// and its currently referenced object.
func Clean(cfg *config.DuckConf, targetName string) error {
	if strings.TrimSpace(targetName) == "" { // clean all
		log.Infof("🧹 clean all")
		for name, t := range cfg.Targets {
			_ = cleanOne(name, t)
		}
		return os.RemoveAll(filepath.Join(".duck", "objects"))
	}
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return err
	}
	log.Infof("🧽 clean %q", key)
	return cleanOne(key, t)
}

func cleanOne(targetName string, t config.Target) error {
	base := strings.TrimSuffix(filepath.Base(t.Template.Path), ".tpl")
	cacheDir := filepath.Join(".duck", targetName)
	linkPath := t.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(cacheDir, base)
	}
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if renderedKey := detectRenderedKeyFromSymlink(linkPath); renderedKey != "" {
			log.Debugf("remove rendered object %s", renderedKey)
			_ = os.RemoveAll(filepath.Join(".duck", "objects", "rendered", renderedKey))
		}
		_ = os.Remove(linkPath)
		log.Infof("🗂️ removed %s", linkPath)
	}
	log.Debugf("remove target dir %s", cacheDir)
	return os.RemoveAll(cacheDir)
}

func detectRenderedKeyFromSymlink(linkPath string) string {
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if dest, err := os.Readlink(linkPath); err == nil {
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(linkPath), dest)
			}
			if abs, err := filepath.Abs(dest); err == nil {
				objDirPrev := filepath.Dir(abs)
				if filepath.Base(filepath.Dir(filepath.Dir(objDirPrev))) == "objects" && filepath.Base(filepath.Dir(objDirPrev)) == "rendered" {
					return filepath.Base(objDirPrev)
				}
			}
		}
	}
	return ""
}

func collectTargets(cfg *config.DuckConf, targetName string) (map[string]config.Target, error) {
	res := map[string]config.Target{}
	if strings.TrimSpace(targetName) == "" { // all
		for k, v := range cfg.Targets {
			res[k] = v
		}
		return res, nil
	}
	// Determine the effective target key
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return nil, err
	}
	res[key] = t
	return res, nil
}
