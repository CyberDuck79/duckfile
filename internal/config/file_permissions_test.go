//nolint:errcheck
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetermineSecurityFileType(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected SecurityFileType
	}{
		{
			name:     "system config in /etc",
			path:     "/etc/duckfile/security.yaml",
			expected: SecurityFileTypeSystem,
		},
		{
			name:     "system config in /usr",
			path:     "/usr/local/etc/duckfile/security.yaml",
			expected: SecurityFileTypeSystem,
		},
		{
			name:     "user config in home directory",
			path:     filepath.Join(os.Getenv("HOME"), ".duckfile/security.yaml"),
			expected: SecurityFileTypeUser,
		},
		{
			name:     "project config with go.mod",
			path:     "/tmp/myproject/security.yaml",
			expected: SecurityFileTypeProject,
		},
		{
			name:     "project config in src directory",
			path:     "/home/user/projects/myapp/src/security.yaml",
			expected: SecurityFileTypeProject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip home directory tests if HOME is not set (CI environments)
			if tt.expected == SecurityFileTypeUser && os.Getenv("HOME") == "" {
				t.Skip("HOME environment variable not set")
			}

			result := DetermineSecurityFileType(tt.path)
			if result != tt.expected {
				t.Errorf("DetermineSecurityFileType(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidateFilePermissions(t *testing.T) {
	// Skip on Windows as permission validation is Unix-specific
	if runtime.GOOS == "windows" {
		t.Skip("File permission validation not supported on Windows")
	}

	// Create a temporary file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-security.yaml")

	// Create test file with content
	content := `# Test security config
allowedHosts:
  - "*.example.com"
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		policy      *FilePermissionPolicy
		fileMode    os.FileMode
		expectValid bool
		expectIssue string
	}{
		{
			name:        "nil policy should be valid",
			policy:      nil,
			fileMode:    0644,
			expectValid: true,
		},
		{
			name: "permissive policy with good permissions",
			policy: &FilePermissionPolicy{
				EnforceOwnership:         false,
				EnforceReadOnly:          false,
				AllowGroupWrite:          true,
				RequireSecureDirectories: false,
			},
			fileMode:    0664,
			expectValid: true,
		},
		{
			name: "read-only policy with writable file",
			policy: &FilePermissionPolicy{
				EnforceOwnership:         false,
				EnforceReadOnly:          true,
				AllowGroupWrite:          false,
				RequireSecureDirectories: false,
			},
			fileMode:    0644,
			expectValid: false,
			expectIssue: "owner has write permission",
		},
		{
			name: "no group write policy with group writable file",
			policy: &FilePermissionPolicy{
				EnforceOwnership:         false,
				EnforceReadOnly:          false,
				AllowGroupWrite:          false,
				RequireSecureDirectories: false,
			},
			fileMode:    0664,
			expectValid: false,
			expectIssue: "group write permission not allowed",
		},
		{
			name: "world writable file should fail",
			policy: &FilePermissionPolicy{
				EnforceOwnership:         false,
				EnforceReadOnly:          false,
				AllowGroupWrite:          true,
				RequireSecureDirectories: false,
			},
			fileMode:    0666,
			expectValid: false,
			expectIssue: "others should not have write permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the file permissions for this test
			if err := os.Chmod(testFile, tt.fileMode); err != nil {
				t.Fatalf("Failed to set file permissions: %v", err)
			}

			configFile := &SecurityConfigFile{
				Path:   testFile,
				Type:   SecurityFileTypeProject,
				Exists: true,
			}

			result, err := ValidateFilePermissions(configFile, tt.policy)
			if err != nil {
				t.Fatalf("ValidateFilePermissions failed: %v", err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
			}

			if !tt.expectValid && tt.expectIssue != "" {
				found := false
				for _, issue := range result.Issues {
					if len(issue) > 0 && len(tt.expectIssue) > 0 {
						// Case-insensitive substring check
						if contains(issue, tt.expectIssue) {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Expected issue containing '%s', got issues: %v", tt.expectIssue, result.Issues)
				}
			}
		})
	}
}

// Helper function for case-insensitive substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateSecurityConfigPermissions(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("File permission validation not supported on Windows")
	}

	tmpDir := t.TempDir()

	// Create multiple test files
	files := []struct {
		name string
		mode os.FileMode
	}{
		{"system.yaml", 0644},
		{"user.yaml", 0600},
		{"project.yaml", 0664},
		{"bad.yaml", 0666},
	}

	var configFiles []*SecurityConfigFile
	for _, file := range files {
		path := filepath.Join(tmpDir, file.name)
		content := "allowedHosts: ['*.example.com']"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file.name, err)
		}

		// Explicitly set the desired permissions (overriding umask)
		if err := os.Chmod(path, file.mode); err != nil {
			t.Fatalf("Failed to set permissions for %s: %v", file.name, err)
		}

		// Verify the file was created with correct permissions
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat file %s: %v", file.name, err)
		}
		actualMode := info.Mode() & os.ModePerm
		t.Logf("File %s: expected mode %o, actual mode %o", file.name, file.mode, actualMode)

		configFiles = append(configFiles, &SecurityConfigFile{
			Path:   path,
			Type:   SecurityFileTypeProject,
			Exists: true,
		})
	}

	policy := &FilePermissionPolicy{
		EnforceOwnership:         false,
		EnforceReadOnly:          false,
		AllowGroupWrite:          true,
		RequireSecureDirectories: false,
	}

	results, err := ValidateSecurityConfigPermissions(configFiles, policy)
	if err != nil {
		t.Fatalf("ValidateSecurityConfigPermissions failed: %v", err)
	}

	if len(results) != len(configFiles) {
		t.Errorf("Expected %d results, got %d", len(configFiles), len(results))
	}

	// Debug: print all results
	for i, result := range results {
		t.Logf("File %d (%s): Valid=%v, Issues=%v", i, result.Path, result.Valid, result.Issues)
	}

	// The last file (bad.yaml with 0666) should have validation issues
	if len(results) > 0 {
		lastResult := results[len(results)-1]
		if lastResult.Valid {
			t.Errorf("Expected bad.yaml (0666) to have validation issues, but got valid=true. Issues: %v", lastResult.Issues)
		}
		if len(lastResult.Issues) == 0 {
			t.Error("Expected bad.yaml to have issues but got none")
		}
	}
}

func TestFixFilePermissions(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("File permission fixing not supported on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "fix-test.yaml")

	// Create test file with bad permissions
	content := "allowedHosts: ['*.example.com']"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Explicitly set bad permissions (world writable)
	if err := os.Chmod(testFile, 0666); err != nil {
		t.Fatalf("Failed to set bad permissions: %v", err)
	}

	configFile := &SecurityConfigFile{
		Path:   testFile,
		Type:   SecurityFileTypeUser,
		Exists: true,
	}

	policy := &FilePermissionPolicy{
		EnforceOwnership:         false,
		EnforceReadOnly:          false,
		AllowGroupWrite:          false,
		RequireSecureDirectories: false,
	}

	// First validate to confirm it has issues
	result, err := ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("ValidateFilePermissions failed: %v", err)
	}

	if result.Valid {
		t.Fatal("Expected test file to have permission issues")
	}

	// Test dry run first
	err = FixFilePermissions(result, policy, true)
	if err != nil {
		t.Errorf("FixFilePermissions dry run failed: %v", err)
	}

	// Permissions should still be bad after dry run
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file after dry run: %v", err)
	}
	if info.Mode()&os.ModePerm != 0666 {
		t.Error("File permissions changed during dry run")
	}

	// Now do actual fix
	err = FixFilePermissions(result, policy, false)
	if err != nil {
		t.Errorf("FixFilePermissions failed: %v", err)
	}

	// Verify permissions were fixed
	info, err = os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file after fix: %v", err)
	}

	expectedMode := os.FileMode(0644)
	if info.Mode()&os.ModePerm != expectedMode {
		t.Errorf("Expected permissions %o, got %o", expectedMode, info.Mode()&os.ModePerm)
	}

	// Re-validate to confirm it's now valid
	result2, err := ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("ValidateFilePermissions after fix failed: %v", err)
	}

	if !result2.Valid {
		t.Errorf("File should be valid after fix, but has issues: %v", result2.Issues)
	}
}

func TestParentDirectoryValidation(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Directory permission validation not supported on Windows")
	}

	// Create a nested directory structure in a user path (not system path)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get user home directory")
	}

	testDir := filepath.Join(homeDir, "duckfile-test", "level1", "level2")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}
	defer os.RemoveAll(filepath.Join(homeDir, "duckfile-test"))

	testFile := filepath.Join(testDir, "security.yaml")
	content := "allowedHosts: ['*.example.com']"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	policy := &FilePermissionPolicy{
		EnforceOwnership:         false,
		EnforceReadOnly:          false,
		AllowGroupWrite:          false,
		RequireSecureDirectories: true,
	}

	configFile := &SecurityConfigFile{
		Path:   testFile,
		Type:   SecurityFileTypeUser,
		Exists: true,
	}

	result, err := ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("ValidateFilePermissions failed: %v", err)
	}

	// Should be valid with secure directory permissions
	if !result.ParentDirSecure {
		t.Errorf("Parent directories should be secure, issues: %v", result.ParentDirIssues)
	}

	// Now make a parent directory world-writable
	level1Dir := filepath.Join(homeDir, "duckfile-test", "level1")
	if err := os.Chmod(level1Dir, 0777); err != nil {
		t.Fatalf("Failed to make directory world-writable: %v", err)
	}

	// Re-validate - should now fail
	result2, err := ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("ValidateFilePermissions failed: %v", err)
	}

	if result2.ParentDirSecure {
		t.Error("Parent directories should be insecure after making one world-writable")
	}

	if len(result2.ParentDirIssues) == 0 {
		t.Error("Expected parent directory issues but got none")
	}
}
