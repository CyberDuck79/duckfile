package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	if keyPair == nil {
		t.Fatal("Expected non-nil key pair")
	}

	if len(keyPair.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("Expected public key size %d, got %d", ed25519.PublicKeySize, len(keyPair.PublicKey))
	}

	if len(keyPair.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("Expected private key size %d, got %d", ed25519.PrivateKeySize, len(keyPair.PrivateKey))
	}

	if keyPair.KeyId == "" {
		t.Error("Expected non-empty key ID")
	}

	// Verify the key pair works by signing and verifying
	testData := []byte("test message")
	signature, err := SignConfig(testData, keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign test data: %v", err)
	}

	err = VerifySignature(testData, signature, keyPair.PublicKey)
	if err != nil {
		t.Fatalf("Failed to verify test signature: %v", err)
	}
}

func TestSignConfig(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	testData := []byte("test configuration data")

	signature, err := SignConfig(testData, keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("SignConfig() failed: %v", err)
	}

	if len(signature) != ed25519.SignatureSize {
		t.Errorf("Expected signature size %d, got %d", ed25519.SignatureSize, len(signature))
	}

	// Test with invalid private key
	invalidKey := make([]byte, 10) // Wrong size
	_, err = SignConfig(testData, invalidKey)
	if err == nil {
		t.Error("Expected error with invalid private key")
	}
}

func TestVerifySignature(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	testData := []byte("test configuration data")
	signature, err := SignConfig(testData, keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign test data: %v", err)
	}

	// Test valid signature
	err = VerifySignature(testData, signature, keyPair.PublicKey)
	if err != nil {
		t.Errorf("Valid signature verification failed: %v", err)
	}

	// Test invalid signature (tampered data)
	tamperedData := []byte("tampered configuration data")
	err = VerifySignature(tamperedData, signature, keyPair.PublicKey)
	if err == nil {
		t.Error("Expected verification to fail with tampered data")
	}

	// Test invalid signature (wrong signature)
	wrongSignature := make([]byte, ed25519.SignatureSize)
	err = VerifySignature(testData, wrongSignature, keyPair.PublicKey)
	if err == nil {
		t.Error("Expected verification to fail with wrong signature")
	}

	// Test invalid public key
	invalidKey := make([]byte, 10) // Wrong size
	err = VerifySignature(testData, signature, invalidKey)
	if err == nil {
		t.Error("Expected error with invalid public key")
	}

	// Test invalid signature size
	invalidSignature := make([]byte, 10) // Wrong size
	err = VerifySignature(testData, invalidSignature, keyPair.PublicKey)
	if err == nil {
		t.Error("Expected error with invalid signature size")
	}
}

func TestSaveAndLoadKeyPair(t *testing.T) {
	tempDir := t.TempDir()

	// Generate a key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save the key pair
	err = SaveKeyPair(keyPair, tempDir)
	if err != nil {
		t.Fatalf("SaveKeyPair() failed: %v", err)
	}

	// Verify files were created
	privateKeyPath := filepath.Join(tempDir, keyPair.KeyId+"-private.key")
	publicKeyPath := filepath.Join(tempDir, keyPair.KeyId+".pub")

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		t.Error("Private key file was not created")
	}

	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		t.Error("Public key file was not created")
	}

	// Load private key
	loadedPrivateKey, err := LoadPrivateKey(privateKeyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey() failed: %v", err)
	}

	if !loadedPrivateKey.Equal(keyPair.PrivateKey) {
		t.Error("Loaded private key does not match original")
	}

	// Load public key
	loadedPublicKey, err := loadPublicKeyFromFile(publicKeyPath)
	if err != nil {
		t.Fatalf("loadPublicKeyFromFile() failed: %v", err)
	}

	if !loadedPublicKey.Equal(keyPair.PublicKey) {
		t.Error("Loaded public key does not match original")
	}
}

func TestLoadPublicKey(t *testing.T) {
	tempDir := t.TempDir()

	// Generate a key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create a keys directory
	keysDir := filepath.Join(tempDir, "keys")
	err = os.MkdirAll(keysDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create keys directory: %v", err)
	}

	// Save public key with standard naming
	publicKeyPath := filepath.Join(keysDir, keyPair.KeyId+".pub")
	publicKeyData := base64.StdEncoding.EncodeToString(keyPair.PublicKey)
	err = os.WriteFile(publicKeyPath, []byte(publicKeyData), 0644)
	if err != nil {
		t.Fatalf("Failed to write public key: %v", err)
	}

	// Set environment variable to point to our test keys directory
	oldKeysPath := os.Getenv("DUCK_KEYS_PATH")
	defer os.Setenv("DUCK_KEYS_PATH", oldKeysPath)
	os.Setenv("DUCK_KEYS_PATH", keysDir)

	// Load the public key
	loadedPublicKey, err := LoadPublicKey(keyPair.KeyId)
	if err != nil {
		t.Fatalf("LoadPublicKey() failed: %v", err)
	}

	if !loadedPublicKey.Equal(keyPair.PublicKey) {
		t.Error("Loaded public key does not match original")
	}

	// Test loading non-existent key
	_, err = LoadPublicKey("non-existent-key")
	if err == nil {
		t.Error("Expected error when loading non-existent key")
	}
}

func TestSignatureFileOperations(t *testing.T) {
	tempDir := t.TempDir()

	// Generate a signature
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	testData := []byte("test data")
	signature, err := SignConfig(testData, keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign test data: %v", err)
	}

	// Save signature to file
	sigPath := filepath.Join(tempDir, "test.sig")
	err = SaveSignatureToFile(signature, sigPath)
	if err != nil {
		t.Fatalf("SaveSignatureToFile() failed: %v", err)
	}

	// Load signature from file
	loadedSignature, err := LoadSignatureFromFile(sigPath)
	if err != nil {
		t.Fatalf("LoadSignatureFromFile() failed: %v", err)
	}

	if len(signature) != len(loadedSignature) {
		t.Errorf("Signature length mismatch: expected %d, got %d", len(signature), len(loadedSignature))
	}

	for i := range signature {
		if signature[i] != loadedSignature[i] {
			t.Error("Loaded signature does not match original")
			break
		}
	}

	// Verify the loaded signature still works
	err = VerifySignature(testData, loadedSignature, keyPair.PublicKey)
	if err != nil {
		t.Fatalf("Verification failed with loaded signature: %v", err)
	}
}

func TestKeyDiscoveryPaths(t *testing.T) {
	// Test that getKeyDiscoveryPaths returns reasonable paths
	paths := getKeyDiscoveryPaths()

	if len(paths) == 0 {
		t.Error("Expected at least one key discovery path")
	}

	// Should include some standard paths
	hasSystemPath := false
	hasUserPath := false
	hasCurrentPath := false

	for _, path := range paths {
		if path == "/etc/duckfile/keys" {
			hasSystemPath = true
		}
		if strings.Contains(path, ".duckfile/keys") {
			hasUserPath = true
		}
		if path == "./keys" || path == "./.duckfile/keys" {
			hasCurrentPath = true
		}
	}

	if !hasSystemPath {
		t.Error("Expected system keys path")
	}
	if !hasUserPath {
		t.Error("Expected user keys path")
	}
	if !hasCurrentPath {
		t.Error("Expected current directory keys path")
	}

	// Test environment variable override
	oldKeysPath := os.Getenv("DUCK_KEYS_PATH")
	defer os.Setenv("DUCK_KEYS_PATH", oldKeysPath)

	testPath := "/custom/keys/path"
	os.Setenv("DUCK_KEYS_PATH", testPath)

	pathsWithEnv := getKeyDiscoveryPaths()
	if len(pathsWithEnv) == 0 || pathsWithEnv[0] != testPath {
		t.Errorf("Expected environment variable path %s to be first, got paths: %v", testPath, pathsWithEnv)
	}
}
