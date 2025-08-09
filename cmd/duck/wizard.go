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
	var name string
	var err error
	if isDefault {
		name, err = ask("Name (human readable) [build]: ")
		if err != nil {
			return config.Target{}, "", err
		}
		if name == "" {
			name = "build"
		}
	} else {
		name, err = ask("Target key (CLI name): ")
		if err != nil {
			return config.Target{}, "", err
		}
		if name == "" {
			return config.Target{}, "", fmt.Errorf("target key cannot be empty")
		}
	}
	binary, err := ask("Binary (leave empty for sync-only): ")
	if err != nil {
		return config.Target{}, "", err
	}
	var fileFlag string
	if strings.TrimSpace(binary) != "" {
		fileFlag, err = ask("fileFlag (e.g. -f, --taskfile) [optional if binary expects path implicitly]: ")
		if err != nil {
			return config.Target{}, "", err
		}
	}
	renderedPath, err := ask("Rendered path (where symlink/file should appear) [auto .duck/<target>/<base>]: ")
	if err != nil {
		return config.Target{}, "", err
	}
	repo, err := ask("Template repo (git URL): ")
	if err != nil {
		return config.Target{}, "", err
	}
	if repo == "" {
		return config.Target{}, "", fmt.Errorf("repo is required")
	}
	ref, err := ask("Template ref (branch/tag/commit) [HEAD]: ")
	if err != nil {
		return config.Target{}, "", err
	}
	path, err := ask("Template path inside repo (e.g. Makefile.tpl): ")
	if err != nil {
		return config.Target{}, "", err
	}
	if path == "" {
		return config.Target{}, "", fmt.Errorf("template path is required")
	}
	if !strings.HasSuffix(path, ".tpl") {
		fmt.Println("(note) It's common to suffix template files with .tpl for clarity.")
	}
	allowMissingAns, err := ask("Allow missing variables? (y/N): ")
	if err != nil {
		return config.Target{}, "", err
	}
	allowMissing := strings.HasPrefix(strings.ToLower(allowMissingAns), "y")
	vars := map[string]config.VarValue{}
	for {
		more, err := ask("Add variable? (y/N): ")
		if err != nil {
			return config.Target{}, "", err
		}
		if strings.ToLower(strings.TrimSpace(more)) != "y" {
			break
		}
		k, err := ask("  Key: ")
		if err != nil {
			return config.Target{}, "", err
		}
		if k == "" {
			fmt.Println("  Skipping empty key")
			continue
		}
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
			v, _ := ask("  Env var name: ")
			vars[k] = config.NewEnvVar(v)
		case "cmd":
			v, _ := ask("  Shell command: ")
			vars[k] = config.NewCmdVar(v)
		case "file":
			v, _ := ask("  File path: ")
			vars[k] = config.NewFileVar(v)
		default:
			fmt.Println("  Unknown type; storing as literal string")
			v, _ := ask("  Value: ")
			vars[k] = config.NewLiteralVar(v)
		}
	}
	targ := config.Target{
		Name:         name,
		Binary:       binary,
		FileFlag:     fileFlag,
		Template:     config.Template{Repo: repo, Ref: ref, Path: path, AllowMissing: allowMissing},
		Variables:    vars,
		RenderedPath: renderedPath,
	}
	if err := config.ValidateTarget(targ, name); err != nil {
		return config.Target{}, "", err
	}
	return targ, name, nil
}
