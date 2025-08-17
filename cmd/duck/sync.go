package main

import (
	"fmt"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

func init() {
	var syncForce bool
	var syncLogLevel string
	var syncAllowedHosts []string
	var syncDeniedHosts []string
	var syncStrictMode bool
	var syncTrackCommitHash *bool
	var syncAutoUpdateOnChange *bool

	// Flag variables for mutual exclusion handling
	var trackCommitHashEnable bool
	var trackCommitHashDisable bool
	var autoUpdateEnable bool
	var autoUpdateDisable bool

	syncCmd := &cobra.Command{
		Use:   "sync [target]",
		Short: "Sync templates into cache without executing",
		Long: `Sync templates into the deterministic cache (.duck/objects) and update symlinks.

USAGE
  duck sync              Sync all targets
  duck sync [target]     Sync only the specified target

FLAGS
  -f, --force            Re-render templates ignoring existing cache
  --log-level string     Log level: error, warn, info, debug
  --track-commit-hash    Enable commit hash validation  
  --no-track-commit-hash Disable commit hash validation
  --auto-update-on-change     Auto-update when commit hash changes
  --no-auto-update-on-change  Disable auto-update on commit changes
  --allowed-hosts strings     Comma-separated allowed Git hosts
  --denied-hosts strings      Comma-separated denied Git hosts
  --strict-hosts              Fail if no host restrictions configured

Use environment variables DUCK_* to set defaults for configuration options.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			var target string
			if len(args) > 0 {
				target = args[0]
			}

			// Resolve and set log level
			logLevelStr := config.ResolveLogLevel(syncLogLevel, cfg)
			effectiveLogLevel, err := run.ParseLogLevel(logLevelStr)
			if err != nil {
				return fmt.Errorf("invalid log level: %w", err)
			}
			run.SetLogLevel(effectiveLogLevel)

			// Build security configuration from flags (CLI flags override environment)
			securityCfg := config.BuildSecurityConfig(syncAllowedHosts, syncDeniedHosts, syncStrictMode)

			// Resolve commit hash CLI flag values
			if trackCommitHashEnable {
				trueVal := true
				syncTrackCommitHash = &trueVal
			} else if trackCommitHashDisable {
				falseVal := false
				syncTrackCommitHash = &falseVal
			}

			if autoUpdateEnable {
				trueVal := true
				syncAutoUpdateOnChange = &trueVal
			} else if autoUpdateDisable {
				falseVal := false
				syncAutoUpdateOnChange = &falseVal
			}

			return run.Sync(cfg, target, syncForce, securityCfg, syncTrackCommitHash, syncAutoUpdateOnChange)
		},
	}

	syncCmd.Flags().BoolVarP(&syncForce, "force", "f", false, "Force re-render even if cache exists")
	syncCmd.Flags().StringVar(&syncLogLevel, "log-level", "", "Set log level (error, warn, info, debug)")

	// Commit hash tracking flags
	syncCmd.Flags().BoolVar(&trackCommitHashEnable, "track-commit-hash", false, "Enable commit hash tracking")
	syncCmd.Flags().BoolVar(&trackCommitHashDisable, "no-track-commit-hash", false, "Disable commit hash tracking")
	syncCmd.Flags().BoolVar(&autoUpdateEnable, "auto-update-on-change", false, "Enable auto-update on commit hash change")
	syncCmd.Flags().BoolVar(&autoUpdateDisable, "no-auto-update-on-change", false, "Disable auto-update on commit hash change")

	// Make flags mutually exclusive
	syncCmd.MarkFlagsMutuallyExclusive("track-commit-hash", "no-track-commit-hash")
	syncCmd.MarkFlagsMutuallyExclusive("auto-update-on-change", "no-auto-update-on-change")

	// Security configuration flags
	syncCmd.Flags().StringSliceVar(&syncAllowedHosts, "allowed-hosts", nil,
		"Comma-separated list of allowed Git hostnames (overrides DUCK_ALLOWED_HOSTS)")
	syncCmd.Flags().StringSliceVar(&syncDeniedHosts, "denied-hosts", nil,
		"Comma-separated list of denied Git hostnames (overrides DUCK_DENIED_HOSTS)")
	syncCmd.Flags().BoolVar(&syncStrictMode, "strict-hosts", false,
		"Fail if no host restrictions are configured (overrides DUCK_STRICT_MODE)")

	rootCmd.AddCommand(syncCmd)
}
