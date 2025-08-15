package main

import (
	"fmt"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

func init() {
	var cleanLogLevel string

	cleanCmd := &cobra.Command{
		Use:   "clean [target]",
		Short: "Purge cached objects and per-target directories",
		Long:  "Purge cache by removing .duck/objects and per-target directories. Provide an optional target to clean only that target.\n\nLogging: Use --log-level to control verbosity (error, warn, info, debug).",
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
			logLevelStr := config.ResolveLogLevel(cleanLogLevel, cfg)
			effectiveLogLevel, err := run.ParseLogLevel(logLevelStr)
			if err != nil {
				return fmt.Errorf("invalid log level: %w", err)
			}
			run.SetLogLevel(effectiveLogLevel)

			return run.Clean(cfg, target)
		},
	}

	cleanCmd.Flags().StringVar(&cleanLogLevel, "log-level", "", "Set log level (error, warn, info, debug)")

	rootCmd.AddCommand(cleanCmd)
}
