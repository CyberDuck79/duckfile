package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// runTargetWizard collects target info interactively.
func runTargetWizard(isDefault bool) (config.Target, string, error) {
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
	// Helper function to ask for required input with retry loop
	askRequired := func(prompt, fieldName string) (string, error) {
		for {
			value, err := ask(prompt)
			if err != nil {
				return "", err
			}
			if value != "" {
				return value, nil
			}
			fmt.Printf("Error: %s cannot be empty. Please try again.\n", fieldName)
		}
	}
	var name string
	var err error
	if isDefault {
		name, err = askRequired("Default target key (called when <target> is not specified): ", "target key")
	} else {
		name, err = askRequired("Target key (CLI name): ", "target key")
	}
	if err != nil {
		return config.Target{}, "", err
	}

	// Check for potential conflicts with subcommand names and warn
	if config.IsReservedTargetName(name) {
		fmt.Printf("⚠️  Warning: Target name '%s' conflicts with the '%s' subcommand.\n", name, name)
		fmt.Printf("   Use 'duck exec %s' to execute this target instead of 'duck %s'.\n", name, name)
		cont, err := ask("Continue anyway? (y/N): ")
		if err != nil {
			return config.Target{}, "", err
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(cont)), "y") {
			return config.Target{}, "", fmt.Errorf("target creation cancelled")
		}
	}

	// Target description
	description, err := ask("Target description (shown in 'duck list') [optional]: ")
	if err != nil {
		return config.Target{}, "", err
	}
	binary, err := ask("Binary (leave empty for sync-only): ")
	if err != nil {
		return config.Target{}, "", err
	}
	var fileFlag string
	var args []string
	if strings.TrimSpace(binary) != "" {
		// FileFlag is required when binary is set - validate immediately
		for {
			fileFlag, err = ask("fileFlag (e.g. -f, --taskfile) [required when binary is set]: ")
			if err != nil {
				return config.Target{}, "", err
			}
			if strings.TrimSpace(fileFlag) != "" {
				break
			}
			fmt.Println("Error: fileFlag is required when binary is set. Please try again.")
		}

		// Default arguments
		argsInput, err := ask("Default args (passed to binary before user args) [optional]: ")
		if err != nil {
			return config.Target{}, "", err
		}
		if strings.TrimSpace(argsInput) != "" {
			// Simple space-split for now - could be enhanced for quoted args
			args = strings.Fields(argsInput)
		}
	}
	renderedPath, err := ask("Rendered path (where symlink/file should appear) [auto .duck/<target>/<base>]: ")
	if err != nil {
		return config.Target{}, "", err
	}

	// Copy vs symlink
	copyRendered := false
	if strings.TrimSpace(renderedPath) != "" {
		copyRendered, err = askBool("Copy rendered file instead of symlink? (y/N) [useful for pushable configs]: ")
		if err != nil {
			return config.Target{}, "", err
		}
	} else {
		// If renderedPath is empty, ask if they want to copy rendered
		copyRendered, err = askBool("Copy rendered file instead of symlink? (y/N) [useful for pushable configs]: ")
		if err != nil {
			return config.Target{}, "", err
		}
		// If they want to copy but didn't provide a path, require it
		if copyRendered {
			for {
				renderedPath, err = ask("Rendered path is required when copyRendered is true: ")
				if err != nil {
					return config.Target{}, "", err
				}
				if strings.TrimSpace(renderedPath) != "" {
					break
				}
				fmt.Println("Error: renderedPath cannot be empty when copyRendered is true. Please try again.")
			}
		}
	}
	repo, err := askRequired("Template repo (git URL): ", "repo")
	if err != nil {
		return config.Target{}, "", err
	}
	ref, err := ask("Template ref (branch/tag/commit) [HEAD]: ")
	if err != nil {
		return config.Target{}, "", err
	}
	path, err := askRequired("Template path inside repo (e.g. Makefile.tpl): ", "template path")
	if err != nil {
		return config.Target{}, "", err
	}
	if !strings.HasSuffix(path, ".tpl") {
		fmt.Println("(note) It's common to suffix template files with .tpl for clarity.")
	}

	// Template security and advanced features
	fmt.Println("\n--- Template Security & Advanced Options ---")

	checksum, err := ask("Template checksum (SHA-256) [optional, for supply-chain security]: ")
	if err != nil {
		return config.Target{}, "", err
	}
	if strings.TrimSpace(checksum) != "" && len(strings.TrimSpace(checksum)) != 64 {
		fmt.Println("(warning) Checksum should be 64 characters (SHA-256 hex). Continuing anyway.")
	}

	submodules, err := askBool("Fetch Git submodules? (y/N) [needed if template repo uses submodules]: ")
	if err != nil {
		return config.Target{}, "", err
	}

	trackCommitHash, err := askBool("Track commit hash? (y/N) [validates template hasn't changed]: ")
	if err != nil {
		return config.Target{}, "", err
	}

	var autoUpdateOnChange bool
	if trackCommitHash {
		autoUpdateOnChange, err = askBool("Auto-update when commit changes? (y/N) [requires trackCommitHash]: ")
		if err != nil {
			return config.Target{}, "", err
		}
	}
	allowMissingAns, err := ask("Allow missing variables? (y/N): ")
	if err != nil {
		return config.Target{}, "", err
	}
	allowMissing := strings.HasPrefix(strings.ToLower(allowMissingAns), "y")

	fmt.Println("\n--- Template Variables ---")
	vars := map[string]config.VarValue{}
	for {
		more, err := ask("Add variable? (y/N): ")
		if err != nil {
			return config.Target{}, "", err
		}
		if strings.ToLower(strings.TrimSpace(more)) != "y" {
			break
		}
		k, err := ask("  Variable name: ")
		if err != nil {
			return config.Target{}, "", err
		}
		if k == "" {
			fmt.Println("  Skipping empty variable name")
			continue
		}
		fmt.Println("  Variable types:")
		fmt.Println("    literal - Static value (e.g., 'production', '8080')")
		fmt.Println("    env     - Environment variable (e.g., from $HOME, $USER)")
		fmt.Println("    cmd     - Shell command output (e.g., 'git rev-parse HEAD')")
		fmt.Println("    file    - File contents (e.g., './VERSION', '~/.ssh/id_rsa.pub')")
		kind, err := ask("  Type (literal/env/cmd/file) [literal]: ")
		if err != nil {
			return config.Target{}, "", err
		}
		kind = strings.ToLower(strings.TrimSpace(kind))
		switch kind {
		case "", "literal":
			v, _ := ask("  Value: ")
			vars[k] = config.NewLiteralVar(v)
		case "env":
			v, _ := ask("  Environment variable name (e.g., HOME, USER, GO_VERSION): ")
			if v == "" {
				fmt.Println("  Warning: empty environment variable name")
			}
			vars[k] = config.NewEnvVar(v)
		case "cmd":
			v, _ := ask("  Shell command (e.g., 'git rev-parse HEAD', 'echo $(date)'): ")
			if v == "" {
				fmt.Println("  Warning: empty command")
			}
			vars[k] = config.NewCmdVar(v)
		case "file":
			v, _ := ask("  File path (e.g., './VERSION', '~/.ssh/id_rsa.pub'): ")
			if v == "" {
				fmt.Println("  Warning: empty file path")
			}
			vars[k] = config.NewFileVar(v)
		default:
			fmt.Println("  Unknown type; storing as literal string")
			v, _ := ask("  Value: ")
			vars[k] = config.NewLiteralVar(v)
		}
		fmt.Printf("  Added variable '%s'\n", k)
	}
	targ := config.Target{
		Description: description,
		Binary:      binary,
		FileFlag:    fileFlag,
		Template: config.Template{
			Repo:               repo,
			Ref:                ref,
			Path:               path,
			Checksum:           strings.TrimSpace(checksum),
			AllowMissing:       allowMissing,
			Submodules:         submodules,
			TrackCommitHash:    trackCommitHash,
			AutoUpdateOnChange: autoUpdateOnChange,
		},
		Variables:    vars,
		RenderedPath: renderedPath,
		CopyRendered: copyRendered,
		Args:         config.ArgList(args),
	}
	if err := config.ValidateTarget(targ, name); err != nil {
		return config.Target{}, "", err
	}
	return targ, name, nil
}
