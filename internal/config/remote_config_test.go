package config

import (
	"strings"
	"testing"
)

// TestRemoteConfigurationValidation tests the new remote configuration validation
func TestRemoteConfigurationValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *DuckConf
		expectErr bool
		errContains string
	}{
		{
			name: "Valid remote configuration",
			config: &DuckConf{
				Version: 1,
				Default: "build",
				Remotes: map[string]Remote{
					"company-main": {
						Repo:               "https://github.com/company/templates.git",
						Ref:                "main",
						TrackCommitHash:    true,
						AutoUpdateOnChange: false,
					},
				},
				Targets: map[string]Target{
					"build": {
						Binary:   "make",
						FileFlag: "-f",
						Template: Template{
							Remote: "company-main",
							Path:   "Makefile.tpl",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid remote reference",
			config: &DuckConf{
				Version: 1,
				Default: "build",
				Remotes: map[string]Remote{
					"company-main": {
						Repo: "https://github.com/company/templates.git",
						Ref:  "main",
					},
				},
				Targets: map[string]Target{
					"build": {
						Binary:   "make",
						FileFlag: "-f",
						Template: Template{
							Remote: "nonexistent",
							Path:   "Makefile.tpl",
						},
					},
				},
			},
			expectErr:   true,
			errContains: "remote \"nonexistent\" not found",
		},
		{
			name: "Mixed remote and inline configuration",
			config: &DuckConf{
				Version: 1,
				Default: "build",
				Remotes: map[string]Remote{
					"company-main": {
						Repo: "https://github.com/company/templates.git",
						Ref:  "main",
					},
				},
				Targets: map[string]Target{
					"build": {
						Binary:   "make",
						FileFlag: "-f",
						Template: Template{
							Remote: "company-main",
							Repo:   "https://github.com/other/repo.git", // Should not be allowed
							Path:   "Makefile.tpl",
						},
					},
				},
			},
			expectErr:   true,
			errContains: "cannot specify remote settings when using remote reference",
		},
		{
			name: "Remote with commit hash tracking validation",
			config: &DuckConf{
				Version: 1,
				Default: "build",
				Remotes: map[string]Remote{
					"invalid-remote": {
						Repo:            "https://github.com/company/templates.git",
						Ref:             "abc1234567890123456789012345678901234567", // commit hash
						TrackCommitHash: true, // Should fail
					},
				},
				Targets: map[string]Target{
					"build": {
						Binary:   "make",
						FileFlag: "-f",
						Template: Template{
							Remote: "invalid-remote",
							Path:   "Makefile.tpl",
						},
					},
				},
			},
			expectErr:   true,
			errContains: "commit hash tracking is invalid when ref is already a commit hash",
		},
		{
			name: "Remote auto-update without tracking",
			config: &DuckConf{
				Version: 1,
				Default: "build",
				Remotes: map[string]Remote{
					"invalid-remote": {
						Repo:               "https://github.com/company/templates.git",
						Ref:                "main",
						TrackCommitHash:    false,
						AutoUpdateOnChange: true, // Should fail without trackCommitHash
					},
				},
				Targets: map[string]Target{
					"build": {
						Binary:   "make",
						FileFlag: "-f",
						Template: Template{
							Remote: "invalid-remote",
							Path:   "Makefile.tpl",
						},
					},
				},
			},
			expectErr:   true,
			errContains: "autoUpdateOnChange requires trackCommitHash to be enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestTemplateConfigResolution tests the template configuration resolution
func TestTemplateConfigResolution(t *testing.T) {
	tests := []struct {
		name     string
		template Template
		remotes  map[string]Remote
		settings *Settings
		expected ResolvedTemplate
		expectErr bool
	}{
		{
			name: "Remote reference resolution",
			template: Template{
				Remote:   "company-main",
				Path:     "Makefile.tpl",
				Checksum: "abc123",
			},
			remotes: map[string]Remote{
				"company-main": {
					Repo:               "https://github.com/company/templates.git",
					Ref:                "main",
					Submodules:         true,
					TrackCommitHash:    true,
					AutoUpdateOnChange: false,
				},
			},
			expected: ResolvedTemplate{
				Repo:               "https://github.com/company/templates.git",
				Ref:                "main",
				Path:               "Makefile.tpl",
				Submodules:         true,
				TrackCommitHash:    true,
				AutoUpdateOnChange: false,
				Checksum:           "abc123",
			},
			expectErr: false,
		},
		{
			name: "Inline template resolution",
			template: Template{
				Repo:               "https://github.com/inline/repo.git",
				Ref:                "v1.0.0",
				Path:               "inline.tpl",
				TrackCommitHash:    true,
				AutoUpdateOnChange: false,
			},
			remotes: nil,
			expected: ResolvedTemplate{
				Repo:               "https://github.com/inline/repo.git",
				Ref:                "v1.0.0",
				Path:               "inline.tpl",
				TrackCommitHash:    true,
				AutoUpdateOnChange: false,
			},
			expectErr: false,
		},
		{
			name: "Inline template with settings fallback",
			template: Template{
				Repo: "https://github.com/inline/repo.git",
				Ref:  "main",
				Path: "template.tpl",
				// TrackCommitHash and AutoUpdateOnChange not set, should use settings
			},
			settings: &Settings{
				TrackCommitHash:    true,
				AutoUpdateOnChange: true,
			},
			expected: ResolvedTemplate{
				Repo:               "https://github.com/inline/repo.git",
				Ref:                "main",
				Path:               "template.tpl",
				TrackCommitHash:    true,
				AutoUpdateOnChange: true,
			},
			expectErr: false,
		},
		{
			name: "Remote reference not found",
			template: Template{
				Remote: "nonexistent",
				Path:   "template.tpl",
			},
			remotes:   map[string]Remote{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := ResolveTemplateConfig(tt.template, tt.remotes, tt.settings)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
				return
			}

			// Compare resolved template fields
			if resolved.Repo != tt.expected.Repo {
				t.Errorf("expected Repo %q, got %q", tt.expected.Repo, resolved.Repo)
			}
			if resolved.Ref != tt.expected.Ref {
				t.Errorf("expected Ref %q, got %q", tt.expected.Ref, resolved.Ref)
			}
			if resolved.Path != tt.expected.Path {
				t.Errorf("expected Path %q, got %q", tt.expected.Path, resolved.Path)
			}
			if resolved.Submodules != tt.expected.Submodules {
				t.Errorf("expected Submodules %v, got %v", tt.expected.Submodules, resolved.Submodules)
			}
			if resolved.TrackCommitHash != tt.expected.TrackCommitHash {
				t.Errorf("expected TrackCommitHash %v, got %v", tt.expected.TrackCommitHash, resolved.TrackCommitHash)
			}
			if resolved.AutoUpdateOnChange != tt.expected.AutoUpdateOnChange {
				t.Errorf("expected AutoUpdateOnChange %v, got %v", tt.expected.AutoUpdateOnChange, resolved.AutoUpdateOnChange)
			}
			if resolved.Checksum != tt.expected.Checksum {
				t.Errorf("expected Checksum %q, got %q", tt.expected.Checksum, resolved.Checksum)
			}
		})
	}
}