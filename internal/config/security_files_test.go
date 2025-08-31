package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSecurityConfigs(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test directory structure
	etcDir := filepath.Join(tempDir, "etc", "duckfile")
	homeDir := filepath.Join(tempDir, "home", "user")
	duckfileDir := filepath.Join(homeDir, ".duckfile")
	configDir := filepath.Join(homeDir, ".config", "duckfile")
	projectDir := filepath.Join(tempDir, "project", ".duckfile")

	// Create directories
	for _, dir := range []string{etcDir, duckfileDir, configDir, projectDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		filepath.Join(etcDir, "security.yaml"):     "version: 1\nallowedHosts: [github.com]",
		filepath.Join(duckfileDir, "security.yml"): "version: 1\nallowedHosts: [gitlab.com]",
		filepath.Join(configDir, "security.yaml"):  "version: 1\nallowedHosts: [bitbucket.org]",
		filepath.Join(projectDir, "security.yaml"): "version: 1\nallowedHosts: [internal.com]",
	}

	for path, content := range testFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Create signature files for some configs
	sigFiles := []string{
		filepath.Join(etcDir, "security.yaml.sig"),
		filepath.Join(duckfileDir, "security.yml.sig"),
	}

	for _, sigPath := range sigFiles {
		if err := os.WriteFile(sigPath, []byte("dummy-signature"), 0644); err != nil {
			t.Fatalf("Failed to create signature file %s: %v", sigPath, err)
		}
	}

	// Mock the discovery paths by temporarily changing directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to project directory for testing
	if err := os.Chdir(filepath.Join(tempDir, "project")); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	// Test the discovery function with limited scope (only project files for now)
	// Since we can't easily mock system and user paths, we'll test project discovery
	configs, err := DiscoverSecurityConfigs()
	if err != nil {
		t.Fatalf("DiscoverSecurityConfigs() failed: %v", err)
	}

	// We should find at least the project config
	found := false
	for _, cfg := range configs {
		if cfg.Type == SecurityFileTypeProject && cfg.Exists && cfg.Readable {
			found = true
			if cfg.Path != "./.duckfile/security.yaml" {
				t.Errorf("Expected project config path './.duckfile/security.yaml', got %s", cfg.Path)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find project security config")
	}
}

func TestCheckSecurityConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		setupFunc      func() string
		fileType       SecurityFileType
		expectExists   bool
		expectReadable bool
		expectSigFile  bool
	}{
		{
			name: "existing readable file",
			setupFunc: func() string {
				path := filepath.Join(tempDir, "existing.yaml")
				os.WriteFile(path, []byte("version: 1"), 0644)
				return path
			},
			fileType:       SecurityFileTypeUser,
			expectExists:   true,
			expectReadable: true,
			expectSigFile:  false,
		},
		{
			name: "existing file with signature",
			setupFunc: func() string {
				path := filepath.Join(tempDir, "signed.yaml")
				os.WriteFile(path, []byte("version: 1"), 0644)
				os.WriteFile(path+".sig", []byte("signature"), 0644)
				return path
			},
			fileType:       SecurityFileTypeSystem,
			expectExists:   true,
			expectReadable: true,
			expectSigFile:  true,
		},
		{
			name: "non-existent file",
			setupFunc: func() string {
				return filepath.Join(tempDir, "nonexistent.yaml")
			},
			fileType:       SecurityFileTypeProject,
			expectExists:   false,
			expectReadable: false,
			expectSigFile:  false,
		},
		{
			name: "directory instead of file",
			setupFunc: func() string {
				path := filepath.Join(tempDir, "directory")
				os.MkdirAll(path, 0755)
				return path
			},
			fileType:       SecurityFileTypeUser,
			expectExists:   false,
			expectReadable: false,
			expectSigFile:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFunc()

			config := checkSecurityConfigFile(path, tt.fileType)

			if tt.expectExists {
				if config == nil {
					t.Fatal("Expected config to be returned for existing file")
				}

				if config.Path != path {
					t.Errorf("Expected path %s, got %s", path, config.Path)
				}

				if config.Type != tt.fileType {
					t.Errorf("Expected type %v, got %v", tt.fileType, config.Type)
				}

				if config.Exists != tt.expectExists {
					t.Errorf("Expected Exists %v, got %v", tt.expectExists, config.Exists)
				}

				if config.Readable != tt.expectReadable {
					t.Errorf("Expected Readable %v, got %v", tt.expectReadable, config.Readable)
				}

				if config.HasSigFile != tt.expectSigFile {
					t.Errorf("Expected HasSigFile %v, got %v", tt.expectSigFile, config.HasSigFile)
				}
			} else {
				if config != nil {
					t.Error("Expected nil config for non-existent file")
				}
			}
		})
	}
}

func TestLoadSecurityConfigFromFile(t *testing.T) {
	// Create a temporary file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "security.yaml")

	content := `version: 1
allowedHosts:
  - github.com
  - gitlab.com
deniedHosts:
  - malicious.com
strictMode: true
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading (should fail in Phase 1 as it's not implemented yet)
	config, err := LoadSecurityConfigFromFile(configPath)
	if err == nil {
		t.Error("Expected error for unimplemented file loading")
	}
	if config != nil {
		t.Error("Expected nil config for unimplemented file loading")
	}

	expectedErrMsg := "file-based security configuration loading not yet implemented"
	if err != nil && !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedErrMsg, err)
	}
}

func TestBuildSecurityConfigWithFiles(t *testing.T) {
	// Test that the function works even when files are discovered
	config, err := BuildSecurityConfigWithFiles([]string{"github.com"}, []string{"bad.com"}, true)
	if err != nil {
		t.Fatalf("BuildSecurityConfigWithFiles() failed: %v", err)
	}

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Should fall back to CLI behavior for now
	if config.Source != "cli" {
		t.Errorf("Expected source 'cli', got %s", config.Source)
	}

	if len(config.AllowedHosts) != 1 || config.AllowedHosts[0] != "github.com" {
		t.Errorf("Expected allowed hosts [github.com], got %v", config.AllowedHosts)
	}

	if len(config.DeniedHosts) != 1 || config.DeniedHosts[0] != "bad.com" {
		t.Errorf("Expected denied hosts [bad.com], got %v", config.DeniedHosts)
	}

	if !config.StrictMode {
		t.Error("Expected strict mode to be true")
	}

	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}
}
