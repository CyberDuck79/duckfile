package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRemoteReferenceValidation tests comprehensive validation scenarios for remote references
func TestRemoteReferenceValidation(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_remote_references",
			config: `
version: 1
default: web

remotes:
  k8s-template:
    repo: https://github.com/test/templates.git
    ref: v1.0.0
    path: deployment.yaml.tpl
  
  docker-template:
    repo: https://github.com/test/docker.git
    ref: main
    path: Dockerfile.tpl

targets:
  web:
    binary: kubectl
    fileFlag: -f
    templateRef: k8s-template
  
  app:
    binary: docker
    fileFlag: -f
    templateRef: docker-template
`,
			expectError: false,
		},
		{
			name: "missing_remote_reference",
			config: `
version: 1
default: web

remotes:
  existing-template:
    repo: https://github.com/test/templates.git
    ref: v1.0.0
    path: template.yaml.tpl

targets:
  web:
    binary: kubectl
    fileFlag: -f
    templateRef: nonexistent-template
`,
			expectError:   true,
			errorContains: "templateRef \"nonexistent-template\" not found in remotes",
		},
		{
			name: "empty_templateRef",
			config: `
version: 1
default: web

remotes:
  valid-template:
    repo: https://github.com/test/templates.git
    ref: v1.0.0
    path: template.yaml.tpl

targets:
  web:
    binary: kubectl
    fileFlag: -f
    templateRef: ""
`,
			expectError:   true,
			errorContains: "either 'template' or 'templateRef' must be specified",
		},
		{
			name: "both_template_and_templateRef",
			config: `
version: 1
default: web

remotes:
  remote-template:
    repo: https://github.com/test/templates.git
    ref: v1.0.0
    path: template.yaml.tpl

targets:
  web:
    binary: kubectl
    fileFlag: -f
    template:
      repo: https://github.com/test/inline.git
      path: inline.yaml.tpl
    templateRef: remote-template
`,
			expectError:   true,
			errorContains: "'template' and 'templateRef' are mutually exclusive",
		},
		{
			name: "neither_template_nor_templateRef",
			config: `
version: 1
default: web

targets:
  web:
    binary: kubectl
    fileFlag: -f
`,
			expectError:   true,
			errorContains: "either 'template' or 'templateRef' must be specified",
		},
		{
			name: "valid_mixed_targets",
			config: `
version: 1
default: remote-target

remotes:
  shared-template:
    repo: https://github.com/test/shared.git
    ref: v2.0.0
    path: shared.yaml.tpl

targets:
  remote-target:
    binary: kubectl
    fileFlag: -f
    templateRef: shared-template
  
  inline-target:
    binary: docker
    fileFlag: -f
    template:
      repo: https://github.com/test/inline.git
      path: inline.tpl
`,
			expectError: false,
		},
		{
			name: "invalid_remote_definition",
			config: `
version: 1
default: web

remotes:
  invalid-remote:
    repo: ""
    path: template.yaml.tpl

targets:
  web:
    binary: kubectl
    fileFlag: -f
    templateRef: invalid-remote
`,
			expectError:   true,
			errorContains: "repository is required",
		},
		{
			name: "remote_missing_path",
			config: `
version: 1
default: web

remotes:
  no-path-remote:
    repo: https://github.com/test/templates.git
    ref: v1.0.0

targets:
  web:
    binary: kubectl
    fileFlag: -f
    templateRef: no-path-remote
`,
			expectError:   true,
			errorContains: "path is required",
		},
		{
			name: "sync_only_target_with_remote_ref",
			config: `
version: 1
default: docs

remotes:
  doc-template:
    repo: https://github.com/test/docs.git
    ref: v1.0.0
    path: README.md.tpl

targets:
  docs:
    templateRef: doc-template
`,
			expectError: false,
		},
		{
			name: "multiple_targets_same_remote",
			config: `
version: 1
default: web

remotes:
  universal-template:
    repo: https://github.com/test/universal.git
    ref: v1.0.0
    path: app.yaml.tpl

targets:
  web:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-template
    variables:
      SERVICE_TYPE:
        value: web
  
  api:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-template
    variables:
      SERVICE_TYPE:
        value: api
  
  worker:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-template
    variables:
      SERVICE_TYPE:
        value: worker
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg DuckConf
			err := yaml.Unmarshal([]byte(tt.config), &cfg)
			if err != nil {
				t.Fatalf("failed to unmarshal config: %v", err)
			}

			err = cfg.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestRemoteTemplateInheritance tests that remote template properties are properly inherited
func TestRemoteTemplateInheritance(t *testing.T) {
	config := `
version: 1
default: test

remotes:
  full-template:
    repo: https://github.com/test/templates.git
    ref: v2.1.0
    path: full.yaml.tpl
    checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    allowMissing: true
    submodules: true
    delims:
      left: "{{"
      right: "}}"
    trackCommitHash: true
    autoUpdateOnChange: false

targets:
  test:
    binary: kubectl
    fileFlag: -f
    templateRef: full-template
`

	var cfg DuckConf
	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("config validation failed: %v", err)
	}

	// Resolve the template to verify inheritance
	target := cfg.Targets["test"]
	resolved, err := ResolveTemplate(target, cfg.Remotes)
	if err != nil {
		t.Fatalf("template resolution failed: %v", err)
	}

	// Verify all properties are inherited
	if resolved.Repo != "https://github.com/test/templates.git" {
		t.Errorf("expected repo to be inherited, got %q", resolved.Repo)
	}
	if resolved.Ref != "v2.1.0" {
		t.Errorf("expected ref to be inherited, got %q", resolved.Ref)
	}
	if resolved.Path != "full.yaml.tpl" {
		t.Errorf("expected path to be inherited, got %q", resolved.Path)
	}
	if resolved.Checksum != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("expected checksum to be inherited, got %q", resolved.Checksum)
	}
	if !resolved.AllowMissing {
		t.Error("expected allowMissing to be inherited as true")
	}
	if !resolved.Submodules {
		t.Error("expected submodules to be inherited as true")
	}
	if resolved.Delims == nil || resolved.Delims.Left != "{{" || resolved.Delims.Right != "}}" {
		t.Errorf("expected delims to be inherited, got %+v", resolved.Delims)
	}
	if !resolved.TrackCommitHash {
		t.Error("expected trackCommitHash to be inherited as true")
	}
	if resolved.AutoUpdateOnChange {
		t.Error("expected autoUpdateOnChange to be inherited as false")
	}
}

// TestRemoteReferenceEdgeCases tests edge cases and boundary conditions
func TestRemoteReferenceEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectError   bool
		errorContains string
	}{
		{
			name: "whitespace_only_templateRef",
			config: `
version: 1
default: test

remotes:
  valid-remote:
    repo: https://github.com/test/templates.git
    path: template.tpl

targets:
  test:
    templateRef: "   "
    binary: echo
    fileFlag: -f
`,
			expectError:   true,
			errorContains: "either 'template' or 'templateRef' must be specified",
		},
		{
			name: "null_templateRef",
			config: `
version: 1
default: test

remotes:
  valid-remote:
    repo: https://github.com/test/templates.git
    path: template.tpl

targets:
  test:
    templateRef: null
    binary: echo
    fileFlag: -f
`,
			expectError:   true,
			errorContains: "either 'template' or 'templateRef' must be specified",
		},
		{
			name: "empty_remotes_section",
			config: `
version: 1
default: test

remotes: {}

targets:
  test:
    template:
      repo: https://github.com/test/inline.git
      path: inline.tpl
    binary: echo
    fileFlag: -f
`,
			expectError: false,
		},
		{
			name: "no_remotes_section_with_templateRef",
			config: `
version: 1
default: test

targets:
  test:
    templateRef: missing-remote
    binary: echo
    fileFlag: -f
`,
			expectError:   true,
			errorContains: "templateRef \"missing-remote\" not found in remotes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg DuckConf
			err := yaml.Unmarshal([]byte(tt.config), &cfg)
			if err != nil {
				t.Fatalf("failed to unmarshal config: %v", err)
			}

			err = cfg.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}
