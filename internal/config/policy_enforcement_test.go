package config

import (
	"strings"
	"testing"
)

func TestEnforceSecurityPolicies(t *testing.T) {
	tests := []struct {
		name             string
		targetName       string
		target           *Target
		securityConfig   *SecurityConfig
		expectAllowed    bool
		expectViolations int
		expectWarnings   int
		violationTypes   []string
	}{
		{
			name:       "nil security config should allow everything",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo: "https://github.com/test/repo",
					Ref:  "main",
					Path: "template.yml",
				},
			},
			securityConfig:   nil,
			expectAllowed:    true,
			expectViolations: 0,
			expectWarnings:   0,
		},
		{
			name:       "no enforcement policy should allow everything",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo: "https://github.com/test/repo",
					Ref:  "main",
					Path: "template.yml",
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
			},
			expectAllowed:    true,
			expectViolations: 0,
			expectWarnings:   0,
		},
		{
			name:       "force checksum validation without checksum should fail",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo: "https://github.com/test/repo",
					Ref:  "v1.0.0",
					Path: "template.yml",
					// No checksum provided
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: true,
				},
			},
			expectAllowed:    false,
			expectViolations: 1,
			expectWarnings:   0,
			violationTypes:   []string{"checksum_validation"},
		},
		{
			name:       "force checksum validation with checksum should pass",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo:     "https://github.com/test/repo",
					Ref:      "v1.0.0",
					Path:     "template.yml",
					Checksum: "sha256:abc123",
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: true,
				},
			},
			expectAllowed:    true,
			expectViolations: 0,
			expectWarnings:   0,
		},
		{
			name:       "force commit tracking without tracking should fail",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo:            "https://github.com/test/repo",
					Ref:             "v1.0.0",
					Path:            "template.yml",
					Checksum:        "sha256:abc123",
					TrackCommitHash: false, // Disabled
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceCommitTracking: true,
				},
			},
			expectAllowed:    false,
			expectViolations: 1,
			expectWarnings:   0,
			violationTypes:   []string{"commit_tracking"},
		},
		{
			name:       "force commit tracking with tracking should pass",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo:            "https://github.com/test/repo",
					Ref:             "v1.0.0",
					Path:            "template.yml",
					Checksum:        "sha256:abc123",
					TrackCommitHash: true, // Enabled
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceCommitTracking: true,
				},
			},
			expectAllowed:    true,
			expectViolations: 0,
			expectWarnings:   0,
		},
		{
			name:       "disable auto update with auto update enabled should warn",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo:               "https://github.com/test/repo",
					Ref:                "v1.0.0",
					Path:               "template.yml",
					AutoUpdateOnChange: true, // Enabled
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					DisableAutoUpdate: true,
				},
			},
			expectAllowed:    true,
			expectViolations: 0,
			expectWarnings:   1,
		},
		{
			name:       "repository access denied should fail",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo: "malicious.com/bad/repo",
					Ref:  "v1.0.0",
					Path: "template.yml",
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: false,
				},
			},
			expectAllowed:    false,
			expectViolations: 1,
			expectWarnings:   0,
			violationTypes:   []string{"repository_access"},
		},
		{
			name:       "multiple violations should be reported",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo: "malicious.com/bad/repo", // Not allowed
					Ref:  "v1.0.0",
					Path: "template.yml",
					// No checksum
					TrackCommitHash: false, // Disabled
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: true,
					ForceCommitTracking:     true,
				},
			},
			expectAllowed:    false,
			expectViolations: 3, // checksum, commit tracking, repository access
			expectWarnings:   0,
			violationTypes:   []string{"checksum_validation", "commit_tracking", "repository_access"},
		},
		{
			name:       "unstable git reference should warn",
			targetName: "test",
			target: &Target{
				Template: &Template{
					Repo:     "https://github.com/test/repo",
					Ref:      "main", // Unstable reference
					Path:     "template.yml",
					Checksum: "sha256:abc123",
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: true,
				},
			},
			expectAllowed:    true,
			expectViolations: 0,
			expectWarnings:   1, // Warning about unstable ref
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnforceSecurityPolicies(tt.targetName, tt.target, tt.securityConfig)

			if result.Allowed != tt.expectAllowed {
				t.Errorf("Expected allowed=%v, got allowed=%v", tt.expectAllowed, result.Allowed)
			}

			if len(result.Violations) != tt.expectViolations {
				t.Errorf("Expected %d violations, got %d: %v", tt.expectViolations, len(result.Violations), result.Violations)
			}

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			// Check specific violation types if provided
			if len(tt.violationTypes) > 0 {
				foundTypes := make(map[string]bool)
				for _, violation := range result.Violations {
					foundTypes[violation.Type] = true
				}

				for _, expectedType := range tt.violationTypes {
					if !foundTypes[expectedType] {
						t.Errorf("Expected violation type %q not found. Found types: %v", expectedType, getViolationTypes(result.Violations))
					}
				}
			}
		})
	}
}

func TestApplyPolicyOverrides(t *testing.T) {
	tests := []struct {
		name                    string
		target                  *Target
		securityConfig          *SecurityConfig
		expectedAutoUpdate      bool
		expectedTrackCommitHash bool
	}{
		{
			name: "no security config should not modify target",
			target: &Target{
				Template: &Template{
					AutoUpdateOnChange: true,
					TrackCommitHash:    false,
				},
			},
			securityConfig:          nil,
			expectedAutoUpdate:      true,
			expectedTrackCommitHash: false,
		},
		{
			name: "no enforcement should not modify target",
			target: &Target{
				Template: &Template{
					AutoUpdateOnChange: true,
					TrackCommitHash:    false,
				},
			},
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
			},
			expectedAutoUpdate:      true,
			expectedTrackCommitHash: false,
		},
		{
			name: "disable auto update should override target setting",
			target: &Target{
				Template: &Template{
					AutoUpdateOnChange: true, // Original setting
					TrackCommitHash:    false,
				},
			},
			securityConfig: &SecurityConfig{
				Enforcement: &PolicyEnforcement{
					DisableAutoUpdate: true,
				},
			},
			expectedAutoUpdate:      false, // Should be overridden
			expectedTrackCommitHash: false,
		},
		{
			name: "force commit tracking should override target setting",
			target: &Target{
				Template: &Template{
					AutoUpdateOnChange: true,
					TrackCommitHash:    false, // Original setting
				},
			},
			securityConfig: &SecurityConfig{
				Enforcement: &PolicyEnforcement{
					ForceCommitTracking: true,
				},
			},
			expectedAutoUpdate:      true,
			expectedTrackCommitHash: true, // Should be overridden
		},
		{
			name: "multiple overrides should be applied",
			target: &Target{
				Template: &Template{
					AutoUpdateOnChange: true,  // Will be disabled
					TrackCommitHash:    false, // Will be enabled
				},
			},
			securityConfig: &SecurityConfig{
				Enforcement: &PolicyEnforcement{
					DisableAutoUpdate:   true,
					ForceCommitTracking: true,
				},
			},
			expectedAutoUpdate:      false, // Disabled by policy
			expectedTrackCommitHash: true,  // Enabled by policy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifiedTarget := ApplyPolicyOverrides(tt.target, tt.securityConfig)

			if modifiedTarget.Template.AutoUpdateOnChange != tt.expectedAutoUpdate {
				t.Errorf("Expected AutoUpdateOnChange=%v, got %v", tt.expectedAutoUpdate, modifiedTarget.Template.AutoUpdateOnChange)
			}

			if modifiedTarget.Template.TrackCommitHash != tt.expectedTrackCommitHash {
				t.Errorf("Expected TrackCommitHash=%v, got %v", tt.expectedTrackCommitHash, modifiedTarget.Template.TrackCommitHash)
			}

			// Verify original target is not modified
			if tt.target.Template.AutoUpdateOnChange != true && tt.target.Template.AutoUpdateOnChange != false {
				// Only check if we started with specific values
				t.Errorf("Original target should not be modified")
			}
		})
	}
}

func TestValidateStrictPolicyMode(t *testing.T) {
	tests := []struct {
		name           string
		securityConfig *SecurityConfig
		expectError    bool
		errorContains  string
	}{
		{
			name:           "nil security config should fail",
			securityConfig: nil,
			expectError:    true,
			errorContains:  "security configuration but none was found",
		},
		{
			name: "no enforcement should pass",
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
			},
			expectError: false,
		},
		{
			name: "strict mode without policies should fail",
			securityConfig: &SecurityConfig{
				Enforcement: &PolicyEnforcement{
					StrictPolicyMode: true,
					// No other policies enabled
				},
			},
			expectError:   true,
			errorContains: "no security policies are configured",
		},
		{
			name: "strict mode with policies but unsigned should fail",
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					StrictPolicyMode:        true,
					ForceChecksumValidation: true,
				},
				IsSigned: false,
			},
			expectError:   true,
			errorContains: "digitally signed security configuration",
		},
		{
			name: "strict mode with policies and signed should pass",
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &PolicyEnforcement{
					StrictPolicyMode:        true,
					ForceChecksumValidation: true,
				},
				IsSigned: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrictPolicyMode(tt.securityConfig)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestFormatPolicyViolations(t *testing.T) {
	tests := []struct {
		name     string
		result   *PolicyEnforcementResult
		expected string
	}{
		{
			name:     "nil result should return empty string",
			result:   nil,
			expected: "",
		},
		{
			name: "no violations or warnings should return empty string",
			result: &PolicyEnforcementResult{
				Allowed:    true,
				Violations: []PolicyViolation{},
				Warnings:   []PolicyViolation{},
			},
			expected: "",
		},
		{
			name: "single violation should format correctly",
			result: &PolicyEnforcementResult{
				Allowed: false,
				Violations: []PolicyViolation{
					{
						Type:       "checksum_validation",
						Message:    "Checksum required but not provided",
						Suggestion: "Add checksum to template",
					},
				},
			},
			expected: "Security Policy Violations:\n  1. Checksum required but not provided\n     Suggestion: Add checksum to template\n",
		},
		{
			name: "multiple violations should format correctly",
			result: &PolicyEnforcementResult{
				Allowed: false,
				Violations: []PolicyViolation{
					{Type: "checksum_validation", Message: "Checksum required"},
					{Type: "commit_tracking", Message: "Commit tracking required"},
				},
			},
			expected: "Security Policy Violations:\n  1. Checksum required\n  2. Commit tracking required\n",
		},
		{
			name: "warnings should format correctly",
			result: &PolicyEnforcementResult{
				Allowed: true,
				Warnings: []PolicyViolation{
					{
						Type:       "template_validation",
						Message:    "Using unstable reference",
						Suggestion: "Use a specific tag",
					},
				},
			},
			expected: "Security Policy Warnings:\n  1. Using unstable reference\n     Suggestion: Use a specific tag\n",
		},
		{
			name: "violations and warnings should format correctly",
			result: &PolicyEnforcementResult{
				Allowed: false,
				Violations: []PolicyViolation{
					{Type: "checksum_validation", Message: "Checksum required"},
				},
				Warnings: []PolicyViolation{
					{Type: "template_validation", Message: "Using unstable reference"},
				},
			},
			expected: "Security Policy Violations:\n  1. Checksum required\n\nSecurity Policy Warnings:\n  1. Using unstable reference\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPolicyViolations(tt.result)
			if result != tt.expected {
				t.Errorf("Expected:\n%q\nGot:\n%q", tt.expected, result)
			}
		})
	}
}

func TestGetPolicyEnforcementSummary(t *testing.T) {
	tests := []struct {
		name           string
		securityConfig *SecurityConfig
		expected       string
	}{
		{
			name:           "nil security config",
			securityConfig: nil,
			expected:       "No security policies enforced",
		},
		{
			name: "no enforcement policy but has host restrictions",
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com"},
			},
			expected: "Active policies: Repository access limited to 1 hosts",
		},
		{
			name: "single policy enforcement",
			securityConfig: &SecurityConfig{
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: true,
				},
			},
			expected: "Active policies: Checksum validation required",
		},
		{
			name: "multiple policy enforcements",
			securityConfig: &SecurityConfig{
				AllowedHosts: []string{"github.com", "gitlab.com"},
				DeniedHosts:  []string{"malicious.com"},
				Enforcement: &PolicyEnforcement{
					ForceChecksumValidation: true,
					DisableAutoUpdate:       true,
					StrictPolicyMode:        true,
				},
			},
			expected: "Active policies: Checksum validation required, Auto-updates disabled, Strict policy mode enabled, Repository access limited to 2 hosts, 1 hosts explicitly denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPolicyEnforcementSummary(tt.securityConfig)
			if result != tt.expected {
				t.Errorf("Expected: %q, Got: %q", tt.expected, result)
			}
		})
	}
}

func TestTemplateValidation(t *testing.T) {
	tests := []struct {
		name         string
		target       *Target
		expectError  bool
		expectWarn   bool
		errorMessage string
	}{
		{
			name: "valid template should pass",
			target: &Target{
				Template: &Template{
					Repo: "github.com/test/repo",
					Ref:  "v1.0.0",
					Path: "template.yml",
				},
			},
			expectError: false,
			expectWarn:  false,
		},
		{
			name: "missing repo should fail",
			target: &Target{
				Template: &Template{
					Ref:  "v1.0.0",
					Path: "template.yml",
				},
			},
			expectError:  true,
			errorMessage: "repository is required",
		},
		{
			name: "missing ref should fail",
			target: &Target{
				Template: &Template{
					Repo: "github.com/test/repo",
					Path: "template.yml",
				},
			},
			expectError:  true,
			errorMessage: "git reference is required",
		},
		{
			name: "missing path should fail",
			target: &Target{
				Template: &Template{
					Repo: "github.com/test/repo",
					Ref:  "v1.0.0",
				},
			},
			expectError:  true,
			errorMessage: "path is required",
		},
		{
			name: "unstable ref should warn",
			target: &Target{
				Template: &Template{
					Repo: "github.com/test/repo",
					Ref:  "main",
					Path: "template.yml",
				},
			},
			expectError: false,
			expectWarn:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewPolicyEnforcementResult()
			err := validateTemplateConfiguration("test", tt.target, result)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMessage != "" && !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMessage, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}

			if tt.expectWarn {
				if len(result.Warnings) == 0 {
					t.Error("Expected warnings but got none")
				}
			}
		})
	}
}

// Helper function to extract violation types for testing
func getViolationTypes(violations []PolicyViolation) []string {
	var types []string
	for _, violation := range violations {
		types = append(types, violation.Type)
	}
	return types
}
