//nolint:errcheck
package run

import (
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestComputeTemplatePathsBasic (moved from run_helpers_test.go) validates path/key derivation and directory creation.
func TestComputeTemplatePathsBasic(t *testing.T) {
	target := config.Target{Template: &config.Template{Repo: "stub", Ref: "main", Path: "dir/file.tpl"}}
	vars := map[string]any{"A": 1}
	paths, err := computeTemplatePaths("myt", target, target.Template, vars)
	if err != nil {
		t.Fatalf("computeTemplatePaths: %v", err)
	}
	if paths.remoteKey == "" || paths.renderedKey == "" {
		t.Fatalf("expected non-empty keys; remote=%q rendered=%q", paths.remoteKey, paths.renderedKey)
	}
	if got := paths.base; got != "file" {
		t.Fatalf("expected base 'file', got %q", got)
	}
}

// TestComputeRenderedCacheKeyOrderIndependence ensures insertion order of map keys
// does not affect the resulting rendered cache key.
func TestComputeRenderedCacheKeyOrderIndependence(t *testing.T) {
	m1 := map[string]any{"alpha": 1, "beta": "two", "gamma": true}
	m2 := map[string]any{"gamma": true, "beta": "two", "alpha": 1} // different literal order
	k1, err := computeRenderedCacheKey(m1)
	if err != nil {
		t.Fatalf("k1: %v", err)
	}
	k2, err := computeRenderedCacheKey(m2)
	if err != nil {
		t.Fatalf("k2: %v", err)
	}
	if k1 != k2 {
		t.Fatalf("expected identical keys for order-independent maps, got %s vs %s", k1, k2)
	}
}

// TestComputeRenderedCacheKeyStability verifies repeated calls with the same map yield identical keys.
func TestComputeRenderedCacheKeyStability(t *testing.T) {
	m := map[string]any{"x": 42, "y": "val"}
	k1, err := computeRenderedCacheKey(m)
	if err != nil {
		t.Fatalf("k1: %v", err)
	}
	k2, err := computeRenderedCacheKey(m)
	if err != nil {
		t.Fatalf("k2: %v", err)
	}
	if k1 != k2 {
		t.Fatalf("expected stable key, got %s vs %s", k1, k2)
	}
}

// TestComputeRenderedCacheKeyValueSensitivity ensures that changing a value changes the key.
func TestComputeRenderedCacheKeyValueSensitivity(t *testing.T) {
	base := map[string]any{"x": 1, "y": "a"}
	mod := map[string]any{"x": 1, "y": "b"} // only value of y differs
	kBase, err := computeRenderedCacheKey(base)
	if err != nil {
		t.Fatalf("base: %v", err)
	}
	kMod, err := computeRenderedCacheKey(mod)
	if err != nil {
		t.Fatalf("mod: %v", err)
	}
	if kBase == kMod {
		t.Fatalf("expected different keys when a variable value changes")
	}
}

// TestComputeRemoteCacheKeyDeterminism validates that remote key is stable for same repo/ref/path
// and changes if any component differs.
func TestComputeRemoteCacheKeyDeterminism(t *testing.T) {
	repo, ref, path := "git@ex/repo.git", "main", "templates/app.tpl"
	k1, err := computeRemoteCacheKey(repo, ref, path)
	if err != nil {
		t.Fatalf("k1: %v", err)
	}
	k2, err := computeRemoteCacheKey(repo, ref, path)
	if err != nil {
		t.Fatalf("k2: %v", err)
	}
	if k1 != k2 {
		t.Fatalf("expected identical remote keys; got %s vs %s", k1, k2)
	}

	type variant struct {
		repo, ref, path string
	}
	variants := []variant{
		{repo: "git@ex/other.git", ref: ref, path: path},
		{repo: repo, ref: "dev", path: path},
		{repo: repo, ref: ref, path: "templates/other.tpl"},
	}
	for i, v := range variants {
		kDiff, err := computeRemoteCacheKey(v.repo, v.ref, v.path)
		if err != nil {
			t.Fatalf("variant %d: %v", i, err)
		}
		if kDiff == k1 {
			t.Fatalf("variant %d produced same key (%s) unexpectedly", i, kDiff)
		}
	}
}

// TestRemoteKeyIndependenceFromVariables ensures that variable changes do not affect the remote key.
func TestRemoteKeyIndependenceFromVariables(t *testing.T) {
	repo, ref, path := "git@example/repo.git", "main", "dir/tpl.tpl"
	k, err := computeRemoteCacheKey(repo, ref, path)
	if err != nil {
		t.Fatalf("compute remote: %v", err)
	}
	// change variables (simulate)
	vars1 := map[string]any{"a": 1}
	vars2 := map[string]any{"a": 2, "b": "x"}
	// remote key unaffected
	kAgain, err := computeRemoteCacheKey(repo, ref, path)
	if err != nil {
		t.Fatalf("compute remote again: %v", err)
	}
	if k != kAgain {
		t.Fatalf("remote key changed unexpectedly with variable changes")
	}
	// rendered keys differ
	r1, _ := computeRenderedCacheKey(vars1)
	r2, _ := computeRenderedCacheKey(vars2)
	if r1 == r2 {
		t.Fatalf("expected rendered keys to differ with different variable sets")
	}
}
