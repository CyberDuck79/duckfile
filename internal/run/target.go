package run

import (
	"fmt"
	"sort"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// Search target from configuration, return effective target name and configuration or error if unknown target.
func searchTarget(cfg *config.DuckConf, targetName string) (string, config.Target, error) {
	// Determine the effective target key
	key := targetName
	if strings.TrimSpace(key) == "" || key == "default" { // "default" still accepted for backwards CLI invocation
		key = cfg.Default
	}
	t, ok := cfg.Targets[key]
	if !ok {
		// Provide helpful list
		keys := make([]string, 0, len(cfg.Targets))
		for k := range cfg.Targets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", config.Target{}, fmt.Errorf("unknown target %q; available: %s", key, strings.Join(keys, ", "))
	}
	return key, t, nil
}

func collectTargets(cfg *config.DuckConf, targetName string) (map[string]config.Target, error) {
	res := map[string]config.Target{}
	if strings.TrimSpace(targetName) == "" { // all
		for k, v := range cfg.Targets {
			res[k] = v
		}
		return res, nil
	}
	// Determine the effective target key
	key, t, err := searchTarget(cfg, targetName)
	if err != nil {
		return nil, err
	}
	res[key] = t
	return res, nil
}
