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
	renderedKey        string
	remoteDir          string
	renderedDir        string
	remoteTemplateFile string
	renderedFile       string
	linkPath           string
}

// computeTemplatePaths derives cache keys and all filesystem paths used during
// template preparation (remote cache dir, rendered cache dir, template file
// locations, and symlink target). It also ensures the per-target directory
// exists. Keys are deterministic and based on repo/ref/path and variables.
func computeTemplatePaths(targetName string, target config.Target, template *config.Template, vars map[string]any) (*templatePaths, error) {
	base := strings.TrimSuffix(filepath.Base(template.Path), ".tpl")
	remoteKey, err := computeRemoteCacheKey(template.Repo, template.Ref, template.Path)
	if err != nil {
		return nil, err
	}
	renderedKey, err := computeRenderedCacheKey(vars)
	if err != nil {
		return nil, err
	}
	remoteDir := filepath.Join(".duck", "objects", "remote", remoteKey)
	renderedDir := filepath.Join(".duck", "objects", "rendered", renderedKey)
	remoteTemplateFile := filepath.Join(remoteDir, filepath.Base(template.Path))
	renderedFile := filepath.Join(renderedDir, base)
	perTargetDir := filepath.Join(".duck", targetName)
	if err := os.MkdirAll(perTargetDir, 0o755); err != nil {
		return nil, err
	}
	linkPath := target.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(perTargetDir, base)
	}
	return &templatePaths{base: base, remoteKey: remoteKey, renderedKey: renderedKey, remoteDir: remoteDir, renderedDir: renderedDir, remoteTemplateFile: remoteTemplateFile, renderedFile: renderedFile, linkPath: linkPath}, nil
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
