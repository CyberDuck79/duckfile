package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// TestSecurityIntegrationBasicWorkflow tests the basic security workflow
// without trying to mock internal implementation details
func TestSecurityIntegrationBasicWorkflow(t *testing.T) {
	// Create temporary directory for this test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Step 1: Generate key pair for signing
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Step 2: Create a security configuration
	configContent := `version: 1
allowedHosts:
  - github.com
  - gitlab.internal.com
deniedHosts:
  - malicious-host.com
strictMode: true
`

	// Create .duckfile directory and config file
	duckfileDir := filepath.Join(tmpDir, ".duckfile")
	if err := os.MkdirAll(duckfileDir, 0700); err != nil {
		t.Fatalf("failed to create .duckfile directory: %v", err)
	}

	configPath := filepath.Join(duckfileDir, "security.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Step 3: Sign the configuration
	signature, err := SignConfig([]byte(configContent), keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("failed to sign config: %v", err)
	}

	sigPath := configPath + ".sig"
	signatureData := base64.StdEncoding.EncodeToString(signature)
	if err := os.WriteFile(sigPath, []byte(signatureData), 0600); err != nil {
		t.Fatalf("failed to write signature file: %v", err)
	}

	// Step 4: Save public key for verification
	pubKeyDir := filepath.Join(tmpDir, "keys")
	if err := os.MkdirAll(pubKeyDir, 0700); err != nil {
		t.Fatalf("failed to create keys directory: %v", err)
	}

	pubKeyPath := filepath.Join(pubKeyDir, keyPair.KeyId+".pub")
	pubKeyData := base64.StdEncoding.EncodeToString(keyPair.PublicKey)
	if err := os.WriteFile(pubKeyPath, []byte(pubKeyData), 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	// Step 5: Test precedence system without signed config first
	config, err := BuildSecurityConfigWithPrecedence([]string{"example.com"}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build security config: %v", err)
	}

	// Without signed config, should use CLI parameters
	if config.Source != "cli" {
		t.Errorf("expected CLI source without signed config, got: %s", config.Source)
	}

	// Step 6: Test file permission validation
	configFile := &SecurityConfigFile{
		Path:       configPath,
		Exists:     true,
		Readable:   true,
		HasSigFile: true,
		Type:       SecurityFileTypeProject,
	}

	policy := &FilePermissionPolicy{
		EnforceOwnership:         false, // Skip ownership for test
		EnforceReadOnly:          false, // Allow writes for config files
		AllowGroupWrite:          false,
		RequireSecureDirectories: true,
	}

	result, err := ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("failed to validate file permissions: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected file permissions to be valid, issues: %v", result.Issues)
	}

	t.Logf("✅ Basic security integration workflow completed successfully")
}

// TestSecurityPrecedenceSystemIntegration tests the precedence system with real files
func TestSecurityPrecedenceSystemIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Test 1: No configuration - should use default
	config, err := BuildSecurityConfigWithPrecedence([]string{}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build default config: %v", err)
	}
	if config.Source != "none" {
		t.Errorf("expected 'none' source with no config, got %s", config.Source)
	}

	// Test 2: CLI flags only
	config, err = BuildSecurityConfigWithPrecedence([]string{"cli-host.com"}, []string{}, true)
	if err != nil {
		t.Fatalf("failed to build CLI config: %v", err)
	}
	if config.Source != "cli" {
		t.Errorf("expected 'cli' source with CLI flags, got %s", config.Source)
	}
	if !stringSliceEqual(config.AllowedHosts, []string{"cli-host.com"}) {
		t.Errorf("expected CLI allowed hosts, got %v", config.AllowedHosts)
	}

	// Test 3: Environment variables
	os.Setenv("DUCK_ALLOWED_HOSTS", "env-host.com,env-host2.com")
	os.Setenv("DUCK_STRICT_MODE", "true")
	defer func() {
		os.Unsetenv("DUCK_ALLOWED_HOSTS")
		os.Unsetenv("DUCK_STRICT_MODE")
	}()

	config, err = BuildSecurityConfigWithPrecedence([]string{}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build env config: %v", err)
	}
	if config.Source != "env" {
		t.Errorf("expected 'env' source with env vars, got %s", config.Source)
	}
	expectedEnvHosts := []string{"env-host.com", "env-host2.com"}
	if !stringSliceEqual(config.AllowedHosts, expectedEnvHosts) {
		t.Errorf("expected env allowed hosts %v, got %v", expectedEnvHosts, config.AllowedHosts)
	}

	// Test 4: CLI flags should override environment variables
	config, err = BuildSecurityConfigWithPrecedence([]string{"cli-override.com"}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build CLI override config: %v", err)
	}
	if config.Source != "cli" {
		t.Errorf("expected 'cli' source to override env, got %s", config.Source)
	}
	if !stringSliceEqual(config.AllowedHosts, []string{"cli-override.com"}) {
		t.Errorf("expected CLI to override env hosts, got %v", config.AllowedHosts)
	}

	// Test 5: Create unsigned config file - should override env but not CLI
	duckfileDir := filepath.Join(tmpDir, ".duckfile")
	os.MkdirAll(duckfileDir, 0700)

	unsignedConfig := `version: 1
allowedHosts:
  - unsigned-host.com
strictMode: false
`
	configPath := filepath.Join(duckfileDir, "security.yaml")
	os.WriteFile(configPath, []byte(unsignedConfig), 0600)

	// Clear environment variables for this test
	os.Unsetenv("DUCK_ALLOWED_HOSTS")
	os.Unsetenv("DUCK_STRICT_MODE")

	// Without CLI flags, should use unsigned config
	config, err = BuildSecurityConfigWithPrecedence([]string{}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build unsigned config: %v", err)
	}
	if config.Source != "unsigned" {
		t.Errorf("expected 'unsigned' source with config file, got %s", config.Source)
	}
	if !stringSliceEqual(config.AllowedHosts, []string{"unsigned-host.com"}) {
		t.Errorf("expected unsigned config hosts, got %v", config.AllowedHosts)
	}

	// With CLI flags, should still use CLI
	config, err = BuildSecurityConfigWithPrecedence([]string{"cli-beats-unsigned.com"}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build CLI config with unsigned present: %v", err)
	}
	if config.Source != "cli" {
		t.Errorf("expected CLI to override unsigned config, got %s", config.Source)
	}

	t.Logf("✅ Security precedence system integration test completed successfully")
}

// TestSecurityHostValidation tests the repository URL validation system
func TestSecurityHostValidation(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create a security configuration with specific rules
	securityConfig := &SecurityConfig{
		AllowedHosts: []string{"github.com", "gitlab.internal.com"},
		DeniedHosts:  []string{"evil-host.com", "malicious-site.com"},
		StrictMode:   true,
		Source:       "test",
	}

	// Test 1: Allowed host should pass
	if err := ValidateRepoAccess("https://github.com/test/repo.git", securityConfig); err != nil {
		t.Errorf("expected github.com to be allowed, got error: %v", err)
	}

	// Test 2: Denied host should fail
	if err := ValidateRepoAccess("https://evil-host.com/test/repo.git", securityConfig); err == nil {
		t.Errorf("expected evil-host.com to be denied")
	}

	// Test 3: Unknown host should be denied in strict mode
	if err := ValidateRepoAccess("https://unknown-host.com/test/repo.git", securityConfig); err == nil {
		t.Errorf("expected unknown host to be denied in strict mode")
	}

	// Test 4: Permissive mode with no allowed hosts should allow unknown hosts
	permissiveConfig := &SecurityConfig{
		AllowedHosts: []string{}, // Empty allowed hosts list
		DeniedHosts:  []string{"evil-host.com"},
		StrictMode:   false, // Permissive mode
		Source:       "test",
	}

	if err := ValidateRepoAccess("https://unknown-host.com/test/repo.git", permissiveConfig); err != nil {
		t.Errorf("expected unknown host to be allowed in permissive mode with no allow list, got: %v", err)
	}

	// Test 5: Denied host should still be denied in permissive mode
	if err := ValidateRepoAccess("https://evil-host.com/test/repo.git", permissiveConfig); err == nil {
		t.Errorf("expected denied host to be blocked even in permissive mode")
	}

	t.Logf("✅ Security host validation test completed successfully")
}

// TestSecurityFileDiscovery tests the security file discovery system
func TestSecurityFileDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create various security configuration files
	testFiles := map[string]string{
		".duckfile/security.yaml": `version: 1
allowedHosts: [github.com]
`,
		".duckfile/security.yml": `version: 1
allowedHosts: [gitlab.com]
`,
	}

	for path, content := range testFiles {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	// Test file discovery
	configs, err := DiscoverSecurityConfigs()
	if err != nil {
		t.Fatalf("failed to discover security configs: %v", err)
	}

	foundFiles := 0
	for _, cfg := range configs {
		if cfg.Exists && cfg.Readable {
			foundFiles++
			t.Logf("Found config: %s (type: %v)", cfg.Path, cfg.Type)
		}
	}

	if foundFiles == 0 {
		t.Errorf("expected to find at least one configuration file")
	}

	t.Logf("✅ Security file discovery test completed successfully")
}

// TestSecurityFilePermissions tests file permission validation
func TestSecurityFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create test file structure
	duckfileDir := filepath.Join(tmpDir, ".duckfile")
	os.MkdirAll(duckfileDir, 0700) // Secure directory permissions

	configPath := filepath.Join(duckfileDir, "security.yaml")
	configContent := `version: 1
allowedHosts: [github.com]
`
	os.WriteFile(configPath, []byte(configContent), 0600) // Secure file permissions

	// Test 1: Valid permissions
	configFile := &SecurityConfigFile{
		Path:       configPath,
		Exists:     true,
		Readable:   true,
		HasSigFile: false,
		Type:       SecurityFileTypeProject,
	}

	policy := &FilePermissionPolicy{
		EnforceOwnership:         false, // Skip ownership checks in test
		EnforceReadOnly:          false, // Don't require read-only for config files
		AllowGroupWrite:          false,
		RequireSecureDirectories: true,
	}

	result, err := ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("failed to validate file permissions: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid permissions, got issues: %v", result.Issues)
	}

	// Test 2: World-writable file (should always be invalid)
	os.Chmod(configPath, 0666) // World writable

	result, err = ValidateFilePermissions(configFile, policy)
	if err != nil {
		t.Fatalf("failed to validate file permissions: %v", err)
	}

	if result.Valid {
		t.Errorf("expected world-writable file to be invalid")
	}

	t.Logf("✅ Security file permissions test completed successfully")
}

// TestSecurityPolicyEnforcement tests policy enforcement
func TestSecurityPolicyEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create configuration with enforcement policies
	strictConfig := `version: 1
allowedHosts:
  - github.com
deniedHosts:
  - evil-host.com
strictMode: true
enforcement:
  forceChecksumValidation: true
  forceCommitTracking: true
  disableAutoUpdate: true
  strictPolicyMode: true
  enforceFilePermissions: true
`

	// Create and save config
	duckfileDir := filepath.Join(tmpDir, ".duckfile")
	os.MkdirAll(duckfileDir, 0700)
	configPath := filepath.Join(duckfileDir, "security.yaml")
	os.WriteFile(configPath, []byte(strictConfig), 0600)

	// Load the security configuration
	securityConfig, err := BuildSecurityConfigWithPrecedence([]string{}, []string{}, false)
	if err != nil {
		t.Fatalf("failed to build security config: %v", err)
	}

	// Test 1: Host validation enforcement
	if !securityConfig.StrictMode {
		t.Errorf("expected strict mode to be enforced")
	}

	// Test allowed host
	if err := ValidateRepoAccess("https://github.com/test/repo.git", securityConfig); err != nil {
		t.Errorf("expected github.com to be allowed, got error: %v", err)
	}

	// Test denied host
	if err := ValidateRepoAccess("https://evil-host.com/test/repo.git", securityConfig); err == nil {
		t.Errorf("expected evil-host.com to be denied")
	}

	// Test 2: Policy enforcement flags (if enforcement is loaded)
	if securityConfig.Enforcement != nil {
		enforcement := securityConfig.Enforcement
		if !enforcement.ForceChecksumValidation {
			t.Errorf("expected forceChecksumValidation to be true")
		}
		if !enforcement.StrictPolicyMode {
			t.Errorf("expected strictPolicyMode to be true")
		}
	}

	t.Logf("✅ Security policy enforcement test completed successfully")
}

// Helper function to compare string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
