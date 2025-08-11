package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

// test seam
var runExec = run.Exec

// Version is injected at build time with: go build -ldflags "-X main.Version=<version>"
// Defaults to "dev" when not set.
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
			target  string
			binArgs []string
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

		var wantVerbose, wantDebug bool
		for i := 0; i < len(duckArgs); i++ {
			switch duckArgs[i] {
			case "-h", "--help":
				return cmd.Help()
			case "-v", "--verbose":
				wantVerbose = true
			case "-d", "--debug":
				wantDebug = true
			case "-vd", "-dv":
				wantVerbose = true
				wantDebug = true
			default:
				if target == "" && !strings.HasPrefix(duckArgs[i], "-") {
					target = duckArgs[i]
				}
			}
		}
		if wantDebug {
			run.SetLogLevel(run.LogDebug)
		} else if wantVerbose {
			run.SetLogLevel(run.LogVerbose)
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

		// Treat the human name of the default target as an alias unless it conflicts with an explicit named target.
		if target != "" && target != "default" {
			if target == cfg.Default.Name {
				target = "default"
			}
		}

		// 3. If no target, use default
		if target == "" {
			target = "default"
		}

		// 4. execute
		return runExec(cfg, target, binArgs)
	},
}

func init() { rootCmd.Version = Version }

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
