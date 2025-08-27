package run

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// validateAndCacheTemplateChecksum validates the template checksum if
// configured and stores it for change detection warnings.
func validateAndCacheTemplateChecksum(target config.Target, src, remoteDir string) error {
	if target.Template.Checksum == "" { // nothing to validate
		return nil
	}
	sumFile := filepath.Join(remoteDir, "checksum.sha256")
	if _, err := os.Stat(sumFile); err == nil { // existing checksum
		if oldChecksum, err := os.ReadFile(sumFile); err == nil && string(oldChecksum) == target.Template.Checksum {
			log.Warnf("template config changed but checksum unchanged (repo/ref/path/vars)")
		}
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read template for checksum validation: %w", err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(b))
	if sum != target.Template.Checksum {
		return fmt.Errorf("template checksum mismatch: expected %s, got %s", target.Template.Checksum, sum)
	}
	if err := os.WriteFile(sumFile, []byte(target.Template.Checksum), 0o644); err != nil {
		return fmt.Errorf("failed to write checksum file: %w", err)
	}
	return nil
}
