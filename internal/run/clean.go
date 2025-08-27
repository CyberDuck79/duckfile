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
