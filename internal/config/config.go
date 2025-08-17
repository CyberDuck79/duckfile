package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/git"
	"gopkg.in/yaml.v3"
)

// Delims lets a template override the default Go template delimiters.
type Delims struct {
	Left  string `yaml:"left"`
	Right string `yaml:"right"`
}

type Template struct {
	Repo     string `yaml:"repo"`
	Ref      string `yaml:"ref"`
	Path     string `yaml:"path"`
	Checksum string `yaml:"checksum,omitempty"`

	// Optional delimiter override to avoid conflicts with downstream tools (e.g., Taskfile).
	Delims *Delims `yaml:"delims,omitempty"`
	// If true, missing keys render as empty strings (zero values). Default: strict error.
	AllowMissing bool `yaml:"allowMissing,omitempty"`
	// If true, enables commit hash storage and validation. Default: false.
	TrackCommitHash bool `yaml:"trackCommitHash,omitempty"`
	// If true, auto-update if commit hash changes; otherwise warn and stop. Default: false.
	AutoUpdateOnChange bool `yaml:"autoUpdateOnChange,omitempty"`
}

// VarKind represents the origin/behavior of a variable value.
type VarKind int

const (
	VarLiteral VarKind = iota // plain scalar (string/number/bool)
	VarEnv                    // !env NAME
	VarCmd                    // !cmd 'sh expression'
	VarFile                   // !file path
)

// VarValue supports tagged scalars like !env, !cmd, !file as well as plain scalars.
// It implements yaml.Unmarshaler to capture custom tags.
type VarValue struct {
	Kind  VarKind
	Arg   string // tag argument (env name, command, or file path)
	Value any    // for literal
}

// MarshalYAML enables preserving custom tags when writing config files.
func (v VarValue) MarshalYAML() (any, error) {
	switch v.Kind {
	case VarEnv:
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!env", Value: v.Arg}
		return n, nil
	case VarCmd:
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!cmd", Value: v.Arg}
		return n, nil
	case VarFile:
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!file", Value: v.Arg}
		return n, nil
	case VarLiteral:
		return v.Value, nil
	default:
		return v.Value, nil
	}
}

func (v *VarValue) UnmarshalYAML(node *yaml.Node) error {
	// Custom tags we accept: !env, !cmd, !file
	switch node.Tag {
	case "!env":
		v.Kind, v.Arg = VarEnv, node.Value
		return nil
	case "!cmd":
		v.Kind, v.Arg = VarCmd, node.Value
		return nil
	case "!file":
		v.Kind, v.Arg = VarFile, node.Value
		return nil
	}

	// Otherwise, treat as literal and parse basic YAML scalar types
	v.Kind = VarLiteral
	switch node.Tag {
	case "!!str", "":
		v.Value = node.Value
		return nil
	case "!!int":
		i, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return err
		}
		v.Value = i
		return nil
	case "!!float":
		f, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return err
		}
		v.Value = f
		return nil
	case "!!bool":
		switch node.Value {
		case "true", "True", "TRUE":
			v.Value = true
		case "false", "False", "FALSE":
			v.Value = false
		default:
			return fmt.Errorf("invalid boolean literal: %q", node.Value)
		}
		return nil
	default:
		// Fallback: store as string
		v.Value = node.Value
		return nil
	}
}

type Target struct {
	// Description is an optional human readable explanation printed by `duck list`.
	Description  string              `yaml:"description,omitempty"`
	Binary       string              `yaml:"binary,omitempty"`
	FileFlag     string              `yaml:"fileFlag,omitempty"`
	Template     Template            `yaml:"template"`
	Variables    map[string]VarValue `yaml:"variables,omitempty"`
	RenderedPath string              `yaml:"renderedPath,omitempty"`
	Args         ArgList             `yaml:"args,omitempty"`
}

// Settings represents the global configuration options
type Settings struct {
	CacheDir           string `yaml:"cacheDir,omitempty"`
	LogLevel           string `yaml:"logLevel,omitempty"`
	Locked             bool   `yaml:"locked,omitempty"`
	TrackCommitHash    bool   `yaml:"trackCommitHash,omitempty"`
	AutoUpdateOnChange bool   `yaml:"autoUpdateOnChange,omitempty"`
}

// GetLogLevel returns the configured log level or default "info"
func (s *Settings) GetLogLevel() string {
	if s == nil || s.LogLevel == "" {
		return "info"
	}
	return s.LogLevel
}

// GetCacheDir returns the configured cache dir or default ".duck/objects"
func (s *Settings) GetCacheDir() string {
	if s == nil || s.CacheDir == "" {
		return ".duck/objects"
	}
	return s.CacheDir
}

// IsLocked returns whether locked mode is enabled
func (s *Settings) IsLocked() bool {
	if s == nil {
		return false
	}
	return s.Locked
}

// GetTrackCommitHash returns whether commit hash tracking is enabled globally
func (s *Settings) GetTrackCommitHash() bool {
	if s == nil {
		return false
	}
	return s.TrackCommitHash
}

// GetAutoUpdateOnChange returns whether auto-update on commit hash change is enabled globally
func (s *Settings) GetAutoUpdateOnChange() bool {
	if s == nil {
		return false
	}
	return s.AutoUpdateOnChange
}

type DuckConf struct {
	Version int `yaml:"version"`
	// Default is the key of the target (in Targets) executed when the user omits a target.
	Default  string            `yaml:"default"`
	Targets  map[string]Target `yaml:"targets"`
	Settings *Settings         `yaml:"settings,omitempty"`
}

// Save writes the configuration to disk as YAML.
func (c *DuckConf) Save(path string) error {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Targets == nil {
		c.Targets = map[string]Target{}
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func Load(path string) (*DuckConf, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg DuckConf
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ArgList accepts either a single string or a list of strings in YAML.
// Examples:
//
//	args: "--silent"           => ["--silent"]
//	args: ["-v", "--color"]  => ["-v","--color"]
type ArgList []string

func (a *ArgList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		// Single string value
		if node.Value == "" {
			*a = []string{}
		} else {
			*a = []string{node.Value}
		}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(node.Content))
		for _, c := range node.Content {
			if c.Kind != yaml.ScalarNode {
				return fmt.Errorf("args array must contain strings")
			}
			out = append(out, c.Value)
		}
		*a = out
		return nil
	default:
		return fmt.Errorf("invalid YAML type for args: %v", node.Kind)
	}
}

// Validate enforces cross-field rules:
// - binary is optional
// - fileFlag and args are only allowed when binary is set
func (c *DuckConf) Validate() error {
	if strings.TrimSpace(c.Default) == "" {
		return fmt.Errorf("default target key must be set to one of the declared targets")
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("no targets declared; 'targets' mapping must contain at least the default target '%s'", c.Default)
	}

	// Validate settings
	if err := validateSettings(c.Settings); err != nil {
		return err
	}

	// Validate each target
	for name, t := range c.Targets {
		if err := validateTarget(t, name); err != nil {
			return err
		}
	}
	if _, ok := c.Targets[c.Default]; !ok {
		// Build list for diagnostics
		keys := make([]string, 0, len(c.Targets))
		for k := range c.Targets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Errorf("default target %q not found; available targets: %s", c.Default, strings.Join(keys, ", "))
	}
	return nil
}

func validateSettings(s *Settings) error {
	if s == nil {
		return nil
	}

	if s.LogLevel != "" {
		validLevels := []string{"error", "warn", "info", "debug"}
		valid := false
		for _, level := range validLevels {
			if s.LogLevel == level {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid logLevel %q, must be one of: %s", s.LogLevel, strings.Join(validLevels, ", "))
		}
	}

	return nil
}

func validateTarget(t Target, name string) error {
	hasBin := strings.TrimSpace(t.Binary) != ""
	if !hasBin {
		if strings.TrimSpace(t.FileFlag) != "" {
			return fmt.Errorf("target %q: fileFlag is not allowed without binary", name)
		}
		if len(t.Args) > 0 {
			return fmt.Errorf("target %q: args are not allowed without binary", name)
		}
	} else { // binary present
		if strings.TrimSpace(t.FileFlag) == "" {
			return fmt.Errorf("target %q: fileFlag is required when binary is set", name)
		}
	}

	// Validate commit hash tracking configuration
	return validateCommitHashTracking(t.Template, name)
}

// validateCommitHashTracking checks if commit hash tracking is properly configured.
// If ref is already a commit hash, commit hash tracking doesn't make sense since commit hashes don't change.
func validateCommitHashTracking(template Template, targetName string) error {
	// If commit hash tracking is enabled, validate that ref is not already a commit hash
	if template.TrackCommitHash {
		if template.Ref != "" && git.IsCommitHash(template.Ref) {
			return fmt.Errorf("target %q: commit hash tracking is invalid when ref is already a commit hash (%s).\n\n"+
				"Commit hashes are immutable and don't change, so tracking them is unnecessary.\n\n"+
				"To fix this issue, choose one of the following options:\n"+
				"  • Change 'ref' to a branch name (e.g., 'main', 'develop') or tag name (e.g., 'v1.0.0')\n"+
				"  • Set 'trackCommitHash: false' in your configuration\n"+
				"  • Remove the 'trackCommitHash' setting to use the default (false)\n\n"+
				"Example configurations:\n"+
				"  ref: main                    # Use branch name\n"+
				"  ref: v1.0.0                  # Use tag name\n"+
				"  trackCommitHash: false       # Disable tracking",
				targetName, template.Ref)
		}
	}

	// If auto-update is enabled, commit hash tracking must also be enabled
	if template.AutoUpdateOnChange && !template.TrackCommitHash {
		return fmt.Errorf("target %q: autoUpdateOnChange requires trackCommitHash to be enabled.\n\n"+
			"To fix this issue, add 'trackCommitHash: true' to your template configuration.\n\n"+
			"Example configuration:\n"+
			"  trackCommitHash: true\n"+
			"  autoUpdateOnChange: true",
			targetName)
	}

	return nil
}

// NewLiteralVar helper.
func NewLiteralVar(val any) VarValue  { return VarValue{Kind: VarLiteral, Value: val} }
func NewEnvVar(name string) VarValue  { return VarValue{Kind: VarEnv, Arg: name} }
func NewCmdVar(cmd string) VarValue   { return VarValue{Kind: VarCmd, Arg: cmd} }
func NewFileVar(path string) VarValue { return VarValue{Kind: VarFile, Arg: path} }

// ValidateTarget exposes target validation rules for external callers.
func ValidateTarget(t Target, name string) error { return validateTarget(t, name) }

// ResolveLogLevel determines the effective log level from CLI flag, environment, and config
// Precedence: CLI flag > Environment variable > Config file > Default ("info")
// Returns the log level string, caller should parse it using run.ParseLogLevel
func ResolveLogLevel(cliLogLevel string, cfg *DuckConf) string {
	// 1. CLI flag has highest precedence
	if cliLogLevel != "" {
		return cliLogLevel
	}

	// 2. Environment variable
	if envLevel := strings.TrimSpace(os.Getenv("DUCK_LOG_LEVEL")); envLevel != "" {
		return envLevel
	}

	// 3. Config file
	if cfg != nil && cfg.Settings != nil {
		return cfg.Settings.GetLogLevel()
	}

	// 4. Default
	return "info"
}

// ResolveTrackCommitHash determines the effective track commit hash setting from CLI flag, environment, and config
// Precedence: CLI flag > Environment variable > Template config > Global settings > Default (false)
func ResolveTrackCommitHash(cliFlag *bool, template *Template, cfg *DuckConf) bool {
	// 1. CLI flag has highest precedence
	if cliFlag != nil {
		return *cliFlag
	}

	// 2. Environment variable
	if envValue := strings.TrimSpace(os.Getenv("DUCK_TRACK_COMMIT_HASH")); envValue != "" {
		return strings.ToLower(envValue) == "true" || envValue == "1"
	}

	// 3. Template configuration
	if template != nil && template.TrackCommitHash {
		return true
	}

	// 4. Global settings
	if cfg != nil && cfg.Settings != nil {
		return cfg.Settings.GetTrackCommitHash()
	}

	// 5. Default
	return false
}

// ResolveAutoUpdateOnChange determines the effective auto-update setting from CLI flag, environment, and config
// Precedence: CLI flag > Environment variable > Template config > Global settings > Default (false)
func ResolveAutoUpdateOnChange(cliFlag *bool, template *Template, cfg *DuckConf) bool {
	// 1. CLI flag has highest precedence
	if cliFlag != nil {
		return *cliFlag
	}

	// 2. Environment variable
	if envValue := strings.TrimSpace(os.Getenv("DUCK_AUTO_UPDATE_ON_CHANGE")); envValue != "" {
		return strings.ToLower(envValue) == "true" || envValue == "1"
	}

	// 3. Template configuration
	if template != nil && template.AutoUpdateOnChange {
		return true
	}

	// 4. Global settings
	if cfg != nil && cfg.Settings != nil {
		return cfg.Settings.GetAutoUpdateOnChange()
	}

	// 5. Default
	return false
}
