//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestDecideRemoteFetchMatrix enumerates core decision branches (without commit tracking branch where remote invalidation needed).
func TestDecideRemoteFetchMatrix(t *testing.T) {
	// temp work dir
	tmp := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(old)

	vars := map[string]any{"X": 1}
	baseTarget := config.Target{Template: config.Template{Repo: "stub", Ref: "main", Path: "f.tpl"}}
	cfg := &config.DuckConf{Version: 1, Targets: map[string]config.Target{"t": baseTarget}, Settings: &config.Settings{TrackCommitHash: true, AutoUpdateOnChange: true}}
	autoFlag := true

	// stub commit hash functions (overridden per-case as needed)
	origRemote := getRemoteCommitFunc
	origCurrent := getCurrentCommitFunc
	defer func() { getRemoteCommitFunc = origRemote; getCurrentCommitFunc = origCurrent }()
	getCurrentCommitFunc = func(dir string) (string, error) { return "hash1", nil }

	type matrixCase struct {
		name         string
		force        bool
		track        bool
		remoteExists bool
		mutateHash   bool
		expect       bool
	}
	cases := []matrixCase{
		{"force_always", true, false, true, false, true},
		{"no_cache_initial", false, false, false, false, true},
		{"cached_no_force", false, false, true, false, false},
		{"cached_track_same_hash", false, true, true, false, false},
		{"cached_track_changed_hash_auto", false, true, true, true, true},
	}

	for _, c := range cases {
		// isolate each case to avoid cross-contamination of remote cache state
		_ = os.RemoveAll(".duck")
		p, err := computeTemplatePaths("t", baseTarget, vars)
		if err != nil {
			t.Fatalf("%s: paths err: %v", c.name, err)
		}
		if c.remoteExists {
			// Create remote cache directory with repository structure
			os.MkdirAll(p.remoteDir, 0o755)
			repoDir := filepath.Join(p.remoteDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			os.WriteFile(filepath.Join(repoDir, "f.tpl"), []byte("template content"), 0o644)
			// Create template cache directory and extracted template
			os.MkdirAll(p.templateDir, 0o755)
			os.WriteFile(p.remoteTemplateFile, []byte("raw"), 0o644)
			// commit hash metadata to enable validation branch when tracking
			writeCommitHashMetadata(p.remoteDir, "hash1")
		}
		// remote hash behavior
		if c.track {
			if c.mutateHash {
				getRemoteCommitFunc = func(repo, ref string) (string, error) { return "hash2", nil }
			} else {
				getRemoteCommitFunc = func(repo, ref string) (string, error) { return "hash1", nil }
			}
		} else {
			getRemoteCommitFunc = func(repo, ref string) (string, error) { return "ignored", nil }
		}
		need, err := decideRemoteFetch(c.force, c.track, baseTarget, cfg, &autoFlag, p)
		if err != nil {
			t.Fatalf("%s: decideRemoteFetch err: %v", c.name, err)
		}
		if need != c.expect {
			t.Fatalf("%s: expected need=%v got %v", c.name, c.expect, need)
		}
	}
}
