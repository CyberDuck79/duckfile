package run

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

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
	// Compute template paths to identify cache keys for this target
	vars := map[string]any{} // Use empty vars for cache key computation
	paths, err := computeTemplatePaths(targetName, t, vars)
	if err != nil {
		log.Warnf("failed to compute paths for target %s: %v", targetName, err)
		// Fall back to basic cleanup
		return cleanOneBasic(targetName, t)
	}

	// Clean rendered cache (if symlink exists)
	base := strings.TrimSuffix(filepath.Base(t.Template.Path), ".tpl")
	cacheDir := filepath.Join(".duck", targetName)
	linkPath := t.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(cacheDir, base)
	}
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if renderedKey := detectRenderedKeyFromSymlink(linkPath); renderedKey != "" {
			renderedDir := filepath.Join(".duck", "objects", "rendered", renderedKey)
			log.Debugf("remove rendered object %s", renderedKey)
			_ = os.RemoveAll(renderedDir)
		}
		_ = os.Remove(linkPath)
		log.Infof("🗂️ removed %s", linkPath)
	}

	// Clean template cache specific to this target
	templateDir := paths.templateDir
	if _, err := os.Stat(templateDir); err == nil {
		log.Debugf("remove template cache %s", paths.templateKey)
		_ = os.RemoveAll(templateDir)
	}

	// Clean remote cache - but be careful as it might be shared
	// For now, we'll be conservative and only clean if no other targets use it
	if shouldCleanRemoteCache(t, paths.remoteKey) {
		remoteDir := paths.remoteDir
		if _, err := os.Stat(remoteDir); err == nil {
			log.Debugf("remove remote cache %s", paths.remoteKey)
			_ = os.RemoveAll(remoteDir)
		}
	}

	// Clean target directory
	log.Debugf("remove target dir %s", cacheDir)
	return os.RemoveAll(cacheDir)
}

// cleanOneBasic provides fallback cleanup when path computation fails
func cleanOneBasic(targetName string, t config.Target) error {
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

// shouldCleanRemoteCache determines if a remote cache can be safely removed
// For now, we'll be conservative and avoid removing shared remote caches during single target clean
func shouldCleanRemoteCache(t config.Target, remoteKey string) bool {
	// Conservative approach: don't clean remote cache for single target clean
	// Remote caches are shared and should only be cleaned during "clean all"
	return false
}
