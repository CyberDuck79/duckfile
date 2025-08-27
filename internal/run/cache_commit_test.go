//nolint:errcheck
package run

import (
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestCacheKeyIntegrationWithConfig tests cache key computation using actual config resolution.
func TestCacheKeyIntegrationWithConfig(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.DuckConf{
		Version: 1,
		Settings: &config.Settings{
			TrackCommitHash: true, // Global setting enabled
		},
	}

	template := &config.Template{
		Repo: "https://github.com/test/repo.git",
		Ref:  "main",
		Path: "template.tpl",
		// TrackCommitHash not set, should inherit from global
	}

	// Test resolution
	trackCommitHash := config.ResolveTrackCommitHash(nil, template, cfg)
	if !trackCommitHash {
		t.Fatal("expected trackCommitHash to be true from global settings")
	}

	// Test cache key computation
	vars := map[string]any{"TEST": "value"}
	remoteKey1, err := computeRemoteCacheKey(template.Repo, template.Ref, template.Path)
	if err != nil {
		t.Fatalf("remote key1: %v", err)
	}
	renderedKey1, err := computeRenderedCacheKey(vars)
	if err != nil {
		t.Fatalf("rendered key1: %v", err)
	}

	// Change global setting and verify key changes
	cfg.Settings.TrackCommitHash = false
	trackCommitHash2 := config.ResolveTrackCommitHash(nil, template, cfg)
	if trackCommitHash2 {
		t.Fatal("expected trackCommitHash to be false from updated global settings")
	}

	remoteKey2, _ := computeRemoteCacheKey(template.Repo, template.Ref, template.Path)
	renderedKey2, _ := computeRenderedCacheKey(vars)
	if remoteKey1 != remoteKey2 {
		t.Fatalf("remote key should not change when tracking setting changes")
	}
	if renderedKey1 != renderedKey2 {
		t.Fatalf("rendered key should not change when tracking setting changes")
	}
}
