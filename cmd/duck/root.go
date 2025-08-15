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
var runExec = func(cfg *config.DuckConf, targetName string, passthrough []string, securityCfg *config.SecurityConfig) error {
	return run.Exec(cfg, targetName, passthrough, securityCfg)
}

// Version is injected at build time with: go build -ldflags "-X main.Version=<version>"
// Defaults to "dev" when not set.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "duck [target] -- [target_args...]",
	Short: "Duckfiles – remote-templating wrapper",
	Long: `Duckfiles – remote-templating wrapper

Duck fetches, renders, and executes remote Git templates with deterministic caching.
Templates are rendered with Go's text/template engine and Sprig functions.

USAGE:
  duck [target]          Execute the specified target (or default if not provided)
  duck [target] -- args  Execute target and pass args to the underlying binary
  duck sync [target]     Sync templates to cache without executing
  duck list              List available targets with descriptions
  duck clean [target]    Clean cached objects and working directories
  duck init              Interactive setup wizard
  duck add               Interactive add target

LOGGING:
Log level can be configured via CLI flag, environment variable, or config file.
Precedence: CLI flag > DUCK_LOG_LEVEL env var > settings.logLevel > default (info)

CLI Flags:
  --log-level=debug       # Set log level (error, warn, info, debug)

Environment Variables:
  DUCK_LOG_LEVEL="info"   # Set default log level

SECURITY:
Host allow/deny lists can be configured via environment variables or CLI flags
to prevent supply-chain attacks. These restrictions are kept separate from
duck.yaml to prevent attackers from modifying both targets and security policies.

Environment Variables:
  DUCK_ALLOWED_HOSTS="github.com,gitlab.internal.com"  # Comma-separated allowed hosts
  DUCK_DENIED_HOSTS="malicious-host.com"               # Comma-separated denied hosts  
  DUCK_STRICT_MODE="true"                              # Fail if no restrictions configured

CLI Flags (override environment):
  --allowed-hosts=host1,host2  # Override allowed hosts
  --denied-hosts=host1,host2   # Override denied hosts
  --strict-hosts               # Enable strict mode

Examples:
  duck                                    # Execute default target
  duck build                              # Execute 'build' target
  duck test -- --verbose                 # Execute 'test' target with args
  duck sync                               # Sync all templates to cache
  duck --allowed-hosts=github.com build   # Only allow GitHub repositories
  duck --log-level=debug build            # Execute with debug logging`,
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true, // Disable to handle custom parsing with -- separator
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Manual parsing for target and passthrough args due to -- separator
		var (
			target   string
			binArgs  []string
			logLevel string
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
		return runExec(cfg, target, binArgs, securityCfg)
	},
}

func init() {
	rootCmd.Version = Version
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
