package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestVarValueUnmarshalBasics verifies that custom YAML tags (!env, !cmd, !file) and
// plain scalar types (string/int/float/bool) are decoded into the expected VarValue
// kind/fields.
func TestVarValueUnmarshalBasics(t *testing.T) {
	yml := `
str: hello
intv: 42
floatv: 3.14
boolt: true
boole: false
envVar: !env HOME
cmdVar: !cmd echo hi
fileVar: !file ./some/path
`
	var raw map[string]VarValue
	if err := yaml.Unmarshal([]byte(yml), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["str"].Kind != VarLiteral || raw["str"].Value != "hello" {
		t.Fatalf("str mismatch: %+v", raw["str"])
	}
	if raw["envVar"].Kind != VarEnv || raw["envVar"].Arg != "HOME" {
		t.Fatalf("env mismatch: %+v", raw["envVar"])
	}
	if raw["cmdVar"].Kind != VarCmd || raw["cmdVar"].Arg != "echo hi" {
		t.Fatalf("cmd mismatch: %+v", raw["cmdVar"])
	}
	if raw["fileVar"].Kind != VarFile || raw["fileVar"].Arg != "./some/path" {
		t.Fatalf("file mismatch: %+v", raw["fileVar"])
	}
}

// TestArgListUnmarshal ensures ArgList accepts a scalar (single arg), an array of
// scalars (multiple args), and an empty string mapping to an empty slice.
func TestArgListUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"args: --silent", []string{"--silent"}},
		{"args: [\"-v\",\"--color\"]", []string{"-v", "--color"}},
		{"args: \"\"", []string{}},
	}
	for _, c := range cases {
		var m struct {
			Args ArgList `yaml:"args"`
		}
		if err := yaml.Unmarshal([]byte(c.in), &m); err != nil {
			t.Fatalf("unmarshal %q: %v", c.in, err)
		}
		got := []string(m.Args)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Fatalf("got %v want %v", got, c.want)
		}
	}
}

// TestValidateTargetBinaryRules checks validation rejects fileFlag or args when
// binary is absent, and accepts them when binary is present.
func TestValidateTargetBinaryRules(t *testing.T) {
	t1 := Target{Binary: "", FileFlag: "-f"}
	if err := ValidateTarget(t1, "x"); err == nil {
		t.Fatalf("expected error for fileFlag without binary")
	}
	t2 := Target{Binary: "", Args: ArgList{"--silent"}}
	if err := ValidateTarget(t2, "x"); err == nil {
		t.Fatalf("expected error for args without binary")
	}
	t3 := Target{Binary: "echo", FileFlag: "-f"}
	if err := ValidateTarget(t3, "x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// New default semantics tests
func TestDuckConfValidateValid(t *testing.T) {
	cfg := &DuckConf{Version: 1, Default: "build", Targets: map[string]Target{"build": {Template: Template{Repo: "r", Path: "p"}}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}
}

func TestDuckConfValidateMissingDefaultKey(t *testing.T) {
	cfg := &DuckConf{Version: 1, Default: "missing", Targets: map[string]Target{"one": {Template: Template{Repo: "r", Path: "p"}}, "two": {Template: Template{Repo: "r2", Path: "p2"}}}}
	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected error for missing default reference")
	}
	msg := err.Error()
	if !strings.Contains(msg, "missing") || !strings.Contains(msg, "one") || !strings.Contains(msg, "two") {
		t.Fatalf("expected error listing available targets, got %q", msg)
	}
}

func TestDuckConfValidateEmptyTargets(t *testing.T) {
	cfg := &DuckConf{Version: 1, Default: "build", Targets: map[string]Target{}}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for empty targets map")
	}
}

func TestDuckConfValidateEmptyDefault(t *testing.T) {
	cfg := &DuckConf{Version: 1, Default: "", Targets: map[string]Target{"build": {Template: Template{Repo: "r", Path: "p"}}}}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for empty default key")
	}
}

// TestValidateTargetFileFlagRequiredWhenBinary ensures fileFlag is mandatory when binary is set.
func TestValidateTargetFileFlagRequiredWhenBinary(t *testing.T) {
	t1 := Target{Binary: "echo", FileFlag: ""}
	if err := ValidateTarget(t1, "x"); err == nil || !strings.Contains(err.Error(), "fileFlag is required") {
		t.Fatalf("expected fileFlag required error, got %v", err)
	}
	t2 := Target{Binary: "echo", FileFlag: "-f"}
	if err := ValidateTarget(t2, "x"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// TestArgListScalarVsArrayEquivalence confirms a scalar arg and single-element array produce same ArgList.
func TestArgListScalarVsArrayEquivalence(t *testing.T) {
	y1 := "args: -v"
	y2 := "args: ['-v']"
	var a1, a2 struct {
		Args ArgList `yaml:"args"`
	}
	if err := yaml.Unmarshal([]byte(y1), &a1); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(y2), &a2); err != nil {
		t.Fatal(err)
	}
	if len(a1.Args) != 1 || len(a2.Args) != 1 || a1.Args[0] != a2.Args[0] {
		t.Fatalf("scalar vs array mismatch: %v %v", a1.Args, a2.Args)
	}
}
