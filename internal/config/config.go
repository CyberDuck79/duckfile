package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Delims lets a template override the default Go template delimiters.
type Delims struct {
	Left  string `yaml:"left"`
	Right string `yaml:"right"`
}

type Template struct {
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
	Path string `yaml:"path"`

	// Optional delimiter override to avoid conflicts with downstream tools (e.g., Taskfile).
	Delims *Delims `yaml:"delims,omitempty"`
	// If true, missing keys render as empty strings (zero values). Default: strict error.
	AllowMissing bool `yaml:"allowMissing,omitempty"`
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
	Name      string              `yaml:"name"`
	Binary    string              `yaml:"binary"`
	FileFlag  string              `yaml:"fileFlag"`
	Template  Template            `yaml:"template"`
	Variables map[string]VarValue `yaml:"variables,omitempty"`
	CacheFile string              `yaml:"cacheFile,omitempty"`
}

type DuckConf struct {
	Version int               `yaml:"version"`
	Default Target            `yaml:"default"`
	Targets map[string]Target `yaml:"targets"`
}

func Load(path string) (*DuckConf, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg DuckConf
	return &cfg, yaml.Unmarshal(raw, &cfg)
}
