//nolint:errcheck
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// Focused unit tests for commit hash metadata helpers & validation.

func TestCommitHashMetadataOperations(t *testing.T) {
	withTempWD(t, func() {
		objDir := "cache_object"
		os.MkdirAll(objDir, 0o755)
		if hasCommitHashMetadata(objDir) {
			t.Fatal("unexpected metadata present")
		}
		if h, err := readCommitHashMetadata(objDir); err != nil || h != "" {
			t.Fatalf("expected empty read, got %q err %v", h, err)
		}
		hash := "a1b2c3d4e5f6789012345678901234567890abcd"
		if err := writeCommitHashMetadata(objDir, hash); err != nil {
			t.Fatalf("write: %v", err)
		}
		if !hasCommitHashMetadata(objDir) {
			t.Fatal("expected metadata after write")
		}
		if h, _ := readCommitHashMetadata(objDir); h != hash {
			t.Fatalf("read mismatch %s", h)
		}
		empty := "empty_cache"
		os.MkdirAll(empty, 0o755)
		if err := writeCommitHashMetadata(empty, ""); err != nil {
			t.Fatalf("write empty: %v", err)
		}
		if hasCommitHashMetadata(empty) {
			t.Fatal("should not have metadata for empty hash write")
		}
	})
}

func TestCommitHashMetadataWhitespace(t *testing.T) {
	withTempWD(t, func() {
		dir := "cache_object"
		os.MkdirAll(dir, 0o755)
		f := filepath.Join(dir, "commit.hash")
		raw := "  a1b2c3d4e5f6789012345678901234567890abcd\n\t  "
		os.WriteFile(f, []byte(raw), 0o644)
		got, err := readCommitHashMetadata(dir)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		want := "a1b2c3d4e5f6789012345678901234567890abcd"
		if got != want {
			t.Fatalf("trim mismatch want %s got %s", want, got)
		}
	})
}

func TestValidateCachedCommitHash_UnchangedChangedNetworkNone(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		objDir := "cache_object"
		os.MkdirAll(objDir, 0o755)
		h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		writeCommitHashMetadata(objDir, h1)
		// unchanged
		getRemoteCommitFunc = func(repo, ref string) (string, error) { return h1, nil }
		if ok, err := validateCachedCommitHash("r", "main", objDir); err != nil || !ok {
			t.Fatalf("unchanged expect ok got %v %v", ok, err)
		}
		// changed
		h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		getRemoteCommitFunc = func(repo, ref string) (string, error) { return h2, nil }
		if ok, err := validateCachedCommitHash("r", "main", objDir); err != nil || ok {
			t.Fatalf("changed expect invalid got ok=%v err=%v", ok, err)
		}
		// network error -> still valid (warning path)
		getRemoteCommitFunc = func(repo, ref string) (string, error) { return "", fmt.Errorf("net err") }
		if ok, err := validateCachedCommitHash("r", "main", objDir); err != nil || !ok {
			t.Fatalf("network path expect ok got ok=%v err=%v", ok, err)
		}
		// no metadata
		empty := "empty"
		os.MkdirAll(empty, 0o755)
		if ok, err := validateCachedCommitHash("r", "main", empty); err != nil || !ok {
			t.Fatalf("no metadata expect ok got %v %v", ok, err)
		}
	})
}

func TestInvalidateCache(t *testing.T) {
	withTempWD(t, func() {
		d := "cache_object"
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0o644)
		writeCommitHashMetadata(d, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if err := invalidateCache(d); err != nil {
			t.Fatalf("invalidate: %v", err)
		}
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Fatalf("dir still exists")
		}
	})
}

// End-to-end behaviors involving prepareAndRenderTemplate (storage, auto-update, disabled auto-update, GC)

func TestCommitHashStorageAndAutoUpdatePaths(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		repoDir := "repo"
		os.MkdirAll(repoDir, 0o755)
		os.WriteFile(filepath.Join(repoDir, "t.tpl"), []byte("hi {{.N}}"), 0o644)
		cloneFunc = func(repo, ref, cacheDir string) (string, error) { return repoDir, nil }
		cfg := &config.DuckConf{Version: 1, Settings: &config.Settings{TrackCommitHash: true, AutoUpdateOnChange: true}, Targets: map[string]config.Target{"t": {Template: config.Template{Repo: "stub", Ref: "main", Path: "t.tpl"}, Variables: map[string]config.VarValue{"N": config.NewLiteralVar("w")}}}}
		h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		getCurrentCommitFunc = func(workdir string) (string, error) { return h1, nil }
		res1, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("first prepare: %v", err)
		}
		remoteDir := filepath.Join(".duck", "objects", "remote", res1.RemoteKey)
		if !hasCommitHashMetadata(remoteDir) {
			t.Fatal("metadata missing")
		}
		if sh, _ := readCommitHashMetadata(remoteDir); sh != h1 {
			t.Fatalf("stored hash mismatch %s", sh)
		}
		// mutate template and change hashes
		os.WriteFile(filepath.Join(repoDir, "t.tpl"), []byte("hi2 {{.N}}"), 0o644)
		getRemoteCommitFunc = func(repo, ref string) (string, error) { return h2, nil }
		getCurrentCommitFunc = func(workdir string) (string, error) { return h2, nil }
		res2, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("second prepare: %v", err)
		}
		if res1.RemoteKey != res2.RemoteKey {
			t.Fatalf("remote key changed")
		}
		if nh, _ := readCommitHashMetadata(remoteDir); nh != h2 {
			t.Fatalf("expected new hash %s", nh)
		}
	})
}

func TestCommitHashChangeWithoutAutoUpdate(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		repoDir := "repo"
		os.MkdirAll(repoDir, 0o755)
		os.WriteFile(filepath.Join(repoDir, "t.tpl"), []byte("hi {{.N}}"), 0o644)
		cloneFunc = func(repo, ref, cacheDir string) (string, error) { return repoDir, nil }
		cfg := &config.DuckConf{Version: 1, Settings: &config.Settings{TrackCommitHash: true, AutoUpdateOnChange: false}, Targets: map[string]config.Target{"t": {Template: config.Template{Repo: "stub", Ref: "main", Path: "t.tpl"}, Variables: map[string]config.VarValue{"N": config.NewLiteralVar("w")}}}}
		h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		getCurrentCommitFunc = func(workdir string) (string, error) { return h1, nil }
		if _, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Fatalf("first prepare: %v", err)
		}
		h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		getRemoteCommitFunc = func(repo, ref string) (string, error) { return h2, nil }
		if _, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil); err == nil {
			t.Fatal("expected error due to changed hash w/out auto-update")
		} else {
			wantPhrases := []string{"template has been updated remotely", "automatic updates are disabled", "Enable autoUpdateOnChange or re-run with --force"}
			for _, p := range wantPhrases {
				if !strings.Contains(err.Error(), p) {
					t.Errorf("error missing phrase %q: %v", p, err)
				}
			}
		}
	})
}

func TestRemoteCacheInvalidationGC(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()
		repoDir := "repo"
		os.MkdirAll(repoDir, 0o755)
		tpl := filepath.Join(repoDir, "t.tpl")
		os.WriteFile(tpl, []byte("v1 {{.N}}"), 0o644)
		cloneFunc = func(repo, ref, cacheDir string) (string, error) { return repoDir, nil }
		cfg := &config.DuckConf{Version: 1, Settings: &config.Settings{TrackCommitHash: true, AutoUpdateOnChange: true}, Targets: map[string]config.Target{"t": {Template: config.Template{Repo: "stub", Ref: "main", Path: "t.tpl"}, Variables: map[string]config.VarValue{"N": config.NewLiteralVar("w")}}}}
		h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		call := 0
		getRemoteCommitFunc = func(repo, ref string) (string, error) {
			if call == 0 {
				return h1, nil
			}
			return h2, nil
		}
		getCurrentCommitFunc = func(workdir string) (string, error) {
			if call == 0 {
				return h1, nil
			}
			return h2, nil
		}
		res1, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("first prepare: %v", err)
		}
		remoteDir := filepath.Join(".duck", "objects", "remote", res1.RemoteKey)
		stale := filepath.Join(remoteDir, "stale.tmp")
		os.WriteFile(stale, []byte("stale"), 0o644)
		if _, err := os.Stat(stale); err != nil {
			t.Fatalf("stale pre: %v", err)
		}
		os.WriteFile(tpl, []byte("v2 {{.N}}"), 0o644)
		call = 1
		res2, err := prepareAndRenderTemplate("t", cfg.Targets["t"], cfg, false, &config.SecurityConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("second prepare: %v", err)
		}
		if res1.RemoteKey != res2.RemoteKey {
			t.Fatalf("remote key changed")
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Fatalf("stale still present")
		}
		if nh, _ := readCommitHashMetadata(remoteDir); nh != h2 {
			t.Fatalf("hash not updated")
		}
	})
}
