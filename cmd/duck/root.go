package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:                "duck [target] -- [target_args...]",
	Short:              "Duckfiles – remote-templating wrapper",
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true, // manual parsing
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Manual flag parsing
		var (
			showVersion bool
			target      string
			binArgs     []string
		)

		// Find "--" separator
		sepIdx := -1
		for i, a := range args {
			if a == "--" {
				sepIdx = i
				break
			}
		}

		duckArgs := args
		if sepIdx != -1 {
			duckArgs = args[:sepIdx]
			binArgs = args[sepIdx+1:]
		}

		// Parse duckflags
		for i := 0; i < len(duckArgs); i++ {
			switch duckArgs[i] {
			case "--version":
				showVersion = true
			case "-h", "--help":
				return cmd.Help()
			default:
				// First non-flag is target
				if target == "" && !strings.HasPrefix(duckArgs[i], "-") {
					target = duckArgs[i]
				}
			}
		}

		if showVersion {
			fmt.Println("duck version", Version)
			return nil
		}

		// 1. detect config file
		configFiles := []string{"duck.yaml", "duck.yml", ".duck.yaml", ".duck.yml"}
		var cfgFile string
		for _, f := range configFiles {
			if _, err := os.Stat(f); err == nil {
				cfgFile = f
				break
			}
		}
		if cfgFile == "" {
			return fmt.Errorf("no config file found (tried: %v)", configFiles)
		}

		// 2. load config
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		// 3. If no target, use default
		if target == "" {
			target = "default"
		}

		// 4. execute
		return run.Exec(cfg, target, binArgs)
	},
}

// Execute is called by main.go
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
