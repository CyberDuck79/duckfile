package run

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
)

type templatePaths struct {
	base               string
	remoteKey          string
	templateKey        string
	renderedKey        string
	remoteDir          string
	templateDir        string
	renderedDir        string
	remoteTemplateFile string
	renderedFile       string
	linkPath           string
}

// computeTemplatePaths derives cache keys and all filesystem paths used during
// template preparation (remote cache dir, template cache dir, rendered cache dir,
// template file locations, and symlink target). It also ensures the per-target directory
// exists. Keys are deterministic and based on repo/ref, path, and variables.
func computeTemplatePaths(targetName string, target config.Target, vars map[string]any) (*templatePaths, error) {
	// For backward compatibility, resolve template config inline
	resolved, err := config.ResolveTemplateConfig(target.Template, nil, nil)
	if err != nil {
		return nil, err
	}
	return computeTemplatePathsResolved(targetName, resolved, vars, target.RenderedPath)
}

// computeTemplatePathsResolved works with an already-resolved template configuration
func computeTemplatePathsResolved(targetName string, resolved config.ResolvedTemplate, vars map[string]any, renderedPath string) (*templatePaths, error) {
	base := strings.TrimSuffix(filepath.Base(resolved.Path), ".tpl")

	// Remote key is shared across all templates using the same repo+ref
	remoteKey, err := computeRemoteCacheKey(resolved.Repo, resolved.Ref)
	if err != nil {
		return nil, err
	}

	// Template key is specific to the template path within the remote
	templateKey, err := computeTemplateCacheKey(remoteKey, resolved.Path)
	if err != nil {
		return nil, err
	}

	renderedKey, err := computeRenderedCacheKey(vars)
	if err != nil {
		return nil, err
	}

	// New cache structure: separate remote and template caches
	remoteDir := filepath.Join(".duck", "objects", "remote", remoteKey)
	templateDir := filepath.Join(".duck", "objects", "template", templateKey)
	renderedDir := filepath.Join(".duck", "objects", "rendered", renderedKey)

	// Template file is now extracted from remote to template cache
	remoteTemplateFile := filepath.Join(templateDir, "raw.tpl")
	renderedFile := filepath.Join(renderedDir, base)

	perTargetDir := filepath.Join(".duck", targetName)
	if err := os.MkdirAll(perTargetDir, 0o755); err != nil {
		return nil, err
	}
	linkPath := renderedPath
	if linkPath == "" {
		linkPath = filepath.Join(perTargetDir, base)
	}

	return &templatePaths{
		base:               base,
		remoteKey:          remoteKey,
		templateKey:        templateKey,
		renderedKey:        renderedKey,
		remoteDir:          remoteDir,
		templateDir:        templateDir,
		renderedDir:        renderedDir,
		remoteTemplateFile: remoteTemplateFile,
		renderedFile:       renderedFile,
		linkPath:           linkPath,
	}, nil
}

// computeRemoteCacheKey builds a stable SHA1 over repo+ref only (path removed for sharing)
func computeRemoteCacheKey(repo, ref string) (string, error) {
	payload := map[string]string{"repo": repo, "ref": ref}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:]), nil
}

// computeTemplateCacheKey builds a stable SHA1 over remoteKey+path for template-specific caching
func computeTemplateCacheKey(remoteKey, path string) (string, error) {
	payload := map[string]string{"remote": remoteKey, "path": path}
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
