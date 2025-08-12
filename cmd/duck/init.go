package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive wizard to create a duck.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat("duck.yaml"); err == nil {
				return fmt.Errorf("duck.yaml already exists")
			}
			return runInitWizard()
		},
	}
	rootCmd.AddCommand(initCmd)
}

func runInitWizard() error {
	fmt.Println("Duckfile init wizard – press Enter to accept defaults or leave optional fields empty.")
	first, name, err := runTargetWizard(true)
	if err != nil {
		return err
	}
	// Build config with default key referencing first target
	cfg := &config.DuckConf{Version: 1, Default: name, Targets: map[string]config.Target{name: first}}
	if err := cfg.Save("duck.yaml"); err != nil {
		return err
	}
	fmt.Printf("Created duck.yaml with default target '%s'.\n", name)
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Add another target? (y/N): ")
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "y" && resp != "yes" {
			break
		}
		cfg2, err := config.Load("duck.yaml")
		if err != nil {
			return err
		}
		nt, tname, err := runTargetWizard(false)
		if err != nil {
			return err
		}
		if cfg2.Targets == nil {
			cfg2.Targets = map[string]config.Target{}
		}
		if _, exists := cfg2.Targets[tname]; exists {
			fmt.Println("Target already exists; skipping.")
			continue
		}
		cfg2.Targets[tname] = nt
		if err := cfg2.Save("duck.yaml"); err != nil {
			return err
		}
		fmt.Println("Added target", tname)
	}
	return nil
}
