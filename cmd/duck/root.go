package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/run"
	"github.com/spf13/cobra"
)

// test seam - updated to include security config
var runExec = func(cfg *config.DuckConf, targetName string, passthrough []string, securityCfg *config.SecurityConfig, trackCommitHashFlag *bool, autoUpdateOnChangeFlag *bool) error {
	return run.Exec(cfg, targetName, passthrough, securityCfg, trackCommitHashFlag, autoUpdateOnChangeFlag)
}

// Version is injected at build time with: go build -ldflags "-X main.Version=<version>"
// Defaults to "dev" when not set.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "duck [target] -- [target_args...]",
	Short: "Universal remote templating for DevOps tools",
	Long: `Duck fetches, renders, and executes remote Git templates with deterministic caching.

USAGE
  duck [target] [flags]                 Execute target (or default if none specified)
  duck [target] -- [args...] [flags]   Execute target and pass args to underlying tool
  duck [command] [flags]                Run a specific command

COMMANDS
  add         Add a new target to existing duck.yaml
  clean       Purge cached objects and per-target directories  
  init        Interactive wizard to create a duck.yaml
  list        List targets defined in duck.yaml
  sync        Sync templates into cache without executing
  version     Print duck version
  help        Help about any command

FLAGS
  -h, --help                     Show help for duck
  --log-level string             Log level: error, warn, info, debug
  --track-commit-hash            Enable commit hash validation
  --no-track-commit-hash         Disable commit hash validation
  --auto-update-on-change        Auto-update when commit hash changes
  --no-auto-update-on-change     Disable auto-update on commit changes
  --allowed-hosts strings        Comma-separated allowed Git hosts
  --denied-hosts strings         Comma-separated denied Git hosts
  --strict-hosts                 Fail if no host restrictions configured

ENVIRONMENT VARIABLES
  DUCK_LOG_LEVEL                 Default log level
  DUCK_TRACK_COMMIT_HASH         Enable commit hash tracking (true/false)
  DUCK_AUTO_UPDATE_ON_CHANGE     Enable auto-update behavior (true/false)
  DUCK_ALLOWED_HOSTS             Comma-separated allowed hosts
  DUCK_DENIED_HOSTS              Comma-separated denied hosts
  DUCK_STRICT_MODE               Enable strict host validation (true/false)

EXAMPLES
  duck                           Execute default target
  duck build                     Execute 'build' target
  duck test -- --verbose         Execute 'test' with args
  duck sync                      Sync all templates
  duck --log-level=debug build   Execute with debug logging

Use "duck [command] --help" for more information about a command.`,
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true, // Disable to handle custom parsing with -- separator
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Manual parsing for target and passthrough args due to -- separator
		var (
			target             string
			binArgs            []string
			logLevel           string
			trackCommitHash    *bool
			autoUpdateOnChange *bool
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

		// Parse our custom flags from duckArgs
		allowedHosts := []string{}
		deniedHosts := []string{}
		strictMode := false

		for i := 0; i < len(duckArgs); i++ {
			arg := duckArgs[i]
			switch {
			case strings.HasPrefix(arg, "--log-level="):
				logLevel = arg[len("--log-level="):]
			case arg == "--log-level" && i+1 < len(duckArgs):
				logLevel = duckArgs[i+1]
				i++ // skip next arg
			case arg == "--track-commit-hash":
				trackTrue := true
				trackCommitHash = &trackTrue
			case arg == "--no-track-commit-hash":
				trackFalse := false
				trackCommitHash = &trackFalse
			case arg == "--auto-update-on-change":
				updateTrue := true
				autoUpdateOnChange = &updateTrue
			case arg == "--no-auto-update-on-change":
				updateFalse := false
				autoUpdateOnChange = &updateFalse
			case strings.HasPrefix(arg, "--allowed-hosts="):
				hostStr := arg[len("--allowed-hosts="):]
				allowedHosts = strings.Split(hostStr, ",")
			case arg == "--allowed-hosts" && i+1 < len(duckArgs):
				hostStr := duckArgs[i+1]
				allowedHosts = strings.Split(hostStr, ",")
				i++ // skip next arg
			case strings.HasPrefix(arg, "--denied-hosts="):
				hostStr := arg[len("--denied-hosts="):]
				deniedHosts = strings.Split(hostStr, ",")
			case arg == "--denied-hosts" && i+1 < len(duckArgs):
				hostStr := duckArgs[i+1]
				deniedHosts = strings.Split(hostStr, ",")
				i++ // skip next arg
			case arg == "--strict-hosts":
				strictMode = true
			case arg == "-h" || arg == "--help":
				return cmd.Help()
			case !strings.HasPrefix(arg, "-"):
				// This is the target name
				if target == "" {
					target = arg
				}
			}
		}

		// 1. load config
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		// 2. Resolve and set log level
		logLevelStr := config.ResolveLogLevel(logLevel, cfg)
		effectiveLogLevel, err := run.ParseLogLevel(logLevelStr)
		if err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}
		run.SetLogLevel(effectiveLogLevel)

		// 3. If no target or explicit legacy "default", translate to configured default key
		if target == "" || target == "default" {
			target = cfg.Default
		}

		// 4. Build security configuration (manual CLI parsing override or environment)
		securityCfg := config.BuildSecurityConfig(allowedHosts, deniedHosts, strictMode)

		// 5. execute with security validation
		return runExec(cfg, target, binArgs, securityCfg, trackCommitHash, autoUpdateOnChange)
	},
}

func init() {
	rootCmd.Version = Version
	// Set custom help template only for root command
	rootCmd.SetHelpTemplate(`{{.Long}}
`)
}

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
