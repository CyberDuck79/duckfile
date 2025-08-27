//nolint:errcheck
package run

import (
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestComputeCacheKeyUnsupportedType triggers the JSON marshal error path by
// supplying a data structure that is not JSON-representable.
func TestComputeRenderedCacheKeyUnsupportedType(t *testing.T) {
	ch := make(chan int)
	if _, err := computeRenderedCacheKey(map[string]any{"CH": ch}); err == nil {
		t.Fatal("expected marshaling error, got nil")
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
