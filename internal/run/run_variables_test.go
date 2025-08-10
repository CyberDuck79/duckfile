package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

func TestResolveVariables(t *testing.T) {
	tmp := t.TempDir()
	fp := filepath.Join(tmp, "val.txt")
	os.WriteFile(fp, []byte("FILEVAL\n"), 0o644)
	os.Setenv("MYVAR", "ENVVAL")
	vars := map[string]config.VarValue{
		"L": config.NewLiteralVar("literal"),
		"E": config.NewEnvVar("MYVAR"),
		"F": config.NewFileVar(fp),
		"C": config.NewCmdVar("printf CMDOUT"),
	}
	out, err := resolveVariables(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out["C"] != "CMDOUT" {
		t.Fatalf("cmd var mismatch: %v", out["C"])
	}
	if out["F"].(string) != "FILEVAL\n" {
		t.Fatalf("file var mismatch: %q", out["F"])
	}
}
