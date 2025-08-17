//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestComputeCacheKeyOrderIndependence (property test) ensures map iteration order
// does not affect the produced cache key by supplying the same logical variables
// in different orders.
func TestComputeCacheKeyOrderIndependence(t *testing.T) {
	varsA := map[string]any{"A": 1, "B": "x"}
	varsB := map[string]any{"B": "x", "A": 1}
	k1, _ := computeCacheKey("repo", "main", "p.tpl", varsA, false)
	k2, _ := computeCacheKey("repo", "main", "p.tpl", varsB, false)
	if k1 != k2 {
		t.Fatalf("keys differ: %s vs %s", k1, k2)
	}
}

// TestRenderTemplateDelimsAndAllowMissing verifies custom delimiters are honored
// and that missing variables render as zero values when AllowMissing=true, leaving
// downstream {{ }} placeholders intact.
func TestRenderTemplateDelimsAndAllowMissing(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "file.tpl")
	os.WriteFile(src, []byte("[[ .FOO ]] {{ .BAR }}"), 0o644)
	dst := filepath.Join(tmp, "out.txt")
	targ := config.Target{Template: config.Template{Delims: &config.Delims{Left: "[[", Right: "]]"}, AllowMissing: true}}
	if err := renderTemplate(src, dst, targ, map[string]any{"FOO": "X"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "X {{ .BAR }}" {
		t.Fatalf("unexpected rendered: %q", string(b))
	}
}

// TestComputeCacheKeyUnsupportedType triggers the JSON marshal error path by
// supplying a data structure that is not JSON-representable.
func TestComputeCacheKeyUnsupportedType(t *testing.T) {
	ch := make(chan int)
	_, err := computeCacheKey("repo", "ref", "p.tpl", map[string]any{"CH": ch}, false)
	if err == nil {
		t.Fatal("expected marshaling error, got nil")
	}
}

// TestRenderTemplateInvalidSyntax ensures a template with invalid syntax returns
// a wrapped parse error.
func TestRenderTemplateInvalidSyntax(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "bad.tpl")
	// Missing closing braces
	os.WriteFile(src, []byte("{{ .FOO "), 0o644)
	dst := filepath.Join(tmp, "out.txt")
	targ := config.Target{Template: config.Template{}}
	err := renderTemplate(src, dst, targ, map[string]any{"FOO": "x"})
	if err == nil || !strings.Contains(err.Error(), "parse template") {
		t.Fatalf("expected parse template error, got %v", err)
	}
}
