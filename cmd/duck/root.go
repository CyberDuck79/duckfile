package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
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

// Global variable to store config file path from --config flag
var configPath string

var rootCmd = &cobra.Command{
	Use:   "duck [-- target_args...]",
	Short: "Universal remote templating for DevOps tools",
	Long: `Duck fetches, renders, and executes remote Git templates with deterministic caching.

USAGE
  duck [flags]                      Execute default target  
  duck -- [args...] [flags]        Execute default target with args
  duck exec [target] [flags]        Execute specific target
  duck [command] [flags]            Run a specific command

EXAMPLES
  duck                              Execute default target
  duck -- --verbose                Execute default target with args
  duck exec build                   Execute 'build' target explicitly
  duck exec sync                    Execute target named 'sync' (not subcommand)
  duck sync                         Run sync subcommand (render templates)
  duck list                         List available targets

ENVIRONMENT VARIABLES
  DUCK_CONFIG                    Path to config file (overrides auto-discovery)
  DUCK_LOG_LEVEL                 Default log level
  DUCK_TRACK_COMMIT_HASH         Enable commit hash tracking (true/false)
  DUCK_AUTO_UPDATE_ON_CHANGE     Enable auto-update behavior (true/false)
  DUCK_ALLOWED_HOSTS             Comma-separated allowed hosts
  DUCK_DENIED_HOSTS              Comma-separated denied hosts
  DUCK_STRICT_MODE               Enable strict host validation (true/false)

FLAGS
  -c, --config string            Path to duck config file (default: auto-discover)
  --log-level string             Log level: error, warn, info, debug
  --track-commit-hash            Enable commit hash validation
  --no-track-commit-hash         Disable commit hash validation
  --auto-update-on-change        Auto-update when commit hash changes
  --no-auto-update-on-change     Disable auto-update on commit changes
  --allowed-hosts strings        Comma-separated allowed Git hosts
  --denied-hosts strings         Comma-separated denied Git hosts
  --strict-hosts                 Fail if no host restrictions configured`,
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true, // Disable to handle custom parsing with -- separator
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Execute targets (including default when no args provided)
		return executeTargetFromArgsWithCmd(cmd, args)
	},
}

func init() {
	rootCmd.Version = Version

	// Add persistent flag available to all subcommands
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "",
		"Path to duck config file (default: auto-discover duck.yaml, duck.yml, .duck.yaml, .duck.yml)")

	// Set custom help template only for root command
	rootCmd.SetHelpTemplate(`{{.Long}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`)
}

// executeTargetFromArgsWithCmd handles the common target execution logic with optional cmd for help
// parseConfigPath extracts config path from arguments, handling both --config and -c flags
func parseConfigPath(arg string, args []string, i int) (string, int, bool) {
	switch {
	case strings.HasPrefix(arg, "--config="):
		return arg[len("--config="):], i, true
	case (arg == "--config" || arg == "-c") && i+1 < len(args):
		return args[i+1], i + 1, true
	default:
		return "", i, false
	}
}

// parseLogLevel extracts log level from arguments
func parseLogLevel(arg string, args []string, i int) (string, int, bool) {
	switch {
	case strings.HasPrefix(arg, "--log-level="):
		return arg[len("--log-level="):], i, true
	case arg == "--log-level" && i+1 < len(args):
		return args[i+1], i + 1, true
	default:
		return "", i, false
	}
}

// parseHostList extracts host list from arguments for allowed/denied hosts
func parseHostList(arg string, args []string, i int, prefix string, flag string) ([]string, int, bool) {
	switch {
	case strings.HasPrefix(arg, prefix):
		hostStr := arg[len(prefix):]
		return strings.Split(hostStr, ","), i, true
	case arg == flag && i+1 < len(args):
		hostStr := args[i+1]
		return strings.Split(hostStr, ","), i + 1, true
	default:
		return nil, i, false
	}
}

// processSingleFlag handles parsing of a single command-line flag
func processSingleFlag(arg string, args []string, i int, result *argResults) (int, bool) {
	// Try parsing config path
	if configPathValue, newI, found := parseConfigPath(arg, args, i); found {
		configPath = configPathValue
		return newI, true
	}

	// Try parsing log level
	if logLevel, newI, found := parseLogLevel(arg, args, i); found {
		result.logLevel = logLevel
		return newI, true
	}

	// Try parsing allowed hosts
	if hosts, newI, found := parseHostList(arg, args, i, "--allowed-hosts=", "--allowed-hosts"); found {
		result.allowedHosts = hosts
		return newI, true
	}

	// Try parsing denied hosts
	if hosts, newI, found := parseHostList(arg, args, i, "--denied-hosts=", "--denied-hosts"); found {
		result.deniedHosts = hosts
		return newI, true
	}

	// Handle boolean and simple flags
	switch arg {
	case "--track-commit-hash":
		trackTrue := true
		result.trackCommitHash = &trackTrue
		return i, true
	case "--no-track-commit-hash":
		trackFalse := false
		result.trackCommitHash = &trackFalse
		return i, true
	case "--auto-update-on-change":
		updateTrue := true
		result.autoUpdateOnChange = &updateTrue
		return i, true
	case "--no-auto-update-on-change":
		updateFalse := false
		result.autoUpdateOnChange = &updateFalse
		return i, true
	case "--strict-hosts":
		result.strictMode = true
		return i, true
	case "-h", "--help":
		result.helpRequested = true
		return i, true
	default:
		if !strings.HasPrefix(arg, "-") && result.target == "" {
			// This is the target name
			result.target = arg
			return i, true
		}
	}

	return i, false
}

// parseArguments extracts all command-line arguments and returns structured data
func parseArguments(args []string) (argResults, error) {
	var result argResults

	// Find "--" separator and split args
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
		result.binArgs = args[sepIdx+1:]
	}

	// Parse flags from duckArgs
	for i := 0; i < len(duckArgs); i++ {
		newI, processed := processSingleFlag(duckArgs[i], duckArgs, i, &result)
		if processed {
			i = newI
		}
	}

	return result, nil
}

// argResults holds the parsed command-line arguments
type argResults struct {
	target             string
	binArgs            []string
	logLevel           string
	trackCommitHash    *bool
	autoUpdateOnChange *bool
	allowedHosts       []string
	deniedHosts        []string
	strictMode         bool
	helpRequested      bool
}

func executeTargetFromArgsWithCmd(cmd *cobra.Command, args []string) error {
	// Parse command-line arguments
	parsedArgs, err := parseArguments(args)
	if err != nil {
		return err
	}

	// Handle help request
	if parsedArgs.helpRequested {
		if cmd != nil {
			return cmd.Help()
		}
		// For exec command without cmd reference, just ignore help flag
		return nil
	}

	// 1. load config
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 2. Resolve and set log level
	logLevelStr := config.ResolveLogLevel(parsedArgs.logLevel, cfg)
	effectiveLogLevel, err := log.ParseLevel(logLevelStr)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	log.SetLevel(effectiveLogLevel)

	// 3. If no target or explicit legacy "default", translate to configured default key
	target := parsedArgs.target
	if target == "" || target == "default" {
		target = cfg.Default
	}

	// 4. Build security configuration with enhanced precedence system
	securityCfg, err := config.BuildSecurityConfigWithPrecedence(parsedArgs.allowedHosts, parsedArgs.deniedHosts, parsedArgs.strictMode)
	if err != nil {
		return fmt.Errorf("failed to build security configuration: %w", err)
	}

	// 5. execute with security validation
	return runExec(cfg, target, parsedArgs.binArgs, securityCfg, parsedArgs.trackCommitHash, parsedArgs.autoUpdateOnChange)
}

// checkForTargetConflictAndWarn checks if there's a target with the same name as the subcommand
// and warns the user about potential confusion
func checkForTargetConflictAndWarn(subcommandName string) {
	cfg, err := loadConfig()
	if err != nil {
		// If we can't load config, don't show warnings
		return
	}

	if _, exists := cfg.Targets[subcommandName]; exists {
		fmt.Fprintf(os.Stderr, "⚠️  Note: You have a target named '%s' but ran the '%s' subcommand.\n", subcommandName, subcommandName)
		fmt.Fprintf(os.Stderr, "   To execute the target instead, use: duck exec %s\n\n", subcommandName)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// resolveConfigFilePath determines the config file path using priority order:
// 1. --config flag (highest precedence)
// 2. DUCK_CONFIG environment variable
// 3. Auto-discovery (lowest precedence)
func resolveConfigFilePath() (string, error) {
	// Priority 1: --config flag (highest precedence)
	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			return "", fmt.Errorf("config file %q not found: %w", configPath, err)
		}
		return configPath, nil
	}

	// Priority 2: DUCK_CONFIG environment variable
	if envConfigPath := os.Getenv("DUCK_CONFIG"); envConfigPath != "" {
		if _, err := os.Stat(envConfigPath); err != nil {
			return "", fmt.Errorf("config file from DUCK_CONFIG %q not found: %w", envConfigPath, err)
		}
		return envConfigPath, nil
	}

	// Priority 3: Auto-discovery (lowest precedence)
	return discoverConfigFile()
}

// discoverConfigFile attempts to find a config file using standard names
func discoverConfigFile() (string, error) {
	configFiles := []string{"duck.yaml", "duck.yml", ".duck.yaml", ".duck.yml"}
	for _, f := range configFiles {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf("no config file found (tried: %v). Use --config to specify a custom path", configFiles)
}

func loadConfig() (*config.DuckConf, error) {
	// Load .env file first (before any config loading)
	// This allows .env variables to be used in config resolution
	if err := config.LoadEnvFileIfExists(log.Infof); err != nil {
		return nil, fmt.Errorf("environment setup failed: %w", err)
	}

	// Resolve config file path using priority order
	cfgFile, err := resolveConfigFilePath()
	if err != nil {
		return nil, err
	}

	log.Debugf("Loading configuration from %s", cfgFile)
	// Load config from the determined file path
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %q: %w", cfgFile, err)
	}
	log.Debugf("Successfully loaded configuration with %d targets", len(cfg.Targets))

	return cfg, nil
}
