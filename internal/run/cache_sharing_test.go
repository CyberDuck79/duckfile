//nolint:errcheck
package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestCacheSharingMultipleTargets verifies that multiple targets using the same remote
// share the remote cache but have separate template and rendered caches.
func TestCacheSharingMultipleTargets(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		// Create test repository with multiple templates
		templateDir := "template_source"
		os.MkdirAll(templateDir, 0o755)
		os.WriteFile(filepath.Join(templateDir, "makefile.tpl"), []byte("build: {{.TARGET}}"), 0o644)
		os.WriteFile(filepath.Join(templateDir, "docker.tpl"), []byte("FROM {{.IMAGE}}"), 0o644)

		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			// Copy both template files
			data1, _ := os.ReadFile(filepath.Join(templateDir, "makefile.tpl"))
			os.WriteFile(filepath.Join(repoDir, "makefile.tpl"), data1, 0o644)
			data2, _ := os.ReadFile(filepath.Join(templateDir, "docker.tpl"))
			os.WriteFile(filepath.Join(repoDir, "docker.tpl"), data2, 0o644)
			return repoDir, nil
		}

		// Define two targets that use the same remote but different templates
		cfg := &config.DuckConf{
			Version: 1,
			Targets: map[string]config.Target{
				"build": {
					Template: config.Template{
						Repo: "https://github.com/test/repo.git",
						Ref:  "main",
						Path: "makefile.tpl",
					},
					Variables: map[string]config.VarValue{
						"TARGET": config.NewLiteralVar("app"),
					},
				},
				"docker": {
					Template: config.Template{
						Repo: "https://github.com/test/repo.git", // Same repo
						Ref:  "main",                             // Same ref
						Path: "docker.tpl",                       // Different template
					},
					Variables: map[string]config.VarValue{
						"IMAGE": config.NewLiteralVar("alpine"),
					},
				},
			},
		}

		// Sync both targets
		if err := Sync(cfg, "", false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Fatalf("sync failed: %v", err)
		}

		// Verify remote cache is shared
		buildPaths, _ := computeTemplatePaths("build", cfg.Targets["build"], map[string]any{"TARGET": "app"})
		dockerPaths, _ := computeTemplatePaths("docker", cfg.Targets["docker"], map[string]any{"IMAGE": "alpine"})

		// Remote keys should be the same (shared cache)
		if buildPaths.remoteKey != dockerPaths.remoteKey {
			t.Errorf("Expected same remote key for shared repo, got build=%s, docker=%s",
				buildPaths.remoteKey, dockerPaths.remoteKey)
		}

		// Template keys should be different (different paths)
		if buildPaths.templateKey == dockerPaths.templateKey {
			t.Errorf("Expected different template keys for different paths, got both=%s",
				buildPaths.templateKey)
		}

		// Rendered keys should be different (different variables)
		if buildPaths.renderedKey == dockerPaths.renderedKey {
			t.Errorf("Expected different rendered keys for different variables, got both=%s",
				buildPaths.renderedKey)
		}

		// Verify remote cache directory exists and is shared
		remoteDir := buildPaths.remoteDir
		if _, err := os.Stat(remoteDir); err != nil {
			t.Errorf("Remote cache directory should exist: %v", err)
		}

		// Verify both template cache directories exist
		if _, err := os.Stat(buildPaths.templateDir); err != nil {
			t.Errorf("Build template cache should exist: %v", err)
		}
		if _, err := os.Stat(dockerPaths.templateDir); err != nil {
			t.Errorf("Docker template cache should exist: %v", err)
		}

		// Verify template contents are correct
		buildContent, _ := os.ReadFile(buildPaths.remoteTemplateFile)
		if string(buildContent) != "build: {{.TARGET}}" {
			t.Errorf("Expected build template content 'build: {{.TARGET}}', got %s", string(buildContent))
		}

		dockerContent, _ := os.ReadFile(dockerPaths.remoteTemplateFile)
		if string(dockerContent) != "FROM {{.IMAGE}}" {
			t.Errorf("Expected docker template content 'FROM {{.IMAGE}}', got %s", string(dockerContent))
		}
	})
}

// TestCacheSharingWithDifferentVariables verifies that targets with same template
// but different variables share remote and template caches but have different rendered caches.
func TestCacheSharingWithDifferentVariables(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		templateDir := "template_source"
		os.MkdirAll(templateDir, 0o755)
		os.WriteFile(filepath.Join(templateDir, "app.tpl"), []byte("app: {{.NAME}}-{{.ENV}}"), 0o644)

		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "app.tpl"))
			os.WriteFile(filepath.Join(repoDir, "app.tpl"), data, 0o644)
			return repoDir, nil
		}

		cfg := &config.DuckConf{
			Version: 1,
			Targets: map[string]config.Target{
				"dev": {
					Template: config.Template{
						Repo: "https://github.com/test/repo.git",
						Ref:  "main",
						Path: "app.tpl",
					},
					Variables: map[string]config.VarValue{
						"NAME": config.NewLiteralVar("myapp"),
						"ENV":  config.NewLiteralVar("development"),
					},
				},
				"prod": {
					Template: config.Template{
						Repo: "https://github.com/test/repo.git", // Same repo
						Ref:  "main",                             // Same ref
						Path: "app.tpl",                          // Same template
					},
					Variables: map[string]config.VarValue{
						"NAME": config.NewLiteralVar("myapp"),
						"ENV":  config.NewLiteralVar("production"), // Different variables
					},
				},
			},
		}

		// Sync both targets
		if err := Sync(cfg, "", false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Fatalf("sync failed: %v", err)
		}

		devPaths, _ := computeTemplatePaths("dev", cfg.Targets["dev"], map[string]any{"NAME": "myapp", "ENV": "development"})
		prodPaths, _ := computeTemplatePaths("prod", cfg.Targets["prod"], map[string]any{"NAME": "myapp", "ENV": "production"})

		// Remote and template keys should be the same (shared)
		if devPaths.remoteKey != prodPaths.remoteKey {
			t.Errorf("Expected same remote key, got dev=%s, prod=%s",
				devPaths.remoteKey, prodPaths.remoteKey)
		}
		if devPaths.templateKey != prodPaths.templateKey {
			t.Errorf("Expected same template key, got dev=%s, prod=%s",
				devPaths.templateKey, prodPaths.templateKey)
		}

		// Rendered keys should be different (different variables)
		if devPaths.renderedKey == prodPaths.renderedKey {
			t.Errorf("Expected different rendered keys for different variables, got both=%s",
				devPaths.renderedKey)
		}

		// Verify only one remote and template cache directory exists
		if _, err := os.Stat(devPaths.remoteDir); err != nil {
			t.Errorf("Shared remote cache should exist: %v", err)
		}
		if _, err := os.Stat(devPaths.templateDir); err != nil {
			t.Errorf("Shared template cache should exist: %v", err)
		}

		// Verify different rendered cache directories exist
		if _, err := os.Stat(devPaths.renderedDir); err != nil {
			t.Errorf("Dev rendered cache should exist: %v", err)
		}
		if _, err := os.Stat(prodPaths.renderedDir); err != nil {
			t.Errorf("Prod rendered cache should exist: %v", err)
		}
	})
}

// TestCacheInvalidationPreservesSharedCache verifies that invalidating cache for one target
// doesn't affect cache for other targets using the same remote.
func TestCacheInvalidationPreservesSharedCache(t *testing.T) {
	withTempWD(t, func() {
		restore := resetSeams(t)
		defer restore()

		templateDir := "template_source"
		os.MkdirAll(templateDir, 0o755)
		os.WriteFile(filepath.Join(templateDir, "shared.tpl"), []byte("shared: {{.VAR}}"), 0o644)

		cloneFunc = func(repo, ref, cacheDir string, submodules bool) (string, error) {
			repoDir := filepath.Join(cacheDir, "repo")
			os.MkdirAll(repoDir, 0o755)
			data, _ := os.ReadFile(filepath.Join(templateDir, "shared.tpl"))
			os.WriteFile(filepath.Join(repoDir, "shared.tpl"), data, 0o644)
			return repoDir, nil
		}

		cfg := &config.DuckConf{
			Version: 1,
			Targets: map[string]config.Target{
				"target1": {
					Template: config.Template{
						Repo: "https://github.com/test/repo.git",
						Ref:  "main",
						Path: "shared.tpl",
					},
					Variables: map[string]config.VarValue{
						"VAR": config.NewLiteralVar("value1"),
					},
				},
				"target2": {
					Template: config.Template{
						Repo: "https://github.com/test/repo.git", // Same remote
						Ref:  "main",
						Path: "shared.tpl", // Same template
					},
					Variables: map[string]config.VarValue{
						"VAR": config.NewLiteralVar("value2"),
					},
				},
			},
		}

		// Sync both targets initially
		if err := Sync(cfg, "", false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Fatalf("initial sync failed: %v", err)
		}

		target1Paths, _ := computeTemplatePaths("target1", cfg.Targets["target1"], map[string]any{"VAR": "value1"})
		target2Paths, _ := computeTemplatePaths("target2", cfg.Targets["target2"], map[string]any{"VAR": "value2"})

		// Verify shared caches exist
		if _, err := os.Stat(target1Paths.remoteDir); err != nil {
			t.Fatalf("Remote cache should exist before clean: %v", err)
		}
		if _, err := os.Stat(target1Paths.templateDir); err != nil {
			t.Fatalf("Template cache should exist before clean: %v", err)
		}

		// Clean target1 only
		if err := Clean(cfg, "target1"); err != nil {
			t.Fatalf("clean target1 failed: %v", err)
		}

		// Verify remote cache still exists (shared with target2) but template cache is removed
		if _, err := os.Stat(target1Paths.remoteDir); err != nil {
			t.Errorf("Remote cache should still exist after cleaning target1 (shared with target2): %v", err)
		}
		if _, err := os.Stat(target1Paths.templateDir); err == nil {
			t.Errorf("Template cache should be removed after cleaning target1")
		}

		// Verify target2's rendered cache still exists
		if _, err := os.Stat(target2Paths.renderedDir); err != nil {
			t.Errorf("Target2 rendered cache should still exist: %v", err)
		}

		// Verify target1 can still sync (reusing shared remote cache)
		if err := Sync(cfg, "target1", false, &config.SecurityConfig{}, nil, nil); err != nil {
			t.Errorf("target1 should be able to sync after clean (reusing shared cache): %v", err)
		}
	})
}
