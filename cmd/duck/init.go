package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/spf13/cobra"
)

// Test seams for mocking in tests
var runInitWizardFunc = runInitWizardImpl
var runTargetWizardFunc = runTargetWizard

func init() {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive wizard to create a duck.yaml",
		Long: `Interactive wizard to create a new duck.yaml configuration file.

This command will guide you through setting up your first duck.yaml with:
- Project name and description
- Default target configuration
- Remote template repository
- Git reference (branch, tag, or commit) 
- Template file path
- Binary to execute (make, task, helm, etc.)
- File flag for the binary
- Template variables

The wizard creates a complete duck.yaml file ready for use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat("duck.yaml"); err == nil {
				return fmt.Errorf("duck.yaml already exists")
			}
			return runInitWizardFunc()
		},
	}
	rootCmd.AddCommand(initCmd)
}

func runInitWizardImpl() error {
	fmt.Println("Duckfile init wizard – press Enter to accept defaults or leave optional fields empty.")
	first, name, err := runTargetWizardFunc(true)
	if err != nil {
		return err
	}
	// Build config with default key referencing first target
	cfg := &config.DuckConf{Version: 1, Default: name, Targets: map[string]config.Target{name: first}}

	// Ask about global settings
	fmt.Println("\n--- Global Settings (optional) ---")
	reader := bufio.NewReader(os.Stdin)
	ask := func(prompt string) (string, error) {
		fmt.Print(prompt)
		txt, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(txt), nil
	}
	askBool := func(prompt string) (bool, error) {
		resp, err := ask(prompt)
		if err != nil {
			return false, err
		}
		return strings.HasPrefix(strings.ToLower(resp), "y"), nil
	}

	configureSettings, err := askBool("Configure global settings? (y/N) [cache, logging, security defaults]: ")
	if err != nil {
		return err
	}

	if configureSettings {
		settings := &config.Settings{}

		cacheDir, err := ask("Cache directory [.duck/objects]: ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(cacheDir) != "" {
			settings.CacheDir = cacheDir
		}

		logLevel, err := ask("Log level (error/warn/info/debug) [info]: ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(logLevel) != "" {
			settings.LogLevel = logLevel
		}

		locked, err := askBool("Locked mode? (y/N) [prevents updates when templates change]: ")
		if err != nil {
			return err
		}
		settings.Locked = locked

		globalTrack, err := askBool("Global commit tracking default? (y/N) [applies to all targets without explicit setting]: ")
		if err != nil {
			return err
		}
		settings.TrackCommitHash = globalTrack

		var globalAutoUpdate bool
		if globalTrack {
			globalAutoUpdate, err = askBool("Global auto-update default? (y/N) [applies to all targets without explicit setting]: ")
			if err != nil {
				return err
			}
			settings.AutoUpdateOnChange = globalAutoUpdate
		}

		cfg.Settings = settings
	}

	if err := cfg.Save("duck.yaml"); err != nil {
		return err
	}
	fmt.Printf("Created duck.yaml with default target '%s'.\n", name)
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
		nt, tname, err := runTargetWizardFunc(false)
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
