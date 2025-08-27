package run

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/log"
)

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

// linkRendered updates (or creates) the per-target symlink to the newly
// rendered file and returns any previously linked rendered cache key for
// potential pruning.
func linkRendered(paths *templatePaths) (string, error) {
	oldRenderedKey := ""
	if fi, err := os.Lstat(paths.linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if dest, err := os.Readlink(paths.linkPath); err == nil {
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(paths.linkPath), dest)
			}
			if abs, err := filepath.Abs(dest); err == nil {
				objDirPrev := filepath.Dir(abs)
				if filepath.Base(filepath.Dir(filepath.Dir(objDirPrev))) == "objects" && filepath.Base(filepath.Dir(objDirPrev)) == "rendered" {
					oldRenderedKey = filepath.Base(objDirPrev)
				}
			}
		}
	}
	if err := ensureSymlink(paths.renderedFile, paths.linkPath); err != nil {
		return "", err
	}
	log.Infof("🔗 symlink %s -> %s", paths.linkPath, paths.renderedFile)
	return oldRenderedKey, nil
}

// pruneOldRendered removes a previous rendered cache directory if it's no
// longer referenced.
func pruneOldRendered(oldRenderedKey, newRenderedKey string) {
	if oldRenderedKey != "" && oldRenderedKey != newRenderedKey {
		_ = os.RemoveAll(filepath.Join(".duck", "objects", "rendered", oldRenderedKey))
	}
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
