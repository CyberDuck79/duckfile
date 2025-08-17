package git

import (
	"testing"
)

// TestIsCommitHash verifies that the IsCommitHash function correctly identifies commit hashes.
func TestIsCommitHash(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected bool
	}{
		{
			name:     "valid commit hash",
			ref:      "a1b2c3d4e5f6789012345678901234567890abcd",
			expected: true,
		},
		{
			name:     "valid commit hash uppercase",
			ref:      "A1B2C3D4E5F6789012345678901234567890ABCD",
			expected: true,
		},
		{
			name:     "valid commit hash mixed case",
			ref:      "A1b2C3d4E5f6789012345678901234567890AbCd",
			expected: true,
		},
		{
			name:     "branch name",
			ref:      "main",
			expected: false,
		},
		{
			name:     "tag name",
			ref:      "v1.0.0",
			expected: false,
		},
		{
			name:     "short hash (7 chars)",
			ref:      "a1b2c3d",
			expected: false,
		},
		{
			name:     "too long hash (41 chars)",
			ref:      "a1b2c3d4e5f6789012345678901234567890abcde",
			expected: false,
		},
		{
			name:     "invalid characters",
			ref:      "g1b2c3d4e5f6789012345678901234567890abcd",
			expected: false,
		},
		{
			name:     "empty string",
			ref:      "",
			expected: false,
		},
		{
			name:     "contains hyphen",
			ref:      "a1b2c3d4-5f6789012345678901234567890abcd",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommitHash(tt.ref)
			if result != tt.expected {
				t.Errorf("IsCommitHash(%q) = %v, want %v", tt.ref, result, tt.expected)
			}
		})
	}
}

// TestIsValidCommitHash tests the internal helper function
func TestIsValidCommitHash(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected bool
	}{
		{
			name:     "valid 40-char hex",
			hash:     "abcdef1234567890abcdef1234567890abcdef12",
			expected: true,
		},
		{
			name:     "all zeros",
			hash:     "0000000000000000000000000000000000000000",
			expected: true,
		},
		{
			name:     "all f's",
			hash:     "ffffffffffffffffffffffffffffffffffffffff",
			expected: true,
		},
		{
			name:     "too short",
			hash:     "abcdef123456789",
			expected: false,
		},
		{
			name:     "contains invalid char",
			hash:     "abcdefg234567890abcdef1234567890abcdef12",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidCommitHash(tt.hash)
			if result != tt.expected {
				t.Errorf("isValidCommitHash(%q) = %v, want %v", tt.hash, result, tt.expected)
			}
		})
	}
}

// TestGetCurrentCommitHashErrors tests error handling in GetCurrentCommitHash
func TestGetCurrentCommitHashErrors(t *testing.T) {
	tests := []struct {
		name        string
		workdir     string
		expectError bool
		description string
	}{
		{
			name:        "nonexistent directory",
			workdir:     "/nonexistent/directory",
			expectError: true,
			description: "should fail with nonexistent directory",
		},
		{
			name:        "empty directory (not a git repo)",
			workdir:     t.TempDir(),
			expectError: true,
			description: "should fail in non-git directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetCurrentCommitHash(tt.workdir)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none - %s", tt.description)
			} else if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v - %s", err, tt.description)
			}
		})
	}
}

// TestGetRemoteCommitHashErrors tests error handling in GetRemoteCommitHash
func TestGetRemoteCommitHashErrors(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		ref         string
		expectError bool
		description string
	}{
		{
			name:        "nonexistent repository",
			repo:        "https://github.com/nonexistent/nonexistent.git",
			ref:         "main",
			expectError: true,
			description: "should fail with nonexistent repository",
		},
		{
			name:        "nonexistent ref",
			repo:        "https://github.com/golang/example.git",
			ref:         "nonexistent-branch",
			expectError: true,
			description: "should fail with nonexistent ref",
		},
		{
			name:        "invalid repo URL",
			repo:        "not-a-valid-url",
			ref:         "main",
			expectError: true,
			description: "should fail with invalid repository URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetRemoteCommitHash(tt.repo, tt.ref)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none - %s", tt.description)
			} else if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v - %s", err, tt.description)
			}
		})
	}
}

// TestIsCommitHashEdgeCases tests edge cases for commit hash validation
func TestIsCommitHashEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		ref         string
		expected    bool
		description string
	}{
		{
			name:        "empty string",
			ref:         "",
			expected:    false,
			description: "empty string should not be valid commit hash",
		},
		{
			name:        "short hash",
			ref:         "a1b2c3d",
			expected:    false,
			description: "short hashes are not full commit hashes",
		},
		{
			name:        "too long",
			ref:         "a1b2c3d4e5f6789012345678901234567890abcdef",
			expected:    false,
			description: "longer than 40 characters should not be valid",
		},
		{
			name:        "contains invalid characters",
			ref:         "a1b2c3d4e5f6789012345678901234567890abcg",
			expected:    false,
			description: "should not contain characters outside hex range",
		},
		{
			name:        "exactly 40 characters but not hex",
			ref:         "this-is-exactly-40-characters-but-not-hex",
			expected:    false,
			description: "must be hexadecimal characters",
		},
		{
			name:        "all zeros (null hash)",
			ref:         "0000000000000000000000000000000000000000",
			expected:    true,
			description: "null hash is technically valid format",
		},
		{
			name:        "all fs",
			ref:         "ffffffffffffffffffffffffffffffffffffffff",
			expected:    true,
			description: "all f's should be valid hex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommitHash(tt.ref)
			if result != tt.expected {
				t.Errorf("IsCommitHash(%q) = %v, want %v - %s", tt.ref, result, tt.expected, tt.description)
			}
		})
	}
}
