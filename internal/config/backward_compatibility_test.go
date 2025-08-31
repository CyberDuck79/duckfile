package config

import (
	"testing"
)

// TestBackwardCompatibility ensures that existing functionality continues to work
// with the enhanced SecurityConfig structure
func TestBackwardCompatibility(t *testing.T) {
	// Test that existing CLI flag behavior is preserved
	config := BuildSecurityConfig([]string{"github.com"}, []string{"bad.com"}, true)

	if config == nil {
		t.Fatal("BuildSecurityConfig returned nil")
	}

	// Verify existing fields work as before
	if len(config.AllowedHosts) != 1 || config.AllowedHosts[0] != "github.com" {
		t.Errorf("AllowedHosts not preserved: got %v", config.AllowedHosts)
	}

	if len(config.DeniedHosts) != 1 || config.DeniedHosts[0] != "bad.com" {
		t.Errorf("DeniedHosts not preserved: got %v", config.DeniedHosts)
	}

	if !config.StrictMode {
		t.Error("StrictMode not preserved")
	}

	if config.Source != "cli" {
		t.Errorf("Expected source 'cli', got %s", config.Source)
	}

	// Verify new fields have sensible defaults
	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}

	if config.IsSigned {
		t.Error("New config should not be signed by default")
	}

	if config.SourceFile != "" {
		t.Errorf("SourceFile should be empty for CLI config, got %s", config.SourceFile)
	}

	// Verify new structs are nil by default (backward compatibility)
	if config.Signature != nil {
		t.Error("Signature should be nil by default")
	}

	if config.Enforcement != nil {
		t.Error("Enforcement should be nil by default")
	}

	if config.FilePermissions != nil {
		t.Error("FilePermissions should be nil by default")
	}

	if config.Metadata != nil {
		t.Error("Metadata should be nil by default")
	}
}

// TestEnvironmentVariableCompatibility tests that environment variables still work
func TestEnvironmentVariableCompatibility(t *testing.T) {
	// Test BuildSecurityConfigWithFiles falls back to existing behavior
	config, err := BuildSecurityConfigWithFiles(nil, nil, false)
	if err != nil {
		t.Fatalf("BuildSecurityConfigWithFiles failed: %v", err)
	}

	if config == nil {
		t.Fatal("BuildSecurityConfigWithFiles returned nil")
	}

	// Should fall back to env config when no CLI flags provided
	if config.Source != "none" && config.Source != "env" {
		t.Errorf("Expected source 'none' or 'env', got %s", config.Source)
	}

	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}
}

// TestValidateRepoAccessCompatibility ensures existing validation still works
func TestValidateRepoAccessCompatibility(t *testing.T) {
	config := &SecurityConfig{
		AllowedHosts: []string{"github.com"},
		DeniedHosts:  []string{"malicious.com"},
		StrictMode:   false,
		Source:       "test",
		Version:      1,
	}

	// Test allowed host (should pass)
	err := ValidateRepoAccess("https://github.com/user/repo.git", config)
	if err != nil {
		t.Errorf("Expected allowed host to pass validation, got: %v", err)
	}

	// Test denied host (should fail)
	err = ValidateRepoAccess("https://malicious.com/user/repo.git", config)
	if err == nil {
		t.Error("Expected denied host to fail validation")
	}

	// Test host not in allow list (should fail)
	err = ValidateRepoAccess("https://unknown.com/user/repo.git", config)
	if err == nil {
		t.Error("Expected unknown host to fail validation when allow list is configured")
	}
}
