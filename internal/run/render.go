package run

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
	sprig "github.com/Masterminds/sprig/v3"
)

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

// ensureRendered renders the template if needed based on force, remote change,
// or absence of a rendered object. It also records the remote key linkage.
func ensureRendered(force, needRemote bool, target config.Target, vars map[string]any, paths *templatePaths) error {
	needRender := force
	if _, err := os.Stat(paths.renderedFile); os.IsNotExist(err) {
		needRender = true
	}
	if needRemote {
		needRender = true
	}
	if !needRender {
		return nil
	}
	log.Infof("🎨 render template -> %s", paths.renderedFile)
	if err := os.MkdirAll(paths.renderedDir, 0o755); err != nil {
		return err
	}
	if err := renderTemplate(paths.remoteTemplateFile, paths.renderedFile, target, vars); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(paths.renderedDir, "remote.key"), []byte(paths.remoteKey), 0o644)
	return nil
}
