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

	syncCmd := &cobra.Command{
		Use:   "sync [target]",
		Short: "Sync templates into cache without executing",
		Long:  "Sync templates into the deterministic cache (.duck/objects) and update symlinks. Provide an optional target to sync only that target. Use -f/--force to re-render ignoring existing cache.\n\nLogging: Use --log-level to control verbosity (error, warn, info, debug).\n\nSecurity: Use --allowed-hosts, --denied-hosts, or --strict-hosts flags to restrict which Git hosts can be accessed, or set DUCK_ALLOWED_HOSTS, DUCK_DENIED_HOSTS, DUCK_STRICT_MODE environment variables.",
		Args:  cobra.MaximumNArgs(1),
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

			return run.Sync(cfg, target, syncForce, securityCfg)
		},
	}

	syncCmd.Flags().BoolVarP(&syncForce, "force", "f", false, "Force re-render even if cache exists")
	syncCmd.Flags().StringVar(&syncLogLevel, "log-level", "", "Set log level (error, warn, info, debug)")

	// Security configuration flags
	syncCmd.Flags().StringSliceVar(&syncAllowedHosts, "allowed-hosts", nil,
		"Comma-separated list of allowed Git hostnames (overrides DUCK_ALLOWED_HOSTS)")
	syncCmd.Flags().StringSliceVar(&syncDeniedHosts, "denied-hosts", nil,
		"Comma-separated list of denied Git hostnames (overrides DUCK_DENIED_HOSTS)")
	syncCmd.Flags().BoolVar(&syncStrictMode, "strict-hosts", false,
		"Fail if no host restrictions are configured (overrides DUCK_STRICT_MODE)")

	rootCmd.AddCommand(syncCmd)
}
