package run

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/CyberDuck79/duckfile/internal/config"
	"github.com/CyberDuck79/duckfile/internal/log"
)

// resolveAndLogVariables resolves target variables and emits debug logging.
func resolveAndLogVariables(target config.Target) (map[string]any, error) {
	vars, err := resolveVariables(target.Variables)
	if err != nil {
		return nil, err
	}
	log.Infof("🔧 resolved variables: %d", len(vars))
	if log.IsLevelEnabled(log.Debug) {
		for k, v := range vars {
			log.Debugf("var %s=%v", k, v)
		}
	}
	return vars, nil
}

func resolveVariables(in map[string]config.VarValue) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch v.Kind {
		case config.VarLiteral:
			out[k] = v.Value
		case config.VarEnv:
			out[k] = os.Getenv(v.Arg)
		case config.VarFile:
			b, err := os.ReadFile(v.Arg)
			if err != nil {
				return nil, fmt.Errorf("read file for var %s: %w", k, err)
			}
			out[k] = string(b)
		case config.VarCmd:
			// Execute with /bin/sh -c to match spec
			cmd := exec.Command("/bin/sh", "-c", v.Arg)
			cmd.Env = os.Environ()
			outb, err := cmd.Output()
			if err != nil {
				// bubble up stderr if possible
				if ee, ok := err.(*exec.ExitError); ok {
					return nil, fmt.Errorf("cmd var %s failed: %v: %s", k, err, string(ee.Stderr))
				}
				return nil, fmt.Errorf("cmd var %s failed: %w", k, err)
			}
			// Trim trailing newline for typical CLI output
			out[k] = strings.TrimRight(string(outb), "\r\n")
		default:
			out[k] = v.Value
		}
	}
	return out, nil
}
