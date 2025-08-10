package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

func TestComputeCacheKeyOrderIndependence(t *testing.T) {
	varsA := map[string]any{"A": 1, "B": "x"}
	varsB := map[string]any{"B": "x", "A": 1}
	k1, _ := computeCacheKey("repo", "main", "p.tpl", varsA)
	k2, _ := computeCacheKey("repo", "main", "p.tpl", varsB)
	if k1 != k2 {
		t.Fatalf("keys differ: %s vs %s", k1, k2)
	}
}

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
