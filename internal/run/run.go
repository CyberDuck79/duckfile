package run

import (
	"bytes"
	"fmt"
	"path/filepath"
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

	// 1. Ensure cache dir
	cacheDir := filepath.Join(".duck", targetOrDefault(targetName, "default"))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	// 2. Fetch template repository
	repoDir, err := git.CloneInto(t.Template.Repo, t.Template.Ref, cacheDir)
	if err != nil {
		return err
	}

	// 3. Render the template to destination
	src := filepath.Join(repoDir, t.Template.Path)
	var dst string
	if t.CacheFile != "" {
		dst = t.CacheFile
	} else {
		dstName := strings.TrimSuffix(filepath.Base(t.Template.Path), ".tpl")
		dst = filepath.Join(cacheDir, dstName)
	}

	vars, err := resolveVariables(t.Variables)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	if err = renderTemplate(src, dst, t, vars); err != nil {
		return err
	}

	// 4. Execute underlying binary
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

func renderTemplate(src, dst string, targ config.Target, data map[string]any) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	funcMap := sprig.TxtFuncMap()
	funcMap["now"] = time.Now
	funcMap["env"] = os.Getenv

	// Delimiters
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

	// Missing keys policy: default strict; allowMissing => zero (empty strings)
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
			cmd := exec.Command("/bin/sh", "-c", v.Arg)
			cmd.Env = os.Environ()
			outb, err := cmd.Output()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return nil, fmt.Errorf("cmd var %s failed: %v: %s", k, err, string(ee.Stderr))
				}
				return nil, fmt.Errorf("cmd var %s failed: %w", k, err)
			}
			out[k] = strings.TrimRight(string(outb), "\r\n")
		default:
			out[k] = v.Value
		}
	}
	return out, nil
}
