package main

import (
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

func init() {
	var cleanVerbose bool
	var cleanDebug bool
	cleanCmd := &cobra.Command{
		Use:   "clean [target]",
		Short: "Purge cached objects and per-target directories",
		Long:  "Purge cache by removing .duck/objects and per-target directories. Provide an optional target to clean only that target.",
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
			if cleanDebug {
				run.SetLogLevel(run.LogDebug)
			} else if cleanVerbose {
				run.SetLogLevel(run.LogVerbose)
			}
			return run.Clean(cfg, target)
		},
	}
	cleanCmd.Flags().BoolVarP(&cleanVerbose, "verbose", "v", false, "Verbose output (steps)")
	cleanCmd.Flags().BoolVarP(&cleanDebug, "debug", "d", false, "Debug output (very detailed)")
	rootCmd.AddCommand(cleanCmd)
}
