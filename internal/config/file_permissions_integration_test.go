//nolint:errcheck
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFilePermissionIntegration(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("File permission validation not supported on Windows")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name              string
		configContent     string
		filePermissions   os.FileMode
		expectLoadSuccess bool
		expectError       string
	}{
		{
			name: "config without enforcement should load successfully",
			configContent: `
version: 1
allowedHosts:
  - "*.github.com"
`,
			filePermissions:   0644,
			expectLoadSuccess: true,
		},
		{
			name: "config with enforcement disabled should load successfully",
			configContent: `
version: 1
allowedHosts:
  - "*.github.com"
enforcement:
  enforceFilePermissions: false
filePermissions:
  enforceOwnership: true
  enforceReadOnly: true
`,
			filePermissions:   0666, // Bad permissions but enforcement is disabled
			expectLoadSuccess: true,
		},
		{
			name: "config with enforcement enabled and good permissions should load successfully",
			configContent: `
version: 1
allowedHosts:
  - "*.github.com"
enforcement:
  enforceFilePermissions: true
filePermissions:
  enforceOwnership: false
  enforceReadOnly: false
  allowGroupWrite: true
  requireSecureDirectories: false
`,
			filePermissions:   0664,
			expectLoadSuccess: true,
		},
		{
			name: "config with enforcement enabled and bad permissions should fail",
			configContent: `
version: 1
allowedHosts:
  - "*.github.com"
enforcement:
  enforceFilePermissions: true
filePermissions:
  enforceOwnership: false
  enforceReadOnly: false
  allowGroupWrite: false
  requireSecureDirectories: false
`,
			filePermissions:   0666, // World writable - should fail
			expectLoadSuccess: false,
			expectError:       "file permission validation failed",
		},
		{
			name: "config with read-only enforcement and writable file should fail",
			configContent: `
version: 1
allowedHosts:
  - "*.github.com"
enforcement:
  enforceFilePermissions: true
filePermissions:
  enforceOwnership: false
  enforceReadOnly: true
  allowGroupWrite: false
  requireSecureDirectories: false
`,
			filePermissions:   0644, // Owner writable but read-only required
			expectLoadSuccess: false,
			expectError:       "owner has write permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config file
			configFile := filepath.Join(tmpDir, "test-security.yaml")
			if err := os.WriteFile(configFile, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			// Explicitly set the desired permissions (overriding umask)
			if err := os.Chmod(configFile, tt.filePermissions); err != nil {
				t.Fatalf("Failed to set file permissions: %v", err)
			}

			// Attempt to load the config
			config, err := LoadSecurityConfigFromFile(configFile)

			if tt.expectLoadSuccess {
				if err != nil {
					t.Errorf("Expected successful load, but got error: %v", err)
				}
				if config == nil {
					t.Error("Expected config to be loaded, but got nil")
				}
			} else {
				if err == nil {
					t.Error("Expected load to fail, but it succeeded")
				} else if tt.expectError != "" && !contains(err.Error(), tt.expectError) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectError, err)
				}
			}

			// Clean up the test file
			os.Remove(configFile)
		})
	}
}

func TestFilePermissionValidationSimple(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("File permission validation not supported on Windows")
	}

	tmpDir := t.TempDir()

	// Create a config with permission enforcement
	configContent := `
version: 1
allowedHosts:
  - "*.secure.com"
enforcement:
  enforceFilePermissions: true
filePermissions:
  enforceOwnership: false
  enforceReadOnly: false
  allowGroupWrite: false
  requireSecureDirectories: false
`

	configFile := filepath.Join(tmpDir, "security.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Make the config file world-writable (bad permissions)
	if err := os.Chmod(configFile, 0666); err != nil {
		t.Fatalf("Failed to set bad permissions: %v", err)
	}

	// Loading should fail due to permission issues
	_, err := LoadSecurityConfigFromFile(configFile)
	if err == nil {
		t.Error("Expected load to fail due to permission issues")
	}

	if !contains(err.Error(), "file permission validation failed") {
		t.Errorf("Expected permission validation error, got: %v", err)
	}

	// Fix permissions and try again
	if err := os.Chmod(configFile, 0644); err != nil {
		t.Fatalf("Failed to fix permissions: %v", err)
	}

	config, err := LoadSecurityConfigFromFile(configFile)
	if err != nil {
		t.Errorf("Expected load to succeed after fixing permissions, got: %v", err)
	}
	if config == nil {
		t.Error("Expected config to be loaded")
	}
}
