package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/log"
)

// findEnvFile searches for .env files in the following priority order:
// 1. .env (current directory)
// 2. .duck/.env (duck cache directory)
// 3. .env.duck (duck-specific variant)
// Returns the path to the first file found, or empty string if none exist.
func findEnvFile() string {
	candidates := []string{
		".env",
		".duck/.env",
		".env.duck",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// LoadEnvFile loads environment variables from a .env file into the OS environment.
// It follows these rules:
// - Supports KEY=VALUE format
// - Skips empty lines and comments (lines starting with #)
// - Removes surrounding quotes (both single and double)
// - Existing environment variables take precedence (are not overwritten)
// - Returns the number of variables loaded and any error
func LoadEnvFile(path string) (int, error) {
	if path == "" {
		return 0, nil // No .env file found, not an error
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open .env file %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warnf("failed to close .env file %s: %v", path, closeErr)
		}
	}()

	count := 0
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return count, fmt.Errorf(".env file %s line %d: invalid format, expected KEY=VALUE", path, lineNum)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Validate key format (basic validation for environment variable names)
		if key == "" {
			return count, fmt.Errorf(".env file %s line %d: empty key", path, lineNum)
		}

		// Remove quotes if present
		value = removeQuotes(value)

		// Only set if not already set (existing env vars take precedence)
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return count, fmt.Errorf("failed to set environment variable %s: %w", key, err)
			}
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("error reading .env file %s: %w", path, err)
	}

	return count, nil
}

// removeQuotes removes surrounding quotes (both single and double) from a string.
// Only removes quotes if the string is fully quoted (starts and ends with the same quote type).
func removeQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// LoadEnvFileIfExists attempts to find and load a .env file and logs the result.
// This is the main entry point for automatic .env loading.
// It returns an error only if a .env file is found but cannot be loaded.
// Not finding a .env file is not considered an error.
// The logFunc parameter allows callers to provide their own logging function.
// If logFunc is nil, no logging is performed.
func LoadEnvFileIfExists(logFunc func(string, ...any)) error {
	envFile := findEnvFile()
	if envFile == "" {
		return nil // No .env file found, not an error
	}

	count, err := LoadEnvFile(envFile)
	if err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	if count > 0 && logFunc != nil {
		logFunc("📄 loaded %d environment variables from %s", count, envFile)
	}

	return nil
}

// LoadEnvFileIfExistsSilent loads .env file without any logging output.
// This is useful for early loading before logging is configured.
func LoadEnvFileIfExistsSilent() error {
	return LoadEnvFileIfExists(nil)
}
