package duck

import (
	"fmt"
	"os"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:                "duck [target] [args...]",
	Short:              "Duckfiles – remote-templating wrapper",
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// manual help because cobra no longer sees -h/--help
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			return cmd.Help()
		}
		// same to add for --version or --debug if needed
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

		// 3. detect optional explicit target
		var target string
		if len(args) > 0 && cfg.Targets[args[0]].Binary != "" {
			target = args[0]
			args = args[1:]
		}

		// 4. execute
		return run.Exec(cfg, target, args)
	},
}

// Execute is called by main.go
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
