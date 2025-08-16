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
	sprig "github.com/Masterminds/sprig/v3"
)

// Test seams (overridable in tests for determinism / stubbing)
var (
	nowFunc     = time.Now
	getenvFunc  = os.Getenv
	execCommand = exec.Command
	cloneFunc   = git.CloneInto
)

// PrepareTemplateResult holds the results of template preparation
// shared between Exec and Sync operations
type PrepareTemplateResult struct {
	ObjFile  string // Path to rendered object file
	LinkPath string // Path where symlink should point
	OldKey   string // Previous cache key (for cleanup)
	CacheKey string // Current cache key
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
func prepareAndRenderTemplate(targetName string, target config.Target, force bool, securityCfg *config.SecurityConfig) (*PrepareTemplateResult, error) {
	logInfo("prepare template for target %q", targetName)

	// Validate repository host access before proceeding
	if err := config.ValidateRepoAccess(target.Template.Repo, securityCfg); err != nil {
		return nil, fmt.Errorf("repository access denied: %w", err)
	}

	// 1. Resolve variables first (no need to clone to do this)
	vars, err := resolveVariables(target.Variables)
	if err != nil {
		return nil, err
	}
	logInfo("resolved variables: %d", len(vars))
	if currentLogLevel == LogDebug {
		for k, v := range vars {
			logDebug("var %s=%v", k, v)
		}
	}

	// 2. Compute deterministic cache key and object path
	base := strings.TrimSuffix(filepath.Base(target.Template.Path), ".tpl")

	cacheKey, err := computeCacheKey(target.Template.Repo, target.Template.Ref, target.Template.Path, vars)
	if err != nil {
		return nil, err
	}
	objDir := filepath.Join(".duck", "objects", cacheKey)
	objFile := filepath.Join(objDir, base)
	// Ensure objects dir exists only if we will write into it later.
	logInfo("cache key %.12s", cacheKey)
	logDebug("object dir %s", objDir)

	// 3. Prepare per-target cache dir and compute symlink path
	cacheDir := filepath.Join(".duck", targetName)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	linkPath := target.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(cacheDir, base) // per-target path
	}

	// 4. If object is missing or force is true, fetch template repo and render it; otherwise, skip cloning
	needRender := force
	if !needRender {
		if _, err := os.Stat(objFile); err != nil {
			needRender = true
		}
	}

	if needRender {
		if force {
			logInfo("force re-render")
		} else {
			logInfo("cache miss; rendering template")
		}
		logDebug("clone %s@%s", target.Template.Repo, target.Template.Ref)
		// Fetch template repository at the requested ref
		repoDir, err := cloneFunc(target.Template.Repo, target.Template.Ref, cacheDir)
		if err != nil {
			return nil, err
		}
		src := filepath.Join(repoDir, target.Template.Path)
		// If checksum is configured, validate it against the template file
		if target.Template.Checksum != "" {
			sumFile := filepath.Join(cacheDir, "checksum.sha256")
			// check if there is a checksum file
			if _, err := os.Stat(sumFile); err == nil {
				if oldChecksum, err := os.ReadFile(sumFile); err == nil {
					logDebug("found old checksum %s", oldChecksum)
					if string(oldChecksum) == target.Template.Checksum {
						logWarn("template config (repo/ref/path/vars) changed but checksum is unchanged")
					}
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
		if err := os.MkdirAll(objDir, 0o755); err != nil {
			return nil, err
		}
		if err := renderTemplate(src, objFile, target, vars); err != nil {
			return nil, err
		}
		logInfo("rendered %s -> %s", target.Template.Path, objFile)
	}
	if _, err := os.Stat(objFile); err == nil {
		logDebug("object present %s", objFile)
	}

	// 5. Determine previous key from existing symlink (if any)
	oldKey := ""
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if dest, err := os.Readlink(linkPath); err == nil {
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(linkPath), dest)
			}
			if abs, err := filepath.Abs(dest); err == nil {
				objDirPrev := filepath.Dir(abs)
				objectsDir := filepath.Base(filepath.Dir(objDirPrev))
				if objectsDir == "objects" {
					oldKey = filepath.Base(objDirPrev)
				}
			}
		}
	}

	// 6. Create/update symlink to the current object
	if err := ensureSymlink(objFile, linkPath); err != nil {
		return nil, err
	}
	logInfo("symlink %s -> %s", linkPath, objFile)

	// 7. If the key changed, remove the old object directory to free cache
	if oldKey != "" && oldKey != cacheKey {
		logInfo("prune old key %s", oldKey)
		_ = os.RemoveAll(filepath.Join(".duck", "objects", oldKey))
	}

	return &PrepareTemplateResult{
		ObjFile:  objFile,
		LinkPath: linkPath,
		OldKey:   oldKey,
		CacheKey: cacheKey,
	}, nil
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
	logInfo("exec: %s %s", target.Binary, strings.Join(args, " "))
	return cmd.Run()
}

// Exec renders and executes one target.
//
// This function uses the shared prepareAndRenderTemplate function for template
// processing (variable resolution, caching, checksum validation, etc.) and then
// executes the target's binary with the rendered template.
func Exec(cfg *config.DuckConf, targetName string, passthrough []string, securityCfg *config.SecurityConfig) error {
	// Determine the effective target key
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return err
	}

	logInfo("exec target %q", key)

	// Ensure executable configuration is present
	if strings.TrimSpace(t.Binary) == "" {
		return fmt.Errorf("target %q has no binary configured; use 'duck sync %s' to render without executing", key, key)
	}

	// Prepare template (shared logic with Sync)
	result, err := prepareAndRenderTemplate(key, t, false, securityCfg)
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

// computeCacheKey builds a stable SHA1 over repo/ref/path and resolved vars.
func computeCacheKey(repo, ref, path string, vars map[string]any) (string, error) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	type kv struct {
		K string      `json:"k"`
		V interface{} `json:"v"`
	}
	pairs := make([]kv, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, kv{K: k, V: vars[k]})
	}
	payload := map[string]any{
		"repo": repo,
		"ref":  ref,
		"path": path,
		"vars": pairs,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:]), nil
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
func Sync(cfg *config.DuckConf, targetName string, force bool, securityCfg *config.SecurityConfig) error {
	targets, err := collectTargets(cfg, targetName)
	if err != nil {
		return err
	}
	for name, t := range targets {
		if _, err := prepareAndRenderTemplate(name, t, force, securityCfg); err != nil {
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
		logInfo("clean all")
		for name, t := range cfg.Targets {
			_ = cleanOne(name, t)
		}
		return os.RemoveAll(filepath.Join(".duck", "objects"))
	}
	// Determine the effective target key
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return err
	}
	logInfo("clean %q", key)
	return cleanOne(key, t)
}

func cleanOne(targetName string, t config.Target) error {
	base := strings.TrimSuffix(filepath.Base(t.Template.Path), ".tpl")
	cacheDir := filepath.Join(".duck", targetName)
	linkPath := t.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(cacheDir, base)
	}
	// Remove symlink if it exists
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		// Remove the object pointed by this symlink as well
		if key := detectKeyFromSymlink(linkPath); key != "" {
			logDebug("remove object %s", key)
			_ = os.RemoveAll(filepath.Join(".duck", "objects", key))
		}
		_ = os.Remove(linkPath)
		logInfo("removed %s", linkPath)
	}
	// Remove per-target cache dir (cloned repo path etc.)
	logDebug("remove cache dir %s", cacheDir)
	return os.RemoveAll(cacheDir)
}

func detectKeyFromSymlink(linkPath string) string {
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if dest, err := os.Readlink(linkPath); err == nil {
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(linkPath), dest)
			}
			if abs, err := filepath.Abs(dest); err == nil {
				objDirPrev := filepath.Dir(abs)
				if filepath.Base(filepath.Dir(objDirPrev)) == "objects" {
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
