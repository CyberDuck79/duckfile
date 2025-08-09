package main

import (
	"fmt"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new target to existing duck.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			nt, name, err := runTargetWizard(false)
			if err != nil {
				return err
			}
			if name == "default" {
				return fmt.Errorf("cannot add target with reserved name 'default'")
			}
			if cfg.Targets == nil {
				cfg.Targets = map[string]config.Target{}
			}
			if _, exists := cfg.Targets[name]; exists {
				return fmt.Errorf("target %s already exists", name)
			}
			cfg.Targets[name] = nt
			if err := cfg.Save("duck.yaml"); err != nil {
				return err
			}
			fmt.Println("Added target", name)
			return nil
		},
	}
	rootCmd.AddCommand(addCmd)
}
