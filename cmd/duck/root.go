package main

import (
	"fmt"
	"os"
	"sort"
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
			// Map default name alias
			if target != "" && target != "default" {
				if target == cfg.Default.Name {
					if _, conflict := cfg.Targets[target]; !conflict {
						target = "default"
					}
				}
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
			// Map default name alias
			if target != "" && target != "default" {
				if target == cfg.Default.Name {
					if _, conflict := cfg.Targets[target]; !conflict {
						target = "default"
					}
				}
			}
			return run.Clean(cfg, target)
		},
	}

	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(cleanCmd)

	// Subcommand: list
	var (
		listShowRemote bool
		listShowVars   bool
		listShowExec   bool
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List targets defined in duck.yaml",
		Long:  "List targets (default + named) from the configuration. Shows name and description by default. Use flags to include remote template, variables, and execution info.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			// Gather in stable order: default first then sorted names
			fmt.Printf("%-12s %-12s %-s\n", "TARGET", "BINARY", "DESCRIPTION")
			printTarget := func(key string, t config.Target) {
				bin := t.Binary
				if bin == "" {
					bin = "-"
				}
				fmt.Printf("%-12s %-12s %-s\n", key, bin, t.Description)
				if listShowRemote {
					fmt.Printf("    repo: %s\n", t.Template.Repo)
					ref := t.Template.Ref
					if ref == "" {
						ref = "HEAD"
					}
					fmt.Printf("    ref: %s\n", ref)
					fmt.Printf("    path: %s\n", t.Template.Path)
				}
				if listShowVars && len(t.Variables) > 0 {
					keys := make([]string, 0, len(t.Variables))
					for k := range t.Variables {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					fmt.Printf("    variables (%d):\n", len(keys))
					for _, k := range keys {
						vv := t.Variables[k]
						var origin string
						switch vv.Kind {
						case config.VarLiteral:
							origin = "literal"
						case config.VarEnv:
							origin = "env"
						case config.VarCmd:
							origin = "cmd"
						case config.VarFile:
							origin = "file"
						default:
							origin = "literal"
						}
						// We do not resolve values here to avoid side effects (commands) and performance hits
						fmt.Printf("      - %s (%s)\n", k, origin)
					}
				}
				if listShowExec && t.Binary != "" {
					fmt.Printf("    exec: %s %s <rendered> %s\n", t.Binary, t.FileFlag, strings.Join(t.Args, " "))
				}
			}
			printTarget("default", cfg.Default)
			if len(cfg.Targets) > 0 {
				keys := make([]string, 0, len(cfg.Targets))
				for k := range cfg.Targets {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					printTarget(k, cfg.Targets[k])
				}
			}
			return nil
		},
	}
	listCmd.Flags().BoolVarP(&listShowRemote, "remote", "r", false, "Show remote template configuration (repo/ref/path/delims)")
	listCmd.Flags().BoolVarP(&listShowVars, "vars", "v", false, "Show variable names and their kinds")
	listCmd.Flags().BoolVarP(&listShowExec, "exec", "e", false, "Show execution line (binary + file flag + args)")
	rootCmd.AddCommand(listCmd)
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
