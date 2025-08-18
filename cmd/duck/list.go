package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
	"github.com/spf13/cobra"
)

func init() {
	var (
		listShowRemote bool
		listShowVars   bool
		listShowExec   bool
		listLogLevel   string
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List targets defined in duck.yaml",
		Long: `List all targets from the duck.yaml configuration file.

Shows target names and descriptions by default. Use flags to include additional details.

FLAGS
  -r, --remote           Show remote template information (repo, ref, path)
  -v, --vars             Show template variables
  -e, --exec             Show execution info (binary, file flag)
  --log-level string     Log level: error, warn, info, debug

EXAMPLES
  duck list              List targets with names and descriptions
  duck list -r           Include remote template details
  duck list -v -e        Show variables and execution info`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Resolve and set log level
			logLevelStr := config.ResolveLogLevel(listLogLevel, cfg)
			effectiveLogLevel, err := log.ParseLevel(logLevelStr)
			if err != nil {
				return fmt.Errorf("invalid log level: %w", err)
			}
			log.SetLevel(effectiveLogLevel)

			fmt.Printf("%-12s %-12s %-s\n", "TARGET", "BINARY", "DESCRIPTION")
			printTarget := func(key string, t config.Target) {
				bin := t.Binary
				if bin == "" {
					bin = "-"
				}
				label := key
				if key == cfg.Default { // mark default
					label = key + "*"
				}
				fmt.Printf("%-12s %-12s %-s\n", label, bin, t.Description)
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
						fmt.Printf("      - %s (%s)\n", k, origin)
					}
				}
				if listShowExec && t.Binary != "" {
					fmt.Printf("    exec: %s %s <rendered> %s\n", t.Binary, t.FileFlag, strings.Join(t.Args, " "))
				}
			}
			keys := make([]string, 0, len(cfg.Targets))
			for k := range cfg.Targets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				printTarget(k, cfg.Targets[k])
			}
			return nil
		},
	}
	listCmd.Flags().BoolVarP(&listShowRemote, "remote", "r", false, "Show remote template configuration (repo/ref/path/delims)")
	listCmd.Flags().BoolVarP(&listShowVars, "vars", "v", false, "Show variable names and their kinds")
	listCmd.Flags().BoolVarP(&listShowExec, "exec", "e", false, "Show execution line (binary + file flag + args)")
	listCmd.Flags().StringVar(&listLogLevel, "log-level", "", "Set log level (error, warn, info, debug)")

	rootCmd.AddCommand(listCmd)
}
