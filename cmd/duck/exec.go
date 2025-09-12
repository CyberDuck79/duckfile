package main

import (
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [target] [-- target_args...]",
	Short: "Execute a target explicitly",
	Long: `Execute a specific target with optional arguments.

This command explicitly executes a target, useful when target names 
conflict with subcommand names.

EXAMPLES
  duck exec build                    Execute 'build' target
  duck exec test -- --verbose       Execute 'test' target with args
  duck exec sync                     Execute target named 'sync' (not subcommand)
  duck exec --config=custom.yaml build  Use custom config`,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true, // Handle custom parsing like root command
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTargetFromArgsWithCmd(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
}
