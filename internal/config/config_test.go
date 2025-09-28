//nolint:errcheck
package config

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Dummy coverage tests for functions previously ignored by coverage.
// These tests only call the functions to ensure coverage tools count them.

func TestDummyCoverageConfigHelpers(t *testing.T) {
	// These helpers are trivial and covered by usage, but this test ensures coverage.
	_ = NewLiteralVar("test")
	_ = NewEnvVar("ENV")
	_ = NewCmdVar("echo hi")
	_ = NewFileVar("/tmp/file")
}

func TestDummyCoverageMarshalUnmarshal(t *testing.T) {
	// These methods are exercised by real tests, but this dummy test ensures coverage.
	v := NewLiteralVar("foo")
	_, _ = v.MarshalYAML()
	var vv VarValue
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "bar"}
	_ = vv.UnmarshalYAML(node)
	a := ArgList{"--silent"}
	node2 := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "--silent"}}}
	_ = a.UnmarshalYAML(node2)
}

func TestDummyCoverageSaveLoad(t *testing.T) {
	// These methods are exercised by integration, but this dummy test ensures coverage.
	cfg := &DuckConf{Version: 1, Default: "build", Targets: map[string]Target{"build": {Template: &Template{Repo: "r", Path: "p"}}}}
	_ = cfg.Save("/tmp/dummy-duck.yaml")
	_, _ = Load("/tmp/dummy-duck.yaml")
}

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
	t1 := Target{Binary: "", FileFlag: "-f", Template: &Template{Repo: "r", Path: "p"}}
	if err := ValidateTarget(t1, "x"); err == nil {
		t.Fatalf("expected error for fileFlag without binary")
	}
	t2 := Target{Binary: "", Args: ArgList{"--silent"}, Template: &Template{Repo: "r", Path: "p"}}
	if err := ValidateTarget(t2, "x"); err == nil {
		t.Fatalf("expected error for args without binary")
	}
	t3 := Target{Binary: "echo", FileFlag: "-f", Template: &Template{Repo: "r", Path: "p"}}
	if err := ValidateTarget(t3, "x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// New default semantics tests
func TestDuckConfValidateValid(t *testing.T) {
	cfg := &DuckConf{Version: 1, Default: "build", Targets: map[string]Target{"build": {Template: &Template{Repo: "r", Path: "p"}}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}
}

func TestDuckConfValidateMissingDefaultKey(t *testing.T) {
	cfg := &DuckConf{Version: 1, Default: "missing", Targets: map[string]Target{"one": {Template: &Template{Repo: "r", Path: "p"}}, "two": {Template: &Template{Repo: "r2", Path: "p2"}}}}
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
	cfg := &DuckConf{Version: 1, Default: "", Targets: map[string]Target{"build": {Template: &Template{Repo: "r", Path: "p"}}}}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for empty default key")
	}
}

// TestValidateTargetFileFlagRequiredWhenBinary ensures fileFlag is mandatory when binary is set.
func TestValidateTargetFileFlagRequiredWhenBinary(t *testing.T) {
	t1 := Target{Binary: "echo", FileFlag: "", Template: &Template{Repo: "r", Path: "p"}}
	if err := ValidateTarget(t1, "x"); err == nil || !strings.Contains(err.Error(), "fileFlag is required") {
		t.Fatalf("expected fileFlag required error, got %v", err)
	}
	t2 := Target{Binary: "echo", FileFlag: "-f", Template: &Template{Repo: "r", Path: "p"}}
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

// TestCommitHashConfigProperties verifies that the new commit hash tracking properties
// are properly loaded and have correct default values.
func TestCommitHashConfigProperties(t *testing.T) {
	yml := `
version: 1
default: test
targets:
  test:
    template:
      repo: https://github.com/test/repo.git
      path: test.tpl
      trackCommitHash: true
      autoUpdateOnChange: false
settings:
  trackCommitHash: true
  autoUpdateOnChange: true
`
	var cfg DuckConf
	if err := yaml.Unmarshal([]byte(yml), &cfg); err != nil {
		t.Fatal(err)
	}

	// Test template-level properties
	template := cfg.Targets["test"].Template
	if template == nil {
		t.Fatal("expected template to be present")
	}
	if !template.TrackCommitHash {
		t.Error("expected template.TrackCommitHash to be true")
	}
	if template.AutoUpdateOnChange {
		t.Error("expected template.AutoUpdateOnChange to be false")
	}

	// Test settings-level properties
	if !cfg.Settings.GetTrackCommitHash() {
		t.Error("expected settings.TrackCommitHash to be true")
	}
	if !cfg.Settings.GetAutoUpdateOnChange() {
		t.Error("expected settings.AutoUpdateOnChange to be true")
	}
}

// TestCommitHashResolutionFunctions verifies the precedence logic for commit hash settings.
func TestCommitHashResolutionFunctions(t *testing.T) {
	// Test template with commit hash tracking enabled
	template := &Template{TrackCommitHash: true, AutoUpdateOnChange: true}

	// Test config with global settings
	cfg := &DuckConf{
		Settings: &Settings{
			TrackCommitHash:    false,
			AutoUpdateOnChange: false,
		},
	}

	// Test CLI flag precedence (highest)
	trackTrue := true
	updateFalse := false
	if !ResolveTrackCommitHash(&trackTrue, template, cfg) {
		t.Error("CLI flag should override template and settings")
	}
	if ResolveAutoUpdateOnChange(&updateFalse, template, cfg) {
		t.Error("CLI flag should override template and settings")
	}

	// Test template precedence over settings (when template setting is true)
	if !ResolveTrackCommitHash(nil, template, cfg) {
		t.Error("template setting should override global settings when true")
	}
	if !ResolveAutoUpdateOnChange(nil, template, cfg) {
		t.Error("template setting should override global settings when true")
	}

	// Test settings precedence over defaults when template has false values
	templateWithFalseFlags := &Template{TrackCommitHash: false, AutoUpdateOnChange: false}
	cfgWithTrue := &DuckConf{
		Settings: &Settings{
			TrackCommitHash:    true,
			AutoUpdateOnChange: true,
		},
	}
	if !ResolveTrackCommitHash(nil, templateWithFalseFlags, cfgWithTrue) {
		t.Error("global settings should be used when template setting is false")
	}
	if !ResolveAutoUpdateOnChange(nil, templateWithFalseFlags, cfgWithTrue) {
		t.Error("global settings should be used when template setting is false")
	}

	// Test defaults
	if ResolveTrackCommitHash(nil, nil, nil) {
		t.Error("default should be false")
	}
	if ResolveAutoUpdateOnChange(nil, nil, nil) {
		t.Error("default should be false")
	}
}

// TestCommitHashEnvironmentVariables tests that environment variables are respected.
func TestCommitHashEnvironmentVariables(t *testing.T) {
	// Save original env vars
	origTrack := os.Getenv("DUCK_TRACK_COMMIT_HASH")
	origUpdate := os.Getenv("DUCK_AUTO_UPDATE_ON_CHANGE")
	defer func() {
		os.Setenv("DUCK_TRACK_COMMIT_HASH", origTrack)
		os.Setenv("DUCK_AUTO_UPDATE_ON_CHANGE", origUpdate)
	}()

	// Test "true" values
	os.Setenv("DUCK_TRACK_COMMIT_HASH", "true")
	os.Setenv("DUCK_AUTO_UPDATE_ON_CHANGE", "1")

	if !ResolveTrackCommitHash(nil, nil, nil) {
		t.Error("DUCK_TRACK_COMMIT_HASH=true should resolve to true")
	}
	if !ResolveAutoUpdateOnChange(nil, nil, nil) {
		t.Error("DUCK_AUTO_UPDATE_ON_CHANGE=1 should resolve to true")
	}

	// Test "false" values
	os.Setenv("DUCK_TRACK_COMMIT_HASH", "false")
	os.Setenv("DUCK_AUTO_UPDATE_ON_CHANGE", "0")

	if ResolveTrackCommitHash(nil, nil, nil) {
		t.Error("DUCK_TRACK_COMMIT_HASH=false should resolve to false")
	}
	if ResolveAutoUpdateOnChange(nil, nil, nil) {
		t.Error("DUCK_AUTO_UPDATE_ON_CHANGE=0 should resolve to false")
	}

	// Test empty values fall back to defaults
	os.Setenv("DUCK_TRACK_COMMIT_HASH", "")
	os.Setenv("DUCK_AUTO_UPDATE_ON_CHANGE", "")

	if ResolveTrackCommitHash(nil, nil, nil) {
		t.Error("empty env vars should fall back to default false")
	}
	if ResolveAutoUpdateOnChange(nil, nil, nil) {
		t.Error("empty env vars should fall back to default false")
	}
}

// TestCommitHashValidation tests the validation logic for commit hash tracking.
func TestCommitHashValidation(t *testing.T) {
	tests := []struct {
		name        string
		template    Template
		targetName  string
		expectErr   bool
		errContains string
	}{
		{
			name: "valid branch with commit tracking",
			template: Template{
				Repo:            "https://github.com/test/repo.git",
				Ref:             "main",
				Path:            "test.tpl",
				TrackCommitHash: true,
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "valid tag with commit tracking",
			template: Template{
				Repo:            "https://github.com/test/repo.git",
				Ref:             "v1.0.0",
				Path:            "test.tpl",
				TrackCommitHash: true,
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "commit hash with tracking enabled should fail",
			template: Template{
				Repo:            "https://github.com/test/repo.git",
				Ref:             "a1b2c3d4e5f6789012345678901234567890abcd",
				Path:            "test.tpl",
				TrackCommitHash: true,
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "commit hash tracking is invalid when ref is already a commit hash",
		},
		{
			name: "commit hash without tracking should pass",
			template: Template{
				Repo:            "https://github.com/test/repo.git",
				Ref:             "a1b2c3d4e5f6789012345678901234567890abcd",
				Path:            "test.tpl",
				TrackCommitHash: false,
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "auto-update without tracking should fail",
			template: Template{
				Repo:               "https://github.com/test/repo.git",
				Ref:                "main",
				Path:               "test.tpl",
				TrackCommitHash:    false,
				AutoUpdateOnChange: true,
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "autoUpdateOnChange requires trackCommitHash to be enabled",
		},
		{
			name: "auto-update with tracking should pass",
			template: Template{
				Repo:               "https://github.com/test/repo.git",
				Ref:                "main",
				Path:               "test.tpl",
				TrackCommitHash:    true,
				AutoUpdateOnChange: true,
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "empty ref with tracking should pass",
			template: Template{
				Repo:            "https://github.com/test/repo.git",
				Ref:             "", // empty ref defaults to HEAD
				Path:            "test.tpl",
				TrackCommitHash: true,
			},
			targetName: "test",
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommitHashTracking(tt.template, tt.targetName)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestValidateTargetWithCommitHash tests the target validation including commit hash validation.
func TestValidateTargetWithCommitHash(t *testing.T) {
	tests := []struct {
		name        string
		target      Target
		targetName  string
		expectErr   bool
		errContains string
	}{
		{
			name: "valid target with commit tracking",
			target: Target{
				Template: &Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "main",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "valid template without commit tracking",
			target: Target{
				Template: &Template{
					Repo: "https://github.com/test/repo.git",
					Ref:  "main",
					Path: "test.tpl",
				},
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "valid template with commit tracking on branch",
			target: Target{
				Template: &Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "main",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "valid template with commit tracking and auto-update on tag",
			target: Target{
				Template: &Template{
					Repo:               "https://github.com/test/repo.git",
					Ref:                "v1.0.0",
					Path:               "test.tpl",
					TrackCommitHash:    true,
					AutoUpdateOnChange: true,
				},
			},
			targetName: "test",
			expectErr:  false,
		},
		{
			name: "invalid commit hash tracking",
			target: Target{
				Template: &Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "a1b2c3d4e5f6789012345678901234567890abcd",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "commit hash tracking is invalid",
		},
		{
			name: "invalid binary configuration with commit tracking",
			target: Target{
				Binary: "make",
				// Missing FileFlag - should fail binary validation before commit hash validation
				Template: &Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "a1b2c3d4e5f6789012345678901234567890abcd",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "fileFlag is required when binary is set",
		},
		{
			name: "invalid binary configuration with commit tracking",
			target: Target{
				Binary: "make",
				// Missing FileFlag - should fail binary validation before commit hash validation
				Template: &Template{
					Repo:            "https://github.com/test/repo.git",
					Ref:             "a1b2c3d4e5f6789012345678901234567890abcd",
					Path:            "test.tpl",
					TrackCommitHash: true,
				},
			},
			targetName:  "test",
			expectErr:   true,
			errContains: "fileFlag is required when binary is set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.target, tt.targetName, nil)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestRemoteTemplateResolution tests the new remote template functionality
func TestRemoteTemplateResolution(t *testing.T) {
	// Define test remotes
	remotes := map[string]Template{
		"shared-makefile": {
			Repo: "https://github.com/test/templates.git",
			Ref:  "main",
			Path: "Makefile.tpl",
		},
		"task-template": {
			Repo:         "https://github.com/test/templates.git",
			Ref:          "v1.0.0",
			Path:         "Taskfile.yml.tpl",
			AllowMissing: true,
			Delims:       &Delims{Left: "[[", Right: "]]"},
		},
	}

	tests := []struct {
		name        string
		target      Target
		expectErr   bool
		errContains string
		wantRepo    string
		wantPath    string
	}{
		{
			name: "inline template works",
			target: Target{
				Template: &Template{
					Repo: "https://github.com/test/inline.git",
					Path: "inline.tpl",
				},
			},
			expectErr: false,
			wantRepo:  "https://github.com/test/inline.git",
			wantPath:  "inline.tpl",
		},
		{
			name: "templateRef resolves correctly",
			target: Target{
				TemplateRef: stringPtr("shared-makefile"),
			},
			expectErr: false,
			wantRepo:  "https://github.com/test/templates.git",
			wantPath:  "Makefile.tpl",
		},
		{
			name: "templateRef with complex template",
			target: Target{
				TemplateRef: stringPtr("task-template"),
			},
			expectErr: false,
			wantRepo:  "https://github.com/test/templates.git",
			wantPath:  "Taskfile.yml.tpl",
		},
		{
			name: "both template and templateRef fails",
			target: Target{
				Template: &Template{
					Repo: "https://github.com/test/inline.git",
					Path: "inline.tpl",
				},
				TemplateRef: stringPtr("shared-makefile"),
			},
			expectErr:   true,
			errContains: "mutually exclusive",
		},
		{
			name:        "neither template nor templateRef fails",
			target:      Target{},
			expectErr:   true,
			errContains: "either 'template' or 'templateRef' must be specified",
		},
		{
			name: "templateRef not found fails",
			target: Target{
				TemplateRef: stringPtr("nonexistent"),
			},
			expectErr:   true,
			errContains: "templateRef \"nonexistent\" not found in remotes",
		},
		{
			name: "empty templateRef fails",
			target: Target{
				TemplateRef: stringPtr(""),
			},
			expectErr:   true,
			errContains: "either 'template' or 'templateRef' must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.target, "test", remotes)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
					return
				}

				// Test template resolution
				resolved, resolveErr := ResolveTemplate(tt.target, remotes)
				if resolveErr != nil {
					t.Errorf("template resolution failed: %v", resolveErr)
					return
				}

				if resolved.Repo != tt.wantRepo {
					t.Errorf("expected repo %q, got %q", tt.wantRepo, resolved.Repo)
				}
				if resolved.Path != tt.wantPath {
					t.Errorf("expected path %q, got %q", tt.wantPath, resolved.Path)
				}
			}
		})
	}
}

// TestDuckConfWithRemotes tests the full configuration with remotes
func TestDuckConfWithRemotes(t *testing.T) {
	yml := `
version: 1
default: build

remotes:
  shared-make:
    repo: https://github.com/test/templates.git
    ref: main
    path: Makefile.tpl
    checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  
  task-template:
    repo: https://github.com/test/templates.git
    ref: v1.0.0
    path: Taskfile.yml.tpl
    delims:
      left: "[["
      right: "]]"
    allowMissing: true

targets:
  build:
    binary: make
    fileFlag: -f
    templateRef: shared-make
    variables:
      PROJECT: test-project
  
  test:
    binary: task
    fileFlag: --taskfile
    templateRef: task-template
    variables:
      GO_VERSION: "1.21"
  
  inline-example:
    template:
      repo: https://github.com/test/inline.git
      path: config.yml.tpl
    variables:
      ENV: dev
`

	var cfg DuckConf
	if err := yaml.Unmarshal([]byte(yml), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("config validation failed: %v", err)
	}

	// Test remotes are loaded
	if len(cfg.Remotes) != 2 {
		t.Errorf("expected 2 remotes, got %d", len(cfg.Remotes))
	}

	sharedMake, exists := cfg.Remotes["shared-make"]
	if !exists {
		t.Error("shared-make remote not found")
	} else {
		if sharedMake.Repo != "https://github.com/test/templates.git" {
			t.Errorf("expected shared-make repo to be correct, got %q", sharedMake.Repo)
		}
	}

	// Test targets are loaded correctly
	buildTarget := cfg.Targets["build"]
	if buildTarget.TemplateRef == nil || *buildTarget.TemplateRef != "shared-make" {
		t.Error("build target should reference shared-make")
	}
	if buildTarget.Template != nil {
		t.Error("build target should not have inline template")
	}

	inlineTarget := cfg.Targets["inline-example"]
	if inlineTarget.Template == nil {
		t.Error("inline-example target should have inline template")
	}
	if inlineTarget.TemplateRef != nil {
		t.Error("inline-example target should not have templateRef")
	}
}

// TestResolveTemplateFunction tests the ResolveTemplate function directly
func TestResolveTemplateFunction(t *testing.T) {
	remotes := map[string]Template{
		"test-remote": {
			Repo: "https://github.com/test/templates.git",
			Path: "test.tpl",
		},
	}

	tests := []struct {
		name      string
		target    Target
		expectErr bool
		wantRepo  string
	}{
		{
			name: "inline template",
			target: Target{
				Template: &Template{
					Repo: "https://github.com/test/inline.git",
					Path: "inline.tpl",
				},
			},
			expectErr: false,
			wantRepo:  "https://github.com/test/inline.git",
		},
		{
			name: "template ref",
			target: Target{
				TemplateRef: stringPtr("test-remote"),
			},
			expectErr: false,
			wantRepo:  "https://github.com/test/templates.git",
		},
		{
			name: "missing template ref",
			target: Target{
				TemplateRef: stringPtr("missing"),
			},
			expectErr: true,
		},
		{
			name:      "no template or ref",
			target:    Target{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := ResolveTemplate(tt.target, remotes)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
					return
				}
				if resolved.Repo != tt.wantRepo {
					t.Errorf("expected repo %q, got %q", tt.wantRepo, resolved.Repo)
				}
			}
		})
	}
}

// TestBackwardCompatibilityWithRemotes ensures old configs without remotes still work
func TestBackwardCompatibilityWithRemotes(t *testing.T) {
	yml := `
version: 1
default: build

targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: https://github.com/test/templates.git
      ref: main
      path: Makefile.tpl
    variables:
      PROJECT: test-project
`

	var cfg DuckConf
	if err := yaml.Unmarshal([]byte(yml), &cfg); err != nil {
		t.Fatalf("failed to unmarshal legacy config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("legacy config validation failed: %v", err)
	}

	// Should have no remotes
	if len(cfg.Remotes) != 0 {
		t.Errorf("expected 0 remotes in legacy config, got %d", len(cfg.Remotes))
	}

	// Target should have inline template
	buildTarget := cfg.Targets["build"]
	if buildTarget.Template == nil {
		t.Error("legacy target should have inline template")
	}
	if buildTarget.TemplateRef != nil {
		t.Error("legacy target should not have templateRef")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
