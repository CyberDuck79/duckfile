//nolint:errcheck
package config

import (
	"os"
	"strings"
	"testing"
)

// TestTrackCommitHashValidation tests validation of commit hash tracking configuration
func TestTrackCommitHashValidation(t *testing.T) {
	tests := []struct {
		name        string
		target      Target
		targetName  string
		expectErr   bool
		errContains string
	}{
		{
			name: "valid commit hash tracking enabled",
			target: Target{
				Template: Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "main",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "valid auto-update with commit hash tracking",
			target: Target{
				Template: Template{
					Repo:               "https://github.com/test/repo.git",
					Ref:                "main",
					Path:               "test.tpl",
					TrackCommitHash:    true,
					AutoUpdateOnChange: true,
				},
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "invalid auto-update without commit hash tracking",
			target: Target{
				Template: Template{
					Repo:               "https://github.com/test/repo.git",
					Ref:                "main",
					Path:               "test.tpl",
					TrackCommitHash:    false,
					AutoUpdateOnChange: true,
				},
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "autoUpdateOnChange requires trackCommitHash",
		},
		{
			name: "commit hash tracking with commit hash ref",
			target: Target{
				Template: Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "a1b2c3d4e5f6789012345678901234567890abcd",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "commit hash tracking is invalid when ref is already a commit hash",
		},
		{
			name: "auto-update with commit hash ref",
			target: Target{
				Template: Template{
					Repo:               "https://github.com/test/repo.git",
					Ref:                "a1b2c3d4e5f6789012345678901234567890abcd",
					Path:               "test.tpl",
					TrackCommitHash:    true,
					AutoUpdateOnChange: true,
				},
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "commit hash tracking is invalid when ref is already a commit hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.target, tt.targetName)

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

// TestResolveTrackCommitHash tests precedence resolution for trackCommitHash setting
func TestResolveTrackCommitHash(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("DUCK_TRACK_COMMIT_HASH")
	defer os.Setenv("DUCK_TRACK_COMMIT_HASH", origEnv)

	tests := []struct {
		name        string
		cliFlag     *bool
		envValue    string
		template    *Template
		cfg         *DuckConf
		expected    bool
		description string
	}{
		{
			name:        "CLI flag true overrides everything",
			cliFlag:     boolPtr(true),
			envValue:    "false",
			template:    &Template{TrackCommitHash: false},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: false}},
			expected:    true,
			description: "CLI flag has highest precedence",
		},
		{
			name:        "CLI flag false overrides everything",
			cliFlag:     boolPtr(false),
			envValue:    "true",
			template:    &Template{TrackCommitHash: true},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: true}},
			expected:    false,
			description: "CLI flag has highest precedence",
		},
		{
			name:        "Environment variable true overrides config",
			cliFlag:     nil,
			envValue:    "true",
			template:    &Template{TrackCommitHash: false},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: false}},
			expected:    true,
			description: "Environment variable overrides template and global config",
		},
		{
			name:        "Environment variable 1 is treated as true",
			cliFlag:     nil,
			envValue:    "1",
			template:    &Template{TrackCommitHash: false},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: false}},
			expected:    true,
			description: "Environment variable '1' is treated as true",
		},
		{
			name:        "Environment variable false overrides config",
			cliFlag:     nil,
			envValue:    "false",
			template:    &Template{TrackCommitHash: true},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: true}},
			expected:    false,
			description: "Environment variable overrides template and global config",
		},
		{
			name:        "Template config overrides global config",
			cliFlag:     nil,
			envValue:    "",
			template:    &Template{TrackCommitHash: true},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: false}},
			expected:    true,
			description: "Template config overrides global config",
		},
		{
			name:        "Global config used when no higher precedence",
			cliFlag:     nil,
			envValue:    "",
			template:    &Template{TrackCommitHash: false},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: true}},
			expected:    true,
			description: "Global config used when template is false",
		},
		{
			name:        "Default false when no configuration",
			cliFlag:     nil,
			envValue:    "",
			template:    &Template{TrackCommitHash: false},
			cfg:         &DuckConf{Settings: &Settings{TrackCommitHash: false}},
			expected:    false,
			description: "Default is false",
		},
		{
			name:        "Default false with nil config",
			cliFlag:     nil,
			envValue:    "",
			template:    nil,
			cfg:         nil,
			expected:    false,
			description: "Default is false with nil config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("DUCK_TRACK_COMMIT_HASH", tt.envValue)
			} else {
				os.Unsetenv("DUCK_TRACK_COMMIT_HASH")
			}

			result := ResolveTrackCommitHash(tt.cliFlag, tt.template, tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %v, got %v - %s", tt.expected, result, tt.description)
			}
		})
	}
}

// TestResolveAutoUpdateOnChange tests precedence resolution for autoUpdateOnChange setting
func TestResolveAutoUpdateOnChange(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("DUCK_AUTO_UPDATE_ON_CHANGE")
	defer os.Setenv("DUCK_AUTO_UPDATE_ON_CHANGE", origEnv)

	tests := []struct {
		name        string
		cliFlag     *bool
		envValue    string
		template    *Template
		cfg         *DuckConf
		expected    bool
		description string
	}{
		{
			name:        "CLI flag true overrides everything",
			cliFlag:     boolPtr(true),
			envValue:    "false",
			template:    &Template{AutoUpdateOnChange: false},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: false}},
			expected:    true,
			description: "CLI flag has highest precedence",
		},
		{
			name:        "CLI flag false overrides everything",
			cliFlag:     boolPtr(false),
			envValue:    "true",
			template:    &Template{AutoUpdateOnChange: true},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: true}},
			expected:    false,
			description: "CLI flag has highest precedence",
		},
		{
			name:        "Environment variable true overrides config",
			cliFlag:     nil,
			envValue:    "true",
			template:    &Template{AutoUpdateOnChange: false},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: false}},
			expected:    true,
			description: "Environment variable overrides template and global config",
		},
		{
			name:        "Environment variable 1 is treated as true",
			cliFlag:     nil,
			envValue:    "1",
			template:    &Template{AutoUpdateOnChange: false},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: false}},
			expected:    true,
			description: "Environment variable '1' is treated as true",
		},
		{
			name:        "Template config overrides global config",
			cliFlag:     nil,
			envValue:    "",
			template:    &Template{AutoUpdateOnChange: true},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: false}},
			expected:    true,
			description: "Template config overrides global config",
		},
		{
			name:        "Global config used when no higher precedence",
			cliFlag:     nil,
			envValue:    "",
			template:    &Template{AutoUpdateOnChange: false},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: true}},
			expected:    true,
			description: "Global config used when template is false",
		},
		{
			name:        "Default false when no configuration",
			cliFlag:     nil,
			envValue:    "",
			template:    &Template{AutoUpdateOnChange: false},
			cfg:         &DuckConf{Settings: &Settings{AutoUpdateOnChange: false}},
			expected:    false,
			description: "Default is false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("DUCK_AUTO_UPDATE_ON_CHANGE", tt.envValue)
			} else {
				os.Unsetenv("DUCK_AUTO_UPDATE_ON_CHANGE")
			}

			result := ResolveAutoUpdateOnChange(tt.cliFlag, tt.template, tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %v, got %v - %s", tt.expected, result, tt.description)
			}
		})
	}
}

// boolPtr is a helper function to create a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}
