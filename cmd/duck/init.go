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
	first, _, err := runTargetWizard(true)
	if err != nil {
		return err
	}
	cfg := &config.DuckConf{Version: 1, Default: first, Targets: map[string]config.Target{}}
	if err := cfg.Save("duck.yaml"); err != nil {
		return err
	}
	fmt.Println("Created duck.yaml with default target.")
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
		nt, name, err := runTargetWizard(false)
		if err != nil {
			return err
		}
		if name == "default" {
			fmt.Println("Skipping – name 'default' is reserved.")
			continue
		}
		if cfg2.Targets == nil {
			cfg2.Targets = map[string]config.Target{}
		}
		if _, exists := cfg2.Targets[name]; exists {
			fmt.Println("Target already exists; skipping.")
			continue
		}
		cfg2.Targets[name] = nt
		if err := cfg2.Save("duck.yaml"); err != nil {
			return err
		}
		fmt.Println("Added target", name)
	}
	return nil
}
