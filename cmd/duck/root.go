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
			case "-v", "--version":
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

func init() {
	// Set the version in the root command
	rootCmd.Version = Version

	// Subcommand: sync
	var syncForce bool
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
			return run.Sync(cfg, target, syncForce)
		},
	}
	syncCmd.Flags().BoolVarP(&syncForce, "force", "f", false, "Force re-render even if cache exists")

	// Subcommand: clean
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
			return run.Clean(cfg, target)
		},
	}

	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(cleanCmd)
}

// Execute is called by main.go
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func loadConfig() (*config.DuckConf, error) {
	// detect config file
	configFiles := []string{"duck.yaml", "duck.yml", ".duck.yaml", ".duck.yml"}
	var cfgFile string
	for _, f := range configFiles {
		if _, err := os.Stat(f); err == nil {
			cfgFile = f
			break
		}
	}
	if cfgFile == "" {
		return nil, fmt.Errorf("no config file found (tried: %v)", configFiles)
	}
	// load config
	return config.Load(cfgFile)
}
