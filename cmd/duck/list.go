package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/spf13/cobra"
)

func init() {
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
