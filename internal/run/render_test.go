//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestRenderTemplateDelimsAndAllowMissing verifies custom delimiters are honored
// and that missing variables render as zero values when AllowMissing=true, leaving
// downstream {{ }} placeholders intact.
func TestRenderTemplateDelimsAndAllowMissing(t *testing.T) {
	withTempWD(t, func() {
		src := "file.tpl"
		os.WriteFile(src, []byte("[[ .FOO ]] {{ .BAR }}"), 0o644)
		dst := "out.txt"
		targ := config.Target{Template: config.Template{Delims: &config.Delims{Left: "[[", Right: "]]"}, AllowMissing: true}}
		if err := renderTemplate(src, dst, targ, map[string]any{"FOO": "X"}); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(dst)
		if string(b) != "X {{ .BAR }}" {
			t.Fatalf("unexpected rendered: %q", string(b))
		}
	})
}

// TestRenderTemplateInvalidSyntax ensures a template with invalid syntax returns
// a wrapped parse error.
func TestRenderTemplateInvalidSyntax(t *testing.T) {
	withTempWD(t, func() {
		src := "bad.tpl"
		os.WriteFile(src, []byte("{{ .FOO "), 0o644) // Missing closing braces
		dst := "out.txt"
		targ := config.Target{Template: config.Template{}}
		err := renderTemplate(src, dst, targ, map[string]any{"FOO": "x"})
		if err == nil || !strings.Contains(err.Error(), "parse template") {
			t.Fatalf("expected parse template error, got %v", err)
		}
	})
}

// TestRenderMissingVariableStrict validates missing variables cause an error in strict mode.
func TestRenderMissingVariableStrict(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		templateSrc := filepath.Join("templateSrc")
		os.MkdirAll(templateSrc, 0o755)
		os.WriteFile(filepath.Join(templateSrc, "file.tpl"), []byte("hello {{ .NAME }} {{ .OTHER }}"), 0o644)
		origClone := cloneFunc
		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			dst := filepath.Join(cacheDir, "repo")
			os.MkdirAll(dst, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateSrc, "file.tpl"))
			os.WriteFile(filepath.Join(dst, "file.tpl"), data, 0o644)
			return dst, nil
		}
		defer func() { cloneFunc = origClone }()
		cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "file.tpl"}, Variables: map[string]config.VarValue{"NAME": config.NewLiteralVar("world")}}}}
		err := Sync(cfg, "default", false, defaultSecurityConfig(), nil, nil)
		if err == nil {
			t.Fatalf("expected render error for missing var")
		}
		if !strings.Contains(err.Error(), "missing") && !strings.Contains(err.Error(), "map has no entry") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestEnsureRenderedLogic exercises decision branches without performing full remote fetch.
func TestEnsureRenderedLogic(t *testing.T) {
	withTempWD(t, func() {
		vars := map[string]any{"A": 1}
		p, err := computeTemplatePaths("t", config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}, vars)
		if err != nil {
			t.Fatalf("paths: %v", err)
		}
		os.MkdirAll(p.remoteDir, 0o755)
		os.MkdirAll(p.templateDir, 0o755)
		os.WriteFile(p.remoteTemplateFile, []byte("val {{ .A }}"), 0o644)
		targ := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
		if err := ensureRendered(false, false, targ, vars, p); err != nil {
			t.Fatalf("first render: %v", err)
		}
		fi1, _ := os.Stat(p.renderedFile)
		if err := ensureRendered(false, false, targ, vars, p); err != nil {
			t.Fatalf("second ensure: %v", err)
		}
		fi2, _ := os.Stat(p.renderedFile)
		if fi1.ModTime() != fi2.ModTime() {
			t.Fatalf("expected no rewrite when unchanged")
		}
		if err := ensureRendered(true, false, targ, vars, p); err != nil {
			t.Fatalf("force ensure: %v", err)
		}
	})
}
