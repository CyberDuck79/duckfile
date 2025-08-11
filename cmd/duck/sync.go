package main

import (
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

func init() {
	var syncForce bool
	var syncVerbose bool
	var syncDebug bool
	syncCmd := &cobra.Command{
		Use:   "sync [target]",
		Short: "Sync templates into cache without executing",
		Long:  "Sync templates into the deterministic cache (.duck/objects) and update symlinks. Provide an optional target to sync only that target. Use -f/--force to re-render ignoring existing cache.",
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
			if syncDebug {
				run.SetLogLevel(run.LogDebug)
			} else if syncVerbose {
				run.SetLogLevel(run.LogVerbose)
			}
			return run.Sync(cfg, target, syncForce)
		},
	}
	syncCmd.Flags().BoolVarP(&syncForce, "force", "f", false, "Force re-render even if cache exists")
	syncCmd.Flags().BoolVarP(&syncVerbose, "verbose", "v", false, "Verbose output (steps)")
	syncCmd.Flags().BoolVarP(&syncDebug, "debug", "d", false, "Debug output (very detailed)")
	rootCmd.AddCommand(syncCmd)
}
