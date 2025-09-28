//nolint:errcheck
package run

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestValidateAndCacheTemplateChecksum covers:
// 1. success path (valid checksum)
// 2. mismatch error path
// 3. warning/reuse path when checksum file already exists (no error expected)
func TestValidateAndCacheTemplateChecksum(t *testing.T) {
	withTempWD(t, func() {
		file := "tpl.tpl"
		content := []byte("hello")
		if err := os.WriteFile(file, content, 0o644); err != nil {
			t.Fatalf("write template: %v", err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(content))
		template := &config.Template{Checksum: checksum}
		target := config.Target{Template: template}
		// success
		if err := validateAndCacheTemplateChecksum(target, template, file, "."); err != nil {
			t.Fatalf("valid checksum: %v", err)
		}
		// mismatch
		badTemplate := &config.Template{Checksum: "deadbeef"}
		bad := config.Target{Template: badTemplate}
		if err := validateAndCacheTemplateChecksum(bad, badTemplate, file, "."); err == nil {
			t.Fatal("expected mismatch error")
		}
		// warning/reuse (invokes log warning internally if config changed but checksum same) - should not error
		if err := validateAndCacheTemplateChecksum(target, template, file, "."); err != nil {
			t.Fatalf("repeat valid checksum: %v", err)
		}
	})
}
