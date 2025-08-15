package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// SecurityConfig holds host allow/deny list configuration loaded from
// environment variables or CLI flags to prevent supply-chain attacks.
// Security settings are kept separate from duck.yaml to prevent attackers
// from modifying both targets and security policies in the same commit.
type SecurityConfig struct {
	AllowedHosts []string
	DeniedHosts  []string
	StrictMode   bool   // Fail if no restrictions are configured
	Source       string // "env", "cli", "none" for audit trail
}

// LoadSecurityConfigFromEnv loads security configuration from environment variables.
// Environment variables used:
//   - DUCK_ALLOWED_HOSTS: comma-separated list of allowed hostnames
//   - DUCK_DENIED_HOSTS: comma-separated list of denied hostnames
//   - DUCK_STRICT_MODE: "true" to fail if no restrictions are configured
func LoadSecurityConfigFromEnv() *SecurityConfig {
	cfg := &SecurityConfig{Source: "none"}

	// Parse DUCK_ALLOWED_HOSTS
	if allowedEnv := strings.TrimSpace(os.Getenv("DUCK_ALLOWED_HOSTS")); allowedEnv != "" {
		cfg.AllowedHosts = parseHostList(allowedEnv)
		cfg.Source = "env"
	}

	// Parse DUCK_DENIED_HOSTS
	if deniedEnv := strings.TrimSpace(os.Getenv("DUCK_DENIED_HOSTS")); deniedEnv != "" {
		cfg.DeniedHosts = parseHostList(deniedEnv)
		cfg.Source = "env"
	}

	// Parse DUCK_STRICT_MODE
	if strictEnv := strings.TrimSpace(os.Getenv("DUCK_STRICT_MODE")); strictEnv != "" {
		cfg.StrictMode = strings.ToLower(strictEnv) == "true"
		if cfg.Source == "none" {
			cfg.Source = "env"
		}
	}

	return cfg
}

// BuildSecurityConfig creates a security configuration with CLI flag precedence.
// CLI flags override environment variables.
func BuildSecurityConfig(cliAllowed []string, cliDenied []string, cliStrict bool) *SecurityConfig {
	// CLI flags have highest precedence
	if len(cliAllowed) > 0 || len(cliDenied) > 0 || cliStrict {
		return &SecurityConfig{
			AllowedHosts: cliAllowed,
			DeniedHosts:  cliDenied,
			StrictMode:   cliStrict,
			Source:       "cli",
		}
	}

	// Fallback to environment variables
	return LoadSecurityConfigFromEnv()
}

// ValidateRepoAccess validates that a repository URL is allowed by the security policy.
// Returns an error if the repository host is denied or not in the allow list.
func ValidateRepoAccess(repoURL string, securityCfg *SecurityConfig) error {
	if securityCfg == nil {
		return fmt.Errorf("security configuration is nil")
	}

	// Extract hostname from repository URL
	host, err := extractHostFromGitURL(repoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository URL %q: %w", repoURL, err)
	}

	// In strict mode, fail if no restrictions are configured
	if securityCfg.StrictMode && len(securityCfg.AllowedHosts) == 0 && len(securityCfg.DeniedHosts) == 0 {
		return fmt.Errorf("strict mode enabled but no host restrictions configured (source: %s)", securityCfg.Source)
	}

	// Deny list takes precedence - check first
	for _, denied := range securityCfg.DeniedHosts {
		if matchHost(host, denied) {
			return fmt.Errorf("repository host %q is explicitly denied (source: %s, denied hosts: %v)",
				host, securityCfg.Source, securityCfg.DeniedHosts)
		}
	}

	// If allow list is configured, host must be in it
	if len(securityCfg.AllowedHosts) > 0 {
		for _, allowed := range securityCfg.AllowedHosts {
			if matchHost(host, allowed) {
				return nil // Explicitly allowed
			}
		}
		return fmt.Errorf("repository host %q is not in allowed hosts %v (source: %s)",
			host, securityCfg.AllowedHosts, securityCfg.Source)
	}

	// No restrictions configured and not in strict mode - allow all
	return nil
}

// extractHostFromGitURL extracts the hostname from various Git URL formats:
//   - HTTPS: https://github.com/user/repo.git
//   - SSH: git@github.com:user/repo.git
//   - SSH with port: ssh://git@github.com:22/user/repo.git
//   - For testing: simple hostnames like "stub", "local", etc. are returned as-is
func extractHostFromGitURL(repoURL string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return "", fmt.Errorf("empty repository URL")
	}

	// For testing: allow simple hostnames/stub URLs
	if !strings.Contains(repoURL, "://") && !strings.Contains(repoURL, "@") {
		// Simple hostname like "stub", "local", "r1", etc. used in tests
		return repoURL, nil
	}

	// Handle HTTPS/HTTP URLs
	if strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "http://") {
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", fmt.Errorf("invalid HTTP(S) URL: %w", err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("no host in HTTP(S) URL")
		}
		return u.Host, nil
	}

	// Handle SSH URLs with ssh:// scheme
	if strings.HasPrefix(repoURL, "ssh://") {
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", fmt.Errorf("invalid SSH URL: %w", err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("no host in SSH URL")
		}
		return u.Host, nil
	}

	// Handle SCP-style SSH URLs: git@github.com:user/repo.git
	if strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") {
		// Split on @ to get user and host:path parts
		parts := strings.SplitN(repoURL, "@", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid SCP-style SSH URL format")
		}

		// Split host:path part on first colon
		hostPart := strings.SplitN(parts[1], ":", 2)
		if len(hostPart) != 2 {
			return "", fmt.Errorf("invalid SCP-style SSH URL format: missing colon after host")
		}

		host := strings.TrimSpace(hostPart[0])
		if host == "" {
			return "", fmt.Errorf("empty hostname in SCP-style SSH URL")
		}

		return host, nil
	}

	return "", fmt.Errorf("unsupported URL format: must be HTTPS, HTTP, ssh://, or SCP-style SSH")
} // matchHost checks if a hostname matches a pattern.
// For now, only exact matches are supported. Future versions could add wildcard support.
func matchHost(host, pattern string) bool {
	return strings.EqualFold(strings.TrimSpace(host), strings.TrimSpace(pattern))
}

// parseHostList parses a comma-separated list of hostnames, trimming whitespace.
func parseHostList(hostList string) []string {
	if hostList == "" {
		return nil
	}

	hosts := strings.Split(hostList, ",")
	result := make([]string, 0, len(hosts))

	for _, host := range hosts {
		if trimmed := strings.TrimSpace(host); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
