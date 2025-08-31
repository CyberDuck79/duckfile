package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSignedConfigurationWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	// Step 1: Generate a key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Step 2: Create keys directory and save keys
	keysDir := filepath.Join(tempDir, "keys")
	err = SaveKeyPair(keyPair, keysDir)
	if err != nil {
		t.Fatalf("Failed to save key pair: %v", err)
	}

	// Step 3: Create a security configuration
	configContent := fmt.Sprintf(`version: 1
allowedHosts:
  - github.com
  - internal.company.com
deniedHosts:
  - malicious.com
strictMode: true

signature:
  algorithm: "Ed25519"
  keyId: "%s"

enforcement:
  forceChecksumValidation: true
  strictPolicyMode: true

filePermissions:
  enforceOwnership: true
  enforceReadOnly: true

metadata:
  createdBy: "security-team"
  purpose: "Production security policy"
  version: 1
`, keyPair.KeyID)

	configPath := filepath.Join(tempDir, "security.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Step 4: Sign the configuration
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config for signing: %v", err)
	}

	signature, err := SignConfig(configData, keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign config: %v", err)
	}

	// Step 5: Save the signature file
	sigPath := configPath + ".sig"
	err = SaveSignatureToFile(signature, sigPath)
	if err != nil {
		t.Fatalf("Failed to save signature: %v", err)
	}

	// Step 6: Set up environment for key discovery
	oldKeysPath := os.Getenv("DUCK_KEYS_PATH")
	defer os.Setenv("DUCK_KEYS_PATH", oldKeysPath)
	os.Setenv("DUCK_KEYS_PATH", keysDir)

	// Step 7: Load and verify the signed configuration
	loadedConfig, err := LoadSecurityConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load signed config: %v", err)
	}

	// Step 8: Verify the loaded configuration
	if loadedConfig == nil {
		t.Fatal("Expected non-nil config")
	}

	// Check basic fields
	if len(loadedConfig.AllowedHosts) != 2 {
		t.Errorf("Expected 2 allowed hosts, got %d", len(loadedConfig.AllowedHosts))
	}

	expectedHosts := []string{"github.com", "internal.company.com"}
	for i, expected := range expectedHosts {
		if i >= len(loadedConfig.AllowedHosts) || loadedConfig.AllowedHosts[i] != expected {
			t.Errorf("Expected allowed host %s at index %d, got %v", expected, i, loadedConfig.AllowedHosts)
		}
	}

	if len(loadedConfig.DeniedHosts) != 1 || loadedConfig.DeniedHosts[0] != "malicious.com" {
		t.Errorf("Expected denied host 'malicious.com', got %v", loadedConfig.DeniedHosts)
	}

	if !loadedConfig.StrictMode {
		t.Error("Expected strict mode to be true")
	}

	// Check signature information
	if loadedConfig.Signature == nil {
		t.Fatal("Expected signature information")
	}

	if loadedConfig.Signature.Algorithm != "Ed25519" {
		t.Errorf("Expected algorithm 'Ed25519', got '%s'", loadedConfig.Signature.Algorithm)
	}

	if loadedConfig.Signature.KeyID != keyPair.KeyID {
		t.Errorf("Expected key ID '%s', got '%s'", keyPair.KeyID, loadedConfig.Signature.KeyID)
	}

	// Check enforcement settings
	if loadedConfig.Enforcement == nil {
		t.Fatal("Expected enforcement settings")
	}

	if !loadedConfig.Enforcement.ForceChecksumValidation {
		t.Error("Expected force checksum validation to be true")
	}

	if !loadedConfig.Enforcement.StrictPolicyMode {
		t.Error("Expected strict policy mode to be true")
	}

	// Check file permissions
	if loadedConfig.FilePermissions == nil {
		t.Fatal("Expected file permissions settings")
	}

	if !loadedConfig.FilePermissions.EnforceOwnership {
		t.Error("Expected enforce ownership to be true")
	}

	if !loadedConfig.FilePermissions.EnforceReadOnly {
		t.Error("Expected enforce read-only to be true")
	}

	// Check metadata
	if loadedConfig.Metadata == nil {
		t.Fatal("Expected metadata")
	}

	if loadedConfig.Metadata.CreatedBy != "security-team" {
		t.Errorf("Expected created by 'security-team', got '%s'", loadedConfig.Metadata.CreatedBy)
	}

	if loadedConfig.Metadata.Purpose != "Production security policy" {
		t.Errorf("Expected purpose 'Production security policy', got '%s'", loadedConfig.Metadata.Purpose)
	}

	// Check that the config knows it was loaded from a file and is signed
	if loadedConfig.SourceFile != configPath {
		t.Errorf("Expected source file '%s', got '%s'", configPath, loadedConfig.SourceFile)
	}

	if !loadedConfig.IsSigned {
		t.Error("Expected config to be marked as signed")
	}

	if loadedConfig.Source != "signed" {
		t.Errorf("Expected source 'signed', got '%s'", loadedConfig.Source)
	}

	if loadedConfig.Version != 1 {
		t.Errorf("Expected version 1, got %d", loadedConfig.Version)
	}
}

func TestUnsignedConfigurationWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	// Create an unsigned configuration
	configContent := `version: 1
allowedHosts:
  - github.com
strictMode: false

enforcement:
  forceChecksumValidation: false
  strictPolicyMode: false
`

	configPath := filepath.Join(tempDir, "security.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the unsigned configuration
	loadedConfig, err := LoadSecurityConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load unsigned config: %v", err)
	}

	// Verify the config is properly loaded but marked as unsigned
	if loadedConfig == nil {
		t.Fatal("Expected non-nil config")
	}

	if len(loadedConfig.AllowedHosts) != 1 || loadedConfig.AllowedHosts[0] != "github.com" {
		t.Errorf("Expected allowed host 'github.com', got %v", loadedConfig.AllowedHosts)
	}

	if loadedConfig.StrictMode {
		t.Error("Expected strict mode to be false")
	}

	// Should be marked as unsigned
	if loadedConfig.IsSigned {
		t.Error("Expected config to be marked as unsigned")
	}

	if loadedConfig.Source != "unsigned" {
		t.Errorf("Expected source 'unsigned', got '%s'", loadedConfig.Source)
	}

	if loadedConfig.SourceFile != configPath {
		t.Errorf("Expected source file '%s', got '%s'", configPath, loadedConfig.SourceFile)
	}
}

func TestSignatureVerificationFailure(t *testing.T) {
	tempDir := t.TempDir()

	// Generate a key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create and save keys
	keysDir := filepath.Join(tempDir, "keys")
	err = SaveKeyPair(keyPair, keysDir)
	if err != nil {
		t.Fatalf("Failed to save key pair: %v", err)
	}

	// Create a configuration
	configContent := fmt.Sprintf(`version: 1
allowedHosts:
  - github.com
signature:
  algorithm: "Ed25519"
  keyId: "%s"
`, keyPair.KeyID)

	configPath := filepath.Join(tempDir, "security.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create a tampered signature file (properly sized but invalid signature)
	sigPath := configPath + ".sig"
	// Create a 64-byte signature that's invalid
	invalidSignature := make([]byte, 64)
	for i := range invalidSignature {
		invalidSignature[i] = byte(i % 256)
	}
	err = SaveSignatureToFile(invalidSignature, sigPath)
	if err != nil {
		t.Fatalf("Failed to write tampered signature: %v", err)
	}

	// Set up environment for key discovery
	oldKeysPath := os.Getenv("DUCK_KEYS_PATH")
	defer os.Setenv("DUCK_KEYS_PATH", oldKeysPath)
	os.Setenv("DUCK_KEYS_PATH", keysDir)

	// Attempt to load - should fail due to invalid signature
	_, err = LoadSecurityConfigFromFile(configPath)
	if err == nil {
		t.Fatal("Expected error due to signature verification failure")
	}

	if !stringContains(err.Error(), "signature verification failed") {
		t.Errorf("Expected signature verification error, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
