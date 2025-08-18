package main

import (
	"fmt"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

func init() {
	var cleanLogLevel string

	cleanCmd := &cobra.Command{
		Use:   "clean [target]",
		Short: "Purge cached objects and per-target directories",
		Long: `Clean the duck cache by removing cached objects and working directories.

USAGE
  duck clean             Clean all cached objects and directories
  duck clean [target]    Clean only the specified target

FLAGS
  --log-level string     Log level: error, warn, info, debug

This removes .duck/objects and per-target directories to force fresh downloads
and rendering on the next sync or execution.`,
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
			logLevelStr := config.ResolveLogLevel(cleanLogLevel, cfg)
			effectiveLogLevel, err := log.ParseLevel(logLevelStr)
			if err != nil {
				return fmt.Errorf("invalid log level: %w", err)
			}
			log.SetLevel(effectiveLogLevel)

			return run.Clean(cfg, target)
		},
	}

	cleanCmd.Flags().StringVar(&cleanLogLevel, "log-level", "", "Set log level (error, warn, info, debug)")

	rootCmd.AddCommand(cleanCmd)
}
