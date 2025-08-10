package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// TestResolveVariables exercises literal, env, file, and command variables to
// confirm each VarKind resolves to the expected runtime value.
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

// TestResolveVariablesCmdFailure ensures a failing shell command used in a !cmd
// variable returns an error that includes the variable key.
func TestResolveVariablesCmdFailure(t *testing.T) {
	vars := map[string]config.VarValue{
		"BAD": config.NewCmdVar("sh -c 'echo err 1>&2; exit 7'"),
	}
	_, err := resolveVariables(vars)
	if err == nil || !strings.Contains(err.Error(), "cmd var BAD failed") {
		t.Fatalf("expected failing cmd var error, got %v", err)
	}
}

// TestResolveVariablesMissingFile validates that a !file variable referencing a
// non-existent path returns a wrapped error identifying the variable key.
func TestResolveVariablesMissingFile(t *testing.T) {
	vars := map[string]config.VarValue{"F": config.NewFileVar("./does-not-exist.txt")}
	_, err := resolveVariables(vars)
	if err == nil || !strings.Contains(err.Error(), "read file for var F") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

// TestAllowMissingVsStrict contrasts strict (missing variable => error) vs
// allowMissing (missing variable => empty string) template rendering modes.
func TestAllowMissingVsStrict(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)
	// template with missing VAR2
	templateSrc := filepath.Join(tmp, "src")
	os.MkdirAll(templateSrc, 0o755)
	os.WriteFile(filepath.Join(templateSrc, "f.tpl"), []byte("A={{ .VAR1 }} B={{ .VAR2 }}"), 0o644)
	// stub clone
	origClone := cloneFunc
	cloneFunc = func(repo, ref, cacheDir string) (string, error) {
		dst := filepath.Join(cacheDir, "repo")
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(templateSrc, "f.tpl"))
		os.WriteFile(filepath.Join(dst, "f.tpl"), b, 0o644)
		return dst, nil
	}
	defer func() { cloneFunc = origClone }()
	// strict -> error
	strictCfg := &config.DuckConf{Version: 1, Default: config.Target{Name: "build", Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "f.tpl"}, Variables: map[string]config.VarValue{"VAR1": config.NewLiteralVar("one")}}}
	if err := Sync(strictCfg, "default", false); err == nil {
		t.Fatalf("expected error for missing variable in strict mode")
	}
	// allowMissing -> empty value
	allowCfg := &config.DuckConf{Version: 1, Default: config.Target{Name: "build", Binary: "echo", FileFlag: "-f", Template: config.Template{Repo: "stub", Path: "f.tpl", AllowMissing: true}, Variables: map[string]config.VarValue{"VAR1": config.NewLiteralVar("one")}}}
	// stub exec so Exec path safe
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("true") }
	defer func() { execCommand = origExec }()
	if err := Sync(allowCfg, "default", false); err != nil {
		t.Fatalf("allowMissing sync err: %v", err)
	}
	link := filepath.Join(".duck", "default", "f")
	target, _ := os.Readlink(link)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	b, _ := os.ReadFile(target)
	if !strings.Contains(string(b), "B=") {
		t.Fatalf("expected empty B= got %q", string(b))
	}
}
