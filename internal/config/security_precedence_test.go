//nolint:errcheck
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestMergeSecurityConfigs validates merging precedence behavior.
func TestMergeSecurityConfigs(t *testing.T) {
	base := &config.SecurityConfig{
		AllowedHosts: []string{"a.com"},
		DeniedHosts:  []string{"x.com"},
		StrictMode:   false,
		Source:       "unsigned",
		SourceFile:   "base.yaml",
		IsSigned:     false,
		Version:      1,
	}
	override := &config.SecurityConfig{
		AllowedHosts: []string{"b.com", "c.com"},
		StrictMode:   true,
		Source:       "cli",
		SourceFile:   "cli-flags",
		IsSigned:     true,
		Version:      2,
	}
	merged := config.MergeSecurityConfigs(base, override)
	if len(merged.AllowedHosts) != 2 || merged.AllowedHosts[0] != "b.com" {
		t.Fatalf("allowed hosts not replaced: %+v", merged.AllowedHosts)
	}
	if len(merged.DeniedHosts) != 1 || merged.DeniedHosts[0] != "x.com" {
		t.Fatalf("denied hosts unexpectedly changed: %+v", merged.DeniedHosts)
	}
	if !merged.StrictMode {
		t.Fatalf("StrictMode not overridden")
	}
	if merged.Source != "cli" {
		t.Fatalf("Source not updated: %s", merged.Source)
	}
	if merged.SourceFile != "cli-flags" {
		t.Fatalf("SourceFile not updated: %s", merged.SourceFile)
	}
	if !merged.IsSigned {
		t.Fatalf("IsSigned not updated")
	}
	if merged.Version != 2 {
		t.Fatalf("Version not updated: %d", merged.Version)
	}
}

// TestGetSecurityConfigPrecedenceInfo exercises unsigned -> env -> cli -> signed precedence.
func TestGetSecurityConfigPrecedenceInfo(t *testing.T) {
	tmp := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", oldHome)

	// Create project unsigned config
	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(filepath.Join(projectDir, ".duckfile"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	unsignedPath := filepath.Join(projectDir, ".duckfile", "security.yaml")
	if err := os.WriteFile(unsignedPath, []byte("version: 1\nallowedHosts:\n  - unsigned.com\n"), 0o644); err != nil {
		t.Fatalf("write unsigned config: %v", err)
	}
	// Work from projectDir so discovery finds it
	oldWd, _ := os.Getwd()
	os.Chdir(projectDir)
	defer os.Chdir(oldWd)

	info, err := config.GetSecurityConfigPrecedenceInfo(nil, nil, false)
	if err != nil {
		t.Fatalf("precedence info error: %v", err)
	}
	if info.EffectiveSource != "unsigned" {
		t.Fatalf("expected unsigned, got %s", info.EffectiveSource)
	}

	// Env overrides unsigned
	os.Setenv("DUCK_ALLOWED_HOSTS", "env.com")
	info, err = config.GetSecurityConfigPrecedenceInfo(nil, nil, false)
	if err != nil {
		t.Fatalf("precedence env error: %v", err)
	}
	if info.EffectiveSource != "env" {
		t.Fatalf("expected env, got %s", info.EffectiveSource)
	}
	os.Unsetenv("DUCK_ALLOWED_HOSTS")

	// CLI overrides env/unsigned
	info, err = config.GetSecurityConfigPrecedenceInfo([]string{"cli.com"}, nil, false)
	if err != nil {
		t.Fatalf("precedence cli error: %v", err)
	}
	if info.EffectiveSource != "cli" {
		t.Fatalf("expected cli, got %s", info.EffectiveSource)
	}

	// Signed config (highest)
	signedDir := filepath.Join(tmp, ".duckfile")
	if err := os.MkdirAll(signedDir, 0o755); err != nil {
		t.Fatalf("mkdir signedDir: %v", err)
	}
	kp, err := config.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	signedPath := filepath.Join(signedDir, "security.yaml")
	signedContent := []byte("version: 1\nsignature:\n  algorithm: ed25519\n  keyId: " + kp.KeyID + "\n  signature: placeholder\nallowedHosts:\n  - signed.com\n")
	if err := os.WriteFile(signedPath, signedContent, 0o644); err != nil {
		t.Fatalf("write signed config: %v", err)
	}
	sig, err := config.SignConfig(signedContent, kp.PrivateKey)
	if err != nil {
		t.Fatalf("sign content: %v", err)
	}
	if err := os.WriteFile(signedPath+".sig", sig, 0o644); err != nil {
		t.Fatalf("write sig: %v", err)
	}
	keyDir := filepath.Join(tmp, ".duckfile", "keys")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatalf("mkdir keydir: %v", err)
	}
	if err := config.SaveKeyPair(kp, keyDir); err != nil {
		t.Fatalf("save key pair: %v", err)
	}

	info, err = config.GetSecurityConfigPrecedenceInfo(nil, nil, false)
	if err != nil {
		t.Fatalf("precedence signed error: %v", err)
	}
	if info.EffectiveSource != "signed" {
		t.Fatalf("expected signed, got %s", info.EffectiveSource)
	}
}
