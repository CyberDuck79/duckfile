package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SignatureKeyPair represents an Ed25519 key pair
type SignatureKeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	KeyId      string
}

// GenerateKeyPair generates a new Ed25519 key pair for signing security configurations
func GenerateKeyPair() (*SignatureKeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	
	// Generate a simple key ID based on the first 8 bytes of the public key
	keyId := fmt.Sprintf("key-%x", publicKey[:8])
	
	return &SignatureKeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		KeyId:      keyId,
	}, nil
}

// SignConfig signs the provided configuration data using Ed25519
func SignConfig(configData []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d bytes, got %d", ed25519.PrivateKeySize, len(privateKey))
	}
	
	signature := ed25519.Sign(privateKey, configData)
	return signature, nil
}

// VerifySignature verifies a signature against the provided data and public key
func VerifySignature(configData, signature []byte, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(publicKey))
	}
	
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size: expected %d bytes, got %d", ed25519.SignatureSize, len(signature))
	}
	
	if !ed25519.Verify(publicKey, configData, signature) {
		return fmt.Errorf("signature verification failed")
	}
	
	return nil
}

// LoadPrivateKey loads a private key from a file
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %s: %w", path, err)
	}
	
	// Try to decode as base64 first
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		// If base64 decoding fails, treat as raw bytes
		decoded = data
	}
	
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size in file %s: expected %d bytes, got %d", path, ed25519.PrivateKeySize, len(decoded))
	}
	
	return ed25519.PrivateKey(decoded), nil
}

// LoadPublicKey loads a public key by key ID using the key discovery mechanism
func LoadPublicKey(keyId string) (ed25519.PublicKey, error) {
	// Try to find the public key in standard locations
	keyPaths := getKeyDiscoveryPaths()
	
	for _, basePath := range keyPaths {
		// Try different naming conventions
		possibleNames := []string{
			fmt.Sprintf("%s.pub", keyId),
			fmt.Sprintf("%s-public.key", keyId),
			fmt.Sprintf("public-%s.key", keyId),
			fmt.Sprintf("%s.public", keyId),
		}
		
		for _, name := range possibleNames {
			keyPath := filepath.Join(basePath, name)
			if publicKey, err := loadPublicKeyFromFile(keyPath); err == nil {
				return publicKey, nil
			}
		}
	}
	
	return nil, fmt.Errorf("public key not found for key ID %s", keyId)
}

// loadPublicKeyFromFile loads a public key from a specific file path
func loadPublicKeyFromFile(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file %s: %w", path, err)
	}
	
	// Try to decode as base64 first
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		// If base64 decoding fails, treat as raw bytes
		decoded = data
	}
	
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size in file %s: expected %d bytes, got %d", path, ed25519.PublicKeySize, len(decoded))
	}
	
	return ed25519.PublicKey(decoded), nil
}

// SaveKeyPair saves a key pair to the specified directory
func SaveKeyPair(keyPair *SignatureKeyPair, outputDir string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}
	
	// Save private key
	privateKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s-private.key", keyPair.KeyId))
	privateKeyData := base64.StdEncoding.EncodeToString(keyPair.PrivateKey)
	if err := os.WriteFile(privateKeyPath, []byte(privateKeyData), 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}
	
	// Save public key
	publicKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s.pub", keyPair.KeyId))
	publicKeyData := base64.StdEncoding.EncodeToString(keyPair.PublicKey)
	if err := os.WriteFile(publicKeyPath, []byte(publicKeyData), 0644); err != nil {
		return fmt.Errorf("failed to save public key: %w", err)
	}
	
	return nil
}

// SaveSignatureToFile saves a signature to a file
func SaveSignatureToFile(signature []byte, filePath string) error {
	signatureData := base64.StdEncoding.EncodeToString(signature)
	if err := os.WriteFile(filePath, []byte(signatureData), 0644); err != nil {
		return fmt.Errorf("failed to save signature to %s: %w", filePath, err)
	}
	return nil
}

// LoadSignatureFromFile loads a signature from a file
func LoadSignatureFromFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature file %s: %w", filePath, err)
	}
	
	// Try to decode as base64 first
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		// If base64 decoding fails, treat as raw bytes
		decoded = data
	}
	
	if len(decoded) != ed25519.SignatureSize {
		return nil, fmt.Errorf("invalid signature size in file %s: expected %d bytes, got %d", filePath, ed25519.SignatureSize, len(decoded))
	}
	
	return decoded, nil
}

// getKeyDiscoveryPaths returns the list of directories to search for keys
func getKeyDiscoveryPaths() []string {
	var paths []string
	
	// Environment variable override
	if envPath := os.Getenv("DUCK_KEYS_PATH"); envPath != "" {
		paths = append(paths, envPath)
	}
	
	// System-wide keys
	paths = append(paths, "/etc/duckfile/keys")
	
	// User-specific keys
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(homeDir, ".duckfile", "keys"),
			filepath.Join(homeDir, ".config", "duckfile", "keys"),
		)
	}
	
	// Current directory keys (for development)
	paths = append(paths, "./keys", "./.duckfile/keys")
	
	return paths
}