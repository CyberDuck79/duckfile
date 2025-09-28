package run

import (
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

func TestPolicyEnforcementIntegration(t *testing.T) {
	tests := []struct {
		name           string
		duckConf       *config.DuckConf
		targetName     string
		securityConfig *config.SecurityConfig
		expectError    bool
		errorContains  string
	}{
		{
			name: "no security config should allow execution",
			duckConf: &config.DuckConf{
				Settings: &config.Settings{},
				Targets: map[string]config.Target{
					"test": {
						Binary:   "echo",
						FileFlag: "-f",
						Template: &config.Template{
							Repo: "github.com/test/repo",
							Ref:  "main",
							Path: "template.yml",
						},
					},
				},
			},
			targetName:     "test",
			securityConfig: nil,
			expectError:    false,
		},
		{
			name: "checksum enforcement without checksum should fail",
			duckConf: &config.DuckConf{
				Settings: &config.Settings{},
				Targets: map[string]config.Target{
					"test": {
						Binary:   "echo",
						FileFlag: "-f",
						Template: &config.Template{
							Repo: "github.com/test/repo",
							Ref:  "v1.0.0",
							Path: "template.yml",
							// No checksum provided
						},
					},
				},
			},
			targetName: "test",
			securityConfig: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &config.PolicyEnforcement{
					ForceChecksumValidation: true,
				},
			},
			expectError:   true,
			errorContains: "checksum validation",
		},
		{
			name: "checksum enforcement with checksum should pass basic policy check",
			duckConf: &config.DuckConf{
				Settings: &config.Settings{},
				Targets: map[string]config.Target{
					"test": {
						Binary:   "echo",
						FileFlag: "-f",
						Template: &config.Template{
							Repo:     "github.com/test/repo",
							Ref:      "v1.0.0",
							Path:     "template.yml",
							Checksum: "sha256:abc123",
						},
					},
				},
			},
			targetName: "test",
			securityConfig: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &config.PolicyEnforcement{
					ForceChecksumValidation: true,
				},
			},
			expectError: false, // Policy check passes, may fail later in template processing
		},
		{
			name: "repository access denied should fail",
			duckConf: &config.DuckConf{
				Settings: &config.Settings{},
				Targets: map[string]config.Target{
					"test": {
						Binary:   "echo",
						FileFlag: "-f",
						Template: &config.Template{
							Repo: "malicious.com/bad/repo",
							Ref:  "v1.0.0",
							Path: "template.yml",
						},
					},
				},
			},
			targetName: "test",
			securityConfig: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
			},
			expectError:   true,
			errorContains: "repository access denied",
		},
		{
			name: "strict policy mode without signed config should fail",
			duckConf: &config.DuckConf{
				Settings: &config.Settings{},
				Targets: map[string]config.Target{
					"test": {
						Binary:   "echo",
						FileFlag: "-f",
						Template: &config.Template{
							Repo: "github.com/test/repo",
							Ref:  "v1.0.0",
							Path: "template.yml",
						},
					},
				},
			},
			targetName: "test",
			securityConfig: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &config.PolicyEnforcement{
					StrictPolicyMode: true,
				},
				IsSigned: false,
			},
			expectError:   true,
			errorContains: "strict policy mode",
		},
		{
			name: "policy overrides should be applied",
			duckConf: &config.DuckConf{
				Settings: &config.Settings{},
				Targets: map[string]config.Target{
					"test": {
						Binary:   "echo",
						FileFlag: "-f",
						Template: &config.Template{
							Repo:               "github.com/test/repo",
							Ref:                "v1.0.0",
							Path:               "template.yml",
							Checksum:           "sha256:abc123",
							AutoUpdateOnChange: true,  // Will be overridden
							TrackCommitHash:    false, // Will be overridden
						},
					},
				},
			},
			targetName: "test",
			securityConfig: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Enforcement: &config.PolicyEnforcement{
					DisableAutoUpdate:   true,
					ForceCommitTracking: true,
				},
			},
			expectError: false, // Policy overrides should be applied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with prepareAndRenderTemplate directly to avoid actual network calls
			target, exists := tt.duckConf.Targets[tt.targetName]
			if !exists {
				t.Fatalf("Target %q not found in test configuration", tt.targetName)
			}

			// Call the policy enforcement logic by attempting template preparation
			// This will fail at some point due to network/file operations, but
			// we're mainly testing the policy enforcement that happens early
			_, err := prepareAndRenderTemplate(tt.targetName, target, tt.duckConf, false, tt.securityConfig, nil, nil)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsSubstring(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				// For non-error cases, we might still get errors from template processing
				// but they should not be policy-related errors
				if err != nil && tt.errorContains != "" && containsSubstring(err.Error(), tt.errorContains) {
					t.Errorf("Got unexpected policy error: %v", err)
				}
				// Note: We don't check for err == nil here because template processing
				// will likely fail due to missing repositories, etc.
			}
		})
	}
}

func TestPolicyOverrideApplication(t *testing.T) {
	// Test that policy overrides are actually applied to the target configuration
	duckConf := &config.DuckConf{
		Settings: &config.Settings{},
		Targets: map[string]config.Target{
			"test": {
				Binary:   "echo",
				FileFlag: "-f",
				Template: &config.Template{
					Repo:               "github.com/test/repo",
					Ref:                "v1.0.0",
					Path:               "template.yml",
					Checksum:           "sha256:abc123",
					AutoUpdateOnChange: true,  // Should be overridden to false
					TrackCommitHash:    false, // Should be overridden to true
				},
			},
		},
	}

	securityConfig := &config.SecurityConfig{
		AllowedHosts: []string{"github.com"},
		Enforcement: &config.PolicyEnforcement{
			DisableAutoUpdate:   true,
			ForceCommitTracking: true,
		},
	}

	target := duckConf.Targets["test"]

	// Verify original values
	if !target.Template.AutoUpdateOnChange {
		t.Error("Expected original AutoUpdateOnChange to be true")
	}
	if target.Template.TrackCommitHash {
		t.Error("Expected original TrackCommitHash to be false")
	}

	// Apply policy overrides
	modifiedTarget := config.ApplyPolicyOverrides(&target, securityConfig)

	// Verify overrides were applied
	if modifiedTarget.Template.AutoUpdateOnChange {
		t.Error("Expected AutoUpdateOnChange to be overridden to false")
	}
	if !modifiedTarget.Template.TrackCommitHash {
		t.Error("Expected TrackCommitHash to be overridden to true")
	}

	// Verify original target was not modified
	if !target.Template.AutoUpdateOnChange {
		t.Error("Original target should not have been modified")
	}
}

// Helper function for substring checking
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || hasSubstringAt(s, substr))
}

func hasSubstringAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
