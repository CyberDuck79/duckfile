//nolint:errcheck
package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

func TestExtractHostFromGitURL(t *testing.T) {
	tests := []struct {
		name      string
		repoURL   string
		want      string
		expectErr bool
	}{
		// HTTPS URLs
		{
			name:    "HTTPS URL with .git suffix",
			repoURL: "https://github.com/user/repo.git",
			want:    "github.com",
		},
		{
			name:    "HTTPS URL without .git suffix",
			repoURL: "https://github.com/user/repo",
			want:    "github.com",
		},
		{
			name:    "HTTPS URL with port",
			repoURL: "https://git.company.com:8443/user/repo.git",
			want:    "git.company.com:8443",
		},
		{
			name:    "HTTP URL",
			repoURL: "http://git.internal.com/user/repo.git",
			want:    "git.internal.com",
		},

		// SSH URLs (SCP-style)
		{
			name:    "SSH SCP-style URL",
			repoURL: "git@github.com:user/repo.git",
			want:    "github.com",
		},
		{
			name:    "SSH SCP-style URL without .git",
			repoURL: "git@gitlab.com:user/repo",
			want:    "gitlab.com",
		},
		{
			name:    "SSH SCP-style URL with custom user",
			repoURL: "myuser@git.company.com:user/repo.git",
			want:    "git.company.com",
		},

		// SSH URLs with ssh:// scheme
		{
			name:    "SSH URL with ssh:// scheme",
			repoURL: "ssh://git@github.com/user/repo.git",
			want:    "github.com",
		},
		{
			name:    "SSH URL with ssh:// scheme and port",
			repoURL: "ssh://git@github.com:22/user/repo.git",
			want:    "github.com:22",
		},

		// Error cases
		{
			name:      "empty URL",
			repoURL:   "",
			expectErr: true,
		},
		{
			name:      "whitespace only URL",
			repoURL:   "   ",
			expectErr: true,
		},
		{
			name:      "invalid HTTPS URL",
			repoURL:   "https://",
			expectErr: true,
		},
		{
			name:      "invalid SCP-style URL (no colon)",
			repoURL:   "git@github.com",
			expectErr: true,
		},
		{
			name:      "invalid SCP-style URL (no @)",
			repoURL:   "github.com:user/repo.git",
			expectErr: true,
		},
		{
			name:      "unsupported protocol",
			repoURL:   "ftp://github.com/user/repo.git",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a custom function to test the private extractHostFromGitURL
			// We'll create a wrapper that exposes it via ValidateRepoAccess
			cfg := &config.SecurityConfig{
				AllowedHosts: []string{"test.com"}, // Dummy allowed host
			}

			err := config.ValidateRepoAccess(tt.repoURL, cfg)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				// Don't check the specific error message for parsing errors
				return
			}

			if err != nil && !tt.expectErr {
				// Check if it's just a "not allowed" error, which means parsing worked
				if !isHostNotAllowedError(err) {
					t.Fatalf("unexpected parsing error: %v", err)
				}
				// If it's a host not allowed error, parsing worked correctly
			}

			// For successful cases, we need to test with the actual host in allowed list
			if !tt.expectErr {
				cfgWithCorrectHost := &config.SecurityConfig{
					AllowedHosts: []string{tt.want},
				}
				err := config.ValidateRepoAccess(tt.repoURL, cfgWithCorrectHost)
				if err != nil {
					t.Fatalf("validation failed with correct host: %v", err)
				}
			}
		})
	}
}

// Helper function to check if error is about host not being allowed (vs parsing error)
func isHostNotAllowedError(err error) bool {
	errMsg := err.Error()
	return containsAny(errMsg, []string{"not in allowed hosts", "explicitly denied"})
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func TestValidateRepoAccess(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		securityCfg *config.SecurityConfig
		expectErr   bool
		errorMsg    string
	}{
		{
			name:    "no restrictions allows all",
			repoURL: "https://github.com/user/repo.git",
			securityCfg: &config.SecurityConfig{
				Source: "none",
			},
			expectErr: false,
		},
		{
			name:    "allowed host passes",
			repoURL: "https://github.com/user/repo.git",
			securityCfg: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Source:       "env",
			},
			expectErr: false,
		},
		{
			name:    "denied host fails",
			repoURL: "https://malicious-host.com/user/repo.git",
			securityCfg: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Source:       "env",
			},
			expectErr: true,
			errorMsg:  "not in allowed hosts",
		},
		{
			name:    "explicitly denied host fails",
			repoURL: "https://malicious-host.com/user/repo.git",
			securityCfg: &config.SecurityConfig{
				AllowedHosts: []string{"github.com", "malicious-host.com"}, // In allow list
				DeniedHosts:  []string{"malicious-host.com"},               // But also denied
				Source:       "cli",
			},
			expectErr: true,
			errorMsg:  "explicitly denied",
		},
		{
			name:    "strict mode with no restrictions fails",
			repoURL: "https://github.com/user/repo.git",
			securityCfg: &config.SecurityConfig{
				StrictMode: true,
				Source:     "cli",
			},
			expectErr: true,
			errorMsg:  "strict mode enabled but no host restrictions configured",
		},
		{
			name:    "case insensitive host matching",
			repoURL: "https://GitHub.COM/user/repo.git",
			securityCfg: &config.SecurityConfig{
				AllowedHosts: []string{"github.com"},
				Source:       "env",
			},
			expectErr: false,
		},
		{
			name:    "SSH URL validation",
			repoURL: "git@gitlab.com:user/repo.git",
			securityCfg: &config.SecurityConfig{
				AllowedHosts: []string{"gitlab.com"},
				Source:       "env",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateRepoAccess(tt.repoURL, tt.securityCfg)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadSecurityConfigFromEnv(t *testing.T) {
	// Save current environment and restore after test
	oldAllowed := os.Getenv("DUCK_ALLOWED_HOSTS")
	oldDenied := os.Getenv("DUCK_DENIED_HOSTS")
	oldStrict := os.Getenv("DUCK_STRICT_MODE")

	defer func() {
		os.Setenv("DUCK_ALLOWED_HOSTS", oldAllowed)
		os.Setenv("DUCK_DENIED_HOSTS", oldDenied)
		os.Setenv("DUCK_STRICT_MODE", oldStrict)
	}()

	tests := []struct {
		name            string
		allowedEnv      string
		deniedEnv       string
		strictEnv       string
		expectedAllowed []string
		expectedDenied  []string
		expectedStrict  bool
		expectedSource  string
	}{
		{
			name:           "no environment variables",
			expectedSource: "none",
		},
		{
			name:            "single allowed host",
			allowedEnv:      "github.com",
			expectedAllowed: []string{"github.com"},
			expectedSource:  "env",
		},
		{
			name:            "multiple allowed hosts",
			allowedEnv:      "github.com,gitlab.com,bitbucket.org",
			expectedAllowed: []string{"github.com", "gitlab.com", "bitbucket.org"},
			expectedSource:  "env",
		},
		{
			name:            "allowed hosts with whitespace",
			allowedEnv:      " github.com , gitlab.com , bitbucket.org ",
			expectedAllowed: []string{"github.com", "gitlab.com", "bitbucket.org"},
			expectedSource:  "env",
		},
		{
			name:           "denied hosts",
			deniedEnv:      "malicious-host.com,suspicious-site.org",
			expectedDenied: []string{"malicious-host.com", "suspicious-site.org"},
			expectedSource: "env",
		},
		{
			name:           "strict mode true",
			strictEnv:      "true",
			expectedStrict: true,
			expectedSource: "env",
		},
		{
			name:           "strict mode TRUE (case insensitive)",
			strictEnv:      "TRUE",
			expectedStrict: true,
			expectedSource: "env",
		},
		{
			name:           "strict mode false",
			strictEnv:      "false",
			expectedStrict: false,
			expectedSource: "env",
		},
		{
			name:            "all settings combined",
			allowedEnv:      "github.com,gitlab.com",
			deniedEnv:       "malicious-host.com",
			strictEnv:       "true",
			expectedAllowed: []string{"github.com", "gitlab.com"},
			expectedDenied:  []string{"malicious-host.com"},
			expectedStrict:  true,
			expectedSource:  "env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("DUCK_ALLOWED_HOSTS")
			os.Unsetenv("DUCK_DENIED_HOSTS")
			os.Unsetenv("DUCK_STRICT_MODE")

			// Set test environment
			if tt.allowedEnv != "" {
				os.Setenv("DUCK_ALLOWED_HOSTS", tt.allowedEnv)
			}
			if tt.deniedEnv != "" {
				os.Setenv("DUCK_DENIED_HOSTS", tt.deniedEnv)
			}
			if tt.strictEnv != "" {
				os.Setenv("DUCK_STRICT_MODE", tt.strictEnv)
			}

			cfg := config.LoadSecurityConfigFromEnv()

			if cfg.Source != tt.expectedSource {
				t.Errorf("expected source %q, got %q", tt.expectedSource, cfg.Source)
			}

			if !stringSliceEqual(cfg.AllowedHosts, tt.expectedAllowed) {
				t.Errorf("expected allowed hosts %v, got %v", tt.expectedAllowed, cfg.AllowedHosts)
			}

			if !stringSliceEqual(cfg.DeniedHosts, tt.expectedDenied) {
				t.Errorf("expected denied hosts %v, got %v", tt.expectedDenied, cfg.DeniedHosts)
			}

			if cfg.StrictMode != tt.expectedStrict {
				t.Errorf("expected strict mode %v, got %v", tt.expectedStrict, cfg.StrictMode)
			}
		})
	}
}

func TestBuildSecurityConfig(t *testing.T) {
	// Save current environment and restore after test
	oldAllowed := os.Getenv("DUCK_ALLOWED_HOSTS")
	oldDenied := os.Getenv("DUCK_DENIED_HOSTS")
	oldStrict := os.Getenv("DUCK_STRICT_MODE")

	defer func() {
		os.Setenv("DUCK_ALLOWED_HOSTS", oldAllowed)
		os.Setenv("DUCK_DENIED_HOSTS", oldDenied)
		os.Setenv("DUCK_STRICT_MODE", oldStrict)
	}()

	tests := []struct {
		name            string
		cliAllowed      []string
		cliDenied       []string
		cliStrict       bool
		envAllowed      string
		envDenied       string
		envStrict       string
		expectedAllowed []string
		expectedDenied  []string
		expectedStrict  bool
		expectedSource  string
	}{
		{
			name:            "CLI flags override environment",
			cliAllowed:      []string{"github.com"},
			cliDenied:       []string{"malicious.com"},
			cliStrict:       true,
			envAllowed:      "gitlab.com",
			envDenied:       "suspicious.com",
			envStrict:       "false",
			expectedAllowed: []string{"github.com"},
			expectedDenied:  []string{"malicious.com"},
			expectedStrict:  true,
			expectedSource:  "cli",
		},
		{
			name:            "environment used when no CLI flags",
			envAllowed:      "gitlab.com,bitbucket.org",
			envDenied:       "bad-host.com",
			envStrict:       "true",
			expectedAllowed: []string{"gitlab.com", "bitbucket.org"},
			expectedDenied:  []string{"bad-host.com"},
			expectedStrict:  true,
			expectedSource:  "env",
		},
		{
			name:           "no configuration results in 'none' source",
			expectedSource: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("DUCK_ALLOWED_HOSTS")
			os.Unsetenv("DUCK_DENIED_HOSTS")
			os.Unsetenv("DUCK_STRICT_MODE")

			// Set test environment if provided
			if tt.envAllowed != "" {
				os.Setenv("DUCK_ALLOWED_HOSTS", tt.envAllowed)
			}
			if tt.envDenied != "" {
				os.Setenv("DUCK_DENIED_HOSTS", tt.envDenied)
			}
			if tt.envStrict != "" {
				os.Setenv("DUCK_STRICT_MODE", tt.envStrict)
			}

			cfg := config.BuildSecurityConfig(tt.cliAllowed, tt.cliDenied, tt.cliStrict)

			if cfg.Source != tt.expectedSource {
				t.Errorf("expected source %q, got %q", tt.expectedSource, cfg.Source)
			}

			if !stringSliceEqual(cfg.AllowedHosts, tt.expectedAllowed) {
				t.Errorf("expected allowed hosts %v, got %v", tt.expectedAllowed, cfg.AllowedHosts)
			}

			if !stringSliceEqual(cfg.DeniedHosts, tt.expectedDenied) {
				t.Errorf("expected denied hosts %v, got %v", tt.expectedDenied, cfg.DeniedHosts)
			}

			if cfg.StrictMode != tt.expectedStrict {
				t.Errorf("expected strict mode %v, got %v", tt.expectedStrict, cfg.StrictMode)
			}
		})
	}
}

// Helper function to compare string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
