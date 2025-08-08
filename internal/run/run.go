package run

import (
	"bytes"
	"crypto/sha1"
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

// Exec renders and executes one target.
func Exec(cfg *config.DuckConf, targetName string, passthrough []string) error {
	t := cfg.Default
	if targetName != "" && targetName != "default" {
		var ok bool
		if t, ok = cfg.Targets[targetName]; !ok {
			return fmt.Errorf("unknown target %q", targetName)
		}
	}

	// 1. Resolve variables first (no need to clone to do this)
	vars, err := resolveVariables(t.Variables)
	if err != nil {
		return err
	}

	// 2. Compute deterministic cache key and object path
	base := strings.TrimSuffix(filepath.Base(t.Template.Path), ".tpl")

	key, err := computeCacheKey(t.Template.Repo, t.Template.Ref, t.Template.Path, vars)
	if err != nil {
		return err
	}
	objDir := filepath.Join(".duck", "objects", key)
	objFile := filepath.Join(objDir, base)
	// Ensure objects dir exists only if we will write into it later.

	// 3. Prepare per-target cache dir and compute symlink path
	cacheDir := filepath.Join(".duck", targetOrDefault(targetName, "default"))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	linkPath := t.RenderedPath
	if linkPath == "" {
		linkPath = filepath.Join(cacheDir, base) // per-target path
	}

	// 4. If object is missing, fetch template repo and render it; otherwise, skip cloning
	if _, statErr := os.Stat(objFile); statErr != nil {
		// Fetch template repository at the requested ref
		repoDir, err := git.CloneInto(t.Template.Repo, t.Template.Ref, cacheDir)
		if err != nil {
			return err
		}
		src := filepath.Join(repoDir, t.Template.Path)
		if err := os.MkdirAll(objDir, 0o755); err != nil {
			return err
		}
		if err := renderTemplate(src, objFile, t, vars); err != nil {
			return err
		}
	}

	// 5. Determine previous key from existing symlink (if any)
	oldKey := ""
	if fi, err := os.Lstat(linkPath); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		if dest, err := os.Readlink(linkPath); err == nil {
			// Resolve relative symlink to absolute
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(linkPath), dest)
			}
			if abs, err := filepath.Abs(dest); err == nil {
				// abs is .../.duck/objects/<key>/<base> ideally
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
		return err
	}

	// 7. If the key changed, remove the old object directory to free cache
	if oldKey != "" && oldKey != key {
		_ = os.RemoveAll(filepath.Join(".duck", "objects", oldKey))
	}

	// 8. Execute underlying binary with the symlink
	// Order: [fileFlag linkPath] + target default args + user passthrough args
	args := append([]string{t.FileFlag, linkPath}, []string(t.Args)...)
	args = append(args, passthrough...)
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

func renderTemplate(src, dst string, targ config.Target, data map[string]any) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Build template with sprig functions and a small set of extras
	funcMap := sprig.TxtFuncMap()
	funcMap["now"] = time.Now
	funcMap["env"] = os.Getenv

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
