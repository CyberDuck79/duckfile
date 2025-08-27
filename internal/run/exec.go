package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// executeTarget handles the binary execution portion of Exec.
// It takes the target configuration, rendered file path, and user arguments
// and executes the binary with proper argument ordering.
func executeTarget(target config.Target, linkPath string, passthrough []string) error {
	// Order: [fileFlag linkPath] + target default args + user passthrough args
	args := append([]string{target.FileFlag, linkPath}, []string(target.Args)...)
	args = append(args, passthrough...)
	cmd := execCommand(target.Binary, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	log.Infof("🚀 exec: %s %s", target.Binary, strings.Join(args, " "))
	return cmd.Run()
}

// Exec renders and executes one target.
//
// This function uses the shared prepareAndRenderTemplate function for template
// processing (variable resolution, caching, checksum validation, etc.) and then
// executes the target's binary with the rendered template.
func Exec(cfg *config.DuckConf, targetName string, passthrough []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
	// Determine the effective target key
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return err
	}

	log.Infof("▶️ exec target %q", key)

	// Ensure executable configuration is present
	if strings.TrimSpace(t.Binary) == "" {
		return fmt.Errorf("target %q has no binary configured; use 'duck sync %s' to render without executing", key, key)
	}

	// Prepare template (shared logic with Sync)
	result, err := prepareAndRenderTemplate(key, t, cfg, false, securityCfg, trackCommitHashFlag, autoUpdateOnChangeFlag)
	if err != nil {
		return err
	}

	// Execute underlying binary with the rendered template
	return executeTarget(t, result.LinkPath, passthrough)
}
