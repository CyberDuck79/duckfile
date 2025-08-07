package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Template struct {
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
	Path string `yaml:"path"`
}

type Target struct {
	Name      string   `yaml:"name"`
	Binary    string   `yaml:"binary"`
	FileFlag  string   `yaml:"fileFlag"`
	Template  Template `yaml:"template"`
	CacheFile string   `yaml:"cacheFile,omitempty"`
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
