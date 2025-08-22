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
func TestComputeRenderedCacheKeyOrderIndependence(t *testing.T) {
	varsA := map[string]any{"A": 1, "B": "x"}
	varsB := map[string]any{"B": "x", "A": 1}
	k1, _ := computeRenderedCacheKey(varsA)
	k2, _ := computeRenderedCacheKey(varsB)
	if k1 != k2 {
		t.Fatalf("rendered keys differ: %s vs %s", k1, k2)
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
func TestComputeRenderedCacheKeyUnsupportedType(t *testing.T) {
	ch := make(chan int)
	if _, err := computeRenderedCacheKey(map[string]any{"CH": ch}); err == nil {
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

// TestSearchTargetErrorList ensures unknown target error path is covered.
func TestSearchTargetErrorList(t *testing.T) {
	cfg := &config.DuckConf{Version: 1, Default: "build", Targets: map[string]config.Target{"build": {}}}
	if _, _, err := searchTarget(cfg, "nope"); err == nil || !strings.Contains(err.Error(), "unknown target") {
		// Expect formatted list of available targets in error
		t.Fatalf("expected unknown target error, got %v", err)
	}
}

// TestTruncateHash covers short and long branches.
func TestTruncateHash(t *testing.T) {
	short := "abcdef12"
	if truncateHash(short) != short {
		t.Fatalf("short hash modified")
	}
	long := "1234567890abcdef1234567890abcdef12345678"
	if got := truncateHash(long); got != long[:12] {
		t.Fatalf("expected %s got %s", long[:12], got)
	}
}

// TestEnsureSymlinkAlreadyCorrect covers early return when symlink already points correctly.
func TestEnsureSymlinkAlreadyCorrect(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "f.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	link := filepath.Join(tmp, "link.txt")
	if err := ensureSymlink(file, link); err != nil {
		t.Fatalf("create link: %v", err)
	}
	// second call should early-return without error
	if err := ensureSymlink(file, link); err != nil {
		t.Fatalf("ensure again: %v", err)
	}
}
