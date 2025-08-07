package run

import (
	"path/filepath"
	"strings"

	"os"
	"os/exec"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/git"
)

// Exec renders (still just copies) and executes one target.
func Exec(cfg *config.DuckConf, targetName string, passthrough []string) error {
	t := cfg.Default
	if targetName != "" {
		t = cfg.Targets[targetName]
	}

	// ------------------------------------------------------------------ #
	// 1. Ensure .duck/<target>/ exists
	// ------------------------------------------------------------------ #
	cacheDir := filepath.Join(".duck", targetOrDefault(targetName, "default"))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	// ------------------------------------------------------------------ #
	// 2. Fetch template repository
	// ------------------------------------------------------------------ #
	repoDir, err := git.CloneInto(t.Template.Repo, t.Template.Ref, cacheDir)
	if err != nil {
		return err
	}

	// ------------------------------------------------------------------ #
	// 3. Copy (later: render) the template to destination
	// ------------------------------------------------------------------ #
	src := filepath.Join(repoDir, t.Template.Path)
	var dst string

	if t.CacheFile != "" { // user forced a custom path
		// honour the exact relative path they gave
		dst = t.CacheFile
	} else { // default behaviour
		dstName := strings.TrimSuffix(filepath.Base(t.Template.Path), ".tpl")
		dst = filepath.Join(cacheDir, dstName)
	}

	if err = copyFile(src, dst); err != nil {
		return err
	}

	// ------------------------------------------------------------------ #
	// 4. Execute underlying binary
	// ------------------------------------------------------------------ #
	args := append([]string{t.FileFlag, dst}, passthrough...)
	cmd := exec.Command(t.Binary, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

func targetOrDefault(t, d string) string {
	if t == "" {
		return d
	}
	return t
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
