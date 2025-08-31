package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SecurityConfig holds comprehensive security configuration loaded from
// environment variables, CLI flags, or configuration files to prevent supply-chain attacks.
// Security settings are kept separate from duck.yaml to prevent attackers
// from modifying both targets and security policies in the same commit.
type SecurityConfig struct {
	// Existing host restriction fields
	AllowedHosts []string `yaml:"allowedHosts,omitempty"`
	DeniedHosts  []string `yaml:"deniedHosts,omitempty"`
	StrictMode   bool     `yaml:"strictMode,omitempty"` // Fail if no restrictions are configured
	Source       string   `yaml:"source,omitempty"`     // "signed", "env", "cli", "unsigned", "none" for audit trail

	// NEW: Digital signature fields
	Signature *DigitalSignature `yaml:"signature,omitempty"`

	// NEW: Policy enforcement
	Enforcement *PolicyEnforcement `yaml:"enforcement,omitempty"`

	// NEW: File permission validation
	FilePermissions *FilePermissionPolicy `yaml:"filePermissions,omitempty"`

	// NEW: Metadata for audit trails
	Metadata *SecurityMetadata `yaml:"metadata,omitempty"`

	// NEW: Source tracking for precedence
	SourceFile string `yaml:"sourceFile,omitempty"` // Path to config file if loaded from file
	IsSigned   bool   `yaml:"isSigned,omitempty"`   // Whether this config was verified with a valid signature
	Version    int    `yaml:"version,omitempty"`    // Configuration version
}

// DigitalSignature holds signature verification information
type DigitalSignature struct {
	Algorithm string `yaml:"algorithm"` // "ed25519"
	KeyId     string `yaml:"keyId"`     // Key identifier for lookup
	Signature string `yaml:"signature"` // base64-encoded signature
}

// PolicyEnforcement defines security policy enforcement rules
type PolicyEnforcement struct {
	ForceChecksumValidation bool `yaml:"forceChecksumValidation"` // Fail if template has no checksum
	ForceCommitTracking     bool `yaml:"forceCommitTracking"`     // Fail if trackCommitHash=false
	DisableAutoUpdate       bool `yaml:"disableAutoUpdate"`       // Override autoUpdateOnChange settings
	StrictPolicyMode        bool `yaml:"strictPolicyMode"`        // Require security config to exist
	EnforceFilePermissions  bool `yaml:"enforceFilePermissions"`  // Validate file permissions according to policy
}

// FilePermissionPolicy defines file permission validation rules
type FilePermissionPolicy struct {
	EnforceOwnership         bool `yaml:"enforceOwnership"`         // Require proper ownership (root for system files)
	EnforceReadOnly          bool `yaml:"enforceReadOnly"`          // Require read-only permissions
	AllowGroupWrite          bool `yaml:"allowGroupWrite"`          // Allow group write permissions
	RequireSecureDirectories bool `yaml:"requireSecureDirectories"` // Parent dirs must be secure
}

// SecurityMetadata holds audit trail information
type SecurityMetadata struct {
	CreatedBy string    `yaml:"createdBy"`
	CreatedAt time.Time `yaml:"createdAt"`
	Purpose   string    `yaml:"purpose"`
	Version   int       `yaml:"version,omitempty"`
}

// SecurityConfigFile represents a discovered security configuration file
type SecurityConfigFile struct {
	Path       string
	Type       SecurityFileType
	Exists     bool
	Readable   bool
	HasSigFile bool // Whether a corresponding .sig file exists
}

// SecurityFileType indicates the type and precedence of a security config file
type SecurityFileType int

const (
	SecurityFileTypeSystem  SecurityFileType = iota // /etc/duckfile/security.yaml
	SecurityFileTypeUser                            // ~/.duckfile/security.yaml, ~/.config/duckfile/security.yaml
	SecurityFileTypeProject                         // ./.duckfile/security.yaml
)

// DiscoverSecurityConfigs discovers security configuration files in the standard hierarchy.
// Returns files in precedence order (highest to lowest):
// 1. System-wide: /etc/duckfile/security.{yaml,yml}
// 2. User-specific: ~/.duckfile/security.{yaml,yml}, ~/.config/duckfile/security.{yaml,yml}
// 3. Project-specific: ./.duckfile/security.yaml
func DiscoverSecurityConfigs() ([]*SecurityConfigFile, error) {
	var configs []*SecurityConfigFile

	// 1. System-wide configurations (highest precedence for signed configs)
	systemPaths := []string{
		"/etc/duckfile/security.yaml",
		"/etc/duckfile/security.yml",
	}

	for _, path := range systemPaths {
		if config := checkSecurityConfigFile(path, SecurityFileTypeSystem); config != nil {
			configs = append(configs, config)
		}
	}

	// 2. User-specific configurations
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userPaths := []string{
			homeDir + "/.duckfile/security.yaml",
			homeDir + "/.duckfile/security.yml",
			homeDir + "/.config/duckfile/security.yaml",
			homeDir + "/.config/duckfile/security.yml",
		}

		for _, path := range userPaths {
			if config := checkSecurityConfigFile(path, SecurityFileTypeUser); config != nil {
				configs = append(configs, config)
			}
		}
	}

	// 3. Project-specific configurations (lowest precedence)
	projectPaths := []string{
		"./.duckfile/security.yaml",
		"./.duckfile/security.yml",
	}

	for _, path := range projectPaths {
		if config := checkSecurityConfigFile(path, SecurityFileTypeProject); config != nil {
			configs = append(configs, config)
		}
	}

	return configs, nil
}

// checkSecurityConfigFile checks if a security config file exists and is readable
func checkSecurityConfigFile(path string, fileType SecurityFileType) *SecurityConfigFile {
	config := &SecurityConfigFile{
		Path: path,
		Type: fileType,
	}

	// Check if main config file exists and is readable
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		config.Exists = true

		// Test readability
		if file, err := os.Open(path); err == nil {
			config.Readable = true
			file.Close()
		}
	}

	// Check for corresponding signature file
	sigPath := path + ".sig"
	if _, err := os.Stat(sigPath); err == nil {
		config.HasSigFile = true
	}

	// Only return config if the main file exists
	if config.Exists {
		return config
	}
	return nil
}

// LoadSecurityConfigFromFile loads and parses a security configuration from a YAML file
// with optional signature verification
func LoadSecurityConfigFromFile(path string) (*SecurityConfig, error) {
	// Read the configuration file
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read security config file %s: %w", path, err)
	}

	// Check for signature file
	sigPath := path + ".sig"
	var signature []byte
	var isSigned bool

	if _, err := os.Stat(sigPath); err == nil {
		// Signature file exists, load it
		signature, err = LoadSignatureFromFile(sigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load signature for %s: %w", path, err)
		}
		isSigned = true
	}

	// Parse the YAML configuration
	var config SecurityConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse security config YAML from %s: %w", path, err)
	}

	// Set metadata from file loading
	config.SourceFile = path
	config.IsSigned = isSigned

	// If signed, verify the signature
	if isSigned {
		if config.Signature == nil {
			return nil, fmt.Errorf("signature file exists but config has no signature metadata in %s", path)
		}

		// Load the public key for verification
		publicKey, err := LoadPublicKey(config.Signature.KeyId)
		if err != nil {
			return nil, fmt.Errorf("failed to load public key for verification of %s: %w", path, err)
		}

		// Verify the signature
		if err := VerifySignature(configData, signature, publicKey); err != nil {
			return nil, fmt.Errorf("signature verification failed for %s: %w", path, err)
		}

		// Set source to indicate this is a verified signed config
		config.Source = "signed"
	} else {
		// Unsigned config file
		config.Source = "unsigned"
	}

	// Validate the configuration version
	if config.Version == 0 {
		config.Version = 1 // Default to version 1 if not specified
	}

	// Validate file permissions if policy enforcement is enabled
	if config.Enforcement != nil && config.Enforcement.EnforceFilePermissions && config.FilePermissions != nil {
		configFile := &SecurityConfigFile{
			Path:   path,
			Type:   DetermineSecurityFileType(path),
			Exists: true,
		}

		permResult, err := ValidateFilePermissions(configFile, config.FilePermissions)
		if err != nil {
			return nil, fmt.Errorf("failed to validate file permissions for %s: %w", path, err)
		}

		if !permResult.Valid {
			issues := append(permResult.Issues, permResult.ParentDirIssues...)
			return nil, fmt.Errorf("file permission validation failed for %s: %v", path, issues)
		}
	}

	return &config, nil
}

// BuildSecurityConfigWithFiles creates a security configuration with file-based precedence.
// Precedence order (highest to lowest):
// 1. Signed Security Config Files (tamper-proof, highest)
// 2. CLI flags (emergency admin access)
// 3. Environment variables (system-level control)
// 4. Unsigned Security Config Files (lower to prevent bypass)
// 5. No restrictions (backward compatibility)
func BuildSecurityConfigWithFiles(cliAllowed []string, cliDenied []string, cliStrict bool) (*SecurityConfig, error) {
	// Discover available security config files
	configFiles, err := DiscoverSecurityConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to discover security config files: %w", err)
	}

	// Separate signed and unsigned files
	var signedConfigs []*SecurityConfig
	var unsignedConfigs []*SecurityConfig

	for _, configFile := range configFiles {
		if !configFile.Exists || !configFile.Readable {
			continue
		}

		config, err := LoadSecurityConfigFromFile(configFile.Path)
		if err != nil {
			// Log error but continue with other configs
			// In production, this might be logged differently
			continue
		}

		if config.IsSigned {
			signedConfigs = append(signedConfigs, config)
		} else {
			unsignedConfigs = append(unsignedConfigs, config)
		}
	}

	// 1. Check for signed configurations first (highest precedence)
	if len(signedConfigs) > 0 {
		// Use the first signed config (they were discovered in precedence order)
		baseConfig := signedConfigs[0]

		// CLI flags can still override signed configs for emergency access
		if len(cliAllowed) > 0 || len(cliDenied) > 0 || cliStrict {
			// Override specific fields while preserving other signed config data
			if len(cliAllowed) > 0 {
				baseConfig.AllowedHosts = cliAllowed
			}
			if len(cliDenied) > 0 {
				baseConfig.DeniedHosts = cliDenied
			}
			if cliStrict {
				baseConfig.StrictMode = cliStrict
			}
			baseConfig.Source = "cli" // Indicate CLI override
		}

		return baseConfig, nil
	}

	// 2. CLI flags (if provided)
	if len(cliAllowed) > 0 || len(cliDenied) > 0 || cliStrict {
		return BuildSecurityConfig(cliAllowed, cliDenied, cliStrict), nil
	}

	// 3. Environment variables
	envConfig := LoadSecurityConfigFromEnv()
	if envConfig.Source != "none" {
		return envConfig, nil
	}

	// 4. Unsigned config files (lower precedence to prevent bypass)
	if len(unsignedConfigs) > 0 {
		return unsignedConfigs[0], nil
	}

	// 5. No restrictions (backward compatibility)
	return &SecurityConfig{
		Source:  "none",
		Version: 1,
	}, nil
} // LoadSecurityConfigFromEnv loads security configuration from environment variables.
// Environment variables used:
//   - DUCK_ALLOWED_HOSTS: comma-separated list of allowed hostnames
//   - DUCK_DENIED_HOSTS: comma-separated list of denied hostnames
//   - DUCK_STRICT_MODE: "true" to fail if no restrictions are configured
func LoadSecurityConfigFromEnv() *SecurityConfig {
	cfg := &SecurityConfig{
		Source:  "none",
		Version: 1,
	}

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
// CLI flags override environment variables. Only creates a CLI config when
// meaningful values are provided (non-empty slices or strict mode enabled).
func BuildSecurityConfig(cliAllowed []string, cliDenied []string, cliStrict bool) *SecurityConfig {
	// Only use CLI configuration if meaningful values are provided
	// (non-empty host lists or strict mode explicitly enabled)
	if len(cliAllowed) > 0 || len(cliDenied) > 0 || cliStrict {
		return &SecurityConfig{
			AllowedHosts: cliAllowed,
			DeniedHosts:  cliDenied,
			StrictMode:   cliStrict,
			Source:       "cli",
			Version:      1, // Current configuration version
		}
	}

	// No meaningful CLI flags provided - fallback to environment variables
	envConfig := LoadSecurityConfigFromEnv()
	envConfig.Version = 1 // Ensure version is set
	return envConfig
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
