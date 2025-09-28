package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRemoteReferenceIntegration provides comprehensive integration testing for the remote reference feature
func TestRemoteReferenceIntegration(t *testing.T) {
	t.Run("complete_remote_reference_workflow", func(t *testing.T) {
		// Test a complete configuration with all remote reference features
		config := `
version: 1
default: webapp

remotes:
  k8s-deployment:
    repo: https://github.com/company/k8s-templates.git
    ref: v2.1.0
    path: deployment.yaml.tpl
    checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    allowMissing: true
    delims:
      left: "{{"
      right: "}}"
  
  docker-config:
    repo: https://github.com/company/docker-templates.git
    ref: main
    path: Dockerfile.tpl
    submodules: true

targets:
  # Multiple targets sharing the same remote template
  webapp:
    binary: kubectl
    fileFlag: -f
    templateRef: k8s-deployment
    variables:
      APP_NAME: webapp
      REPLICAS: 3
      PORT: 8080

  api:
    binary: kubectl  
    fileFlag: -f
    templateRef: k8s-deployment
    variables:
      APP_NAME: api
      REPLICAS: 2
      PORT: 3000
  
  # Target with different remote reference
  docker-build:
    binary: docker
    fileFlag: -f
    templateRef: docker-config
    variables:
      BASE_IMAGE: node:18
  
  # Mixed: inline template alongside remote references
  monitoring:
    binary: docker
    fileFlag: -f
    template:
      repo: https://github.com/company/monitoring.git
      ref: stable
      path: docker-compose.yml.tpl
    variables:
      ENV: production
  
  # Sync-only target with remote reference
  docs:
    templateRef: docker-config
    variables:
      BASE_IMAGE: nginx:alpine
`

		var cfg DuckConf
		err := yaml.Unmarshal([]byte(config), &cfg)
		if err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		// Validate the complete configuration
		if err := cfg.Validate(); err != nil {
			t.Fatalf("config validation failed: %v", err)
		}

		// Test 1: Verify remotes are loaded correctly
		if len(cfg.Remotes) != 2 {
			t.Errorf("expected 2 remotes, got %d", len(cfg.Remotes))
		}

		k8sTemplate, exists := cfg.Remotes["k8s-deployment"]
		if !exists {
			t.Fatal("k8s-deployment remote not found")
		}
		if k8sTemplate.Repo != "https://github.com/company/k8s-templates.git" {
			t.Errorf("expected k8s repo URL, got %q", k8sTemplate.Repo)
		}
		if k8sTemplate.Ref != "v2.1.0" {
			t.Errorf("expected k8s ref v2.1.0, got %q", k8sTemplate.Ref)
		}

		// Test 2: Verify targets with remote references
		webappTarget := cfg.Targets["webapp"]
		if webappTarget.TemplateRef == nil || *webappTarget.TemplateRef != "k8s-deployment" {
			t.Errorf("webapp should reference k8s-deployment, got %v", webappTarget.TemplateRef)
		}
		if webappTarget.Template != nil {
			t.Error("webapp should not have inline template")
		}

		apiTarget := cfg.Targets["api"]
		if apiTarget.TemplateRef == nil || *apiTarget.TemplateRef != "k8s-deployment" {
			t.Errorf("api should reference k8s-deployment, got %v", apiTarget.TemplateRef)
		}

		// Test 3: Verify inline template still works
		monitoringTarget := cfg.Targets["monitoring"]
		if monitoringTarget.Template == nil {
			t.Error("monitoring should have inline template")
		}
		if monitoringTarget.TemplateRef != nil {
			t.Error("monitoring should not have templateRef")
		}

		// Test 4: Verify sync-only target with remote reference
		docsTarget := cfg.Targets["docs"]
		if docsTarget.TemplateRef == nil || *docsTarget.TemplateRef != "docker-config" {
			t.Errorf("docs should reference docker-config, got %v", docsTarget.TemplateRef)
		}
		if docsTarget.Binary != "" {
			t.Error("docs should be sync-only (no binary)")
		}

		// Test 5: Test template resolution for all target types
		// Remote reference resolution
		resolved, err := ResolveTemplate(webappTarget, cfg.Remotes)
		if err != nil {
			t.Fatalf("failed to resolve webapp template: %v", err)
		}
		if resolved.Repo != k8sTemplate.Repo {
			t.Errorf("resolved template should inherit repo, got %q", resolved.Repo)
		}
		if !resolved.AllowMissing {
			t.Error("resolved template should inherit allowMissing=true")
		}

		// Inline template resolution
		resolved, err = ResolveTemplate(monitoringTarget, cfg.Remotes)
		if err != nil {
			t.Fatalf("failed to resolve monitoring template: %v", err)
		}
		if resolved.Repo != "https://github.com/company/monitoring.git" {
			t.Errorf("inline template should use its own repo, got %q", resolved.Repo)
		}

		// Test 6: Verify variables are preserved
		if len(webappTarget.Variables) != 3 {
			t.Errorf("webapp should have 3 variables, got %d", len(webappTarget.Variables))
		}
		appNameVar, exists := webappTarget.Variables["APP_NAME"]
		if !exists {
			t.Error("APP_NAME variable should exist")
		} else if appNameVar.Value != "webapp" {
			t.Errorf("webapp APP_NAME should be 'webapp', got %v", appNameVar.Value)
		}
	})

	t.Run("shared_remote_cache_efficiency", func(t *testing.T) {
		// Test that multiple targets referencing the same remote would share cache
		config := `
version: 1
default: service1

remotes:
  universal-service:
    repo: https://github.com/company/service-template.git
    ref: v1.0.0
    path: service.yaml.tpl

targets:
  service1:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-service
    variables:
      SERVICE_NAME: service1
  
  service2:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-service
    variables:
      SERVICE_NAME: service2
  
  service3:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-service
    variables:
      SERVICE_NAME: service3
`

		var cfg DuckConf
		err := yaml.Unmarshal([]byte(config), &cfg)
		if err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		if err := cfg.Validate(); err != nil {
			t.Fatalf("config validation failed: %v", err)
		}

		// All three targets should resolve to the same remote template
		universalTemplate := cfg.Remotes["universal-service"]

		for _, targetName := range []string{"service1", "service2", "service3"} {
			target := cfg.Targets[targetName]
			resolved, err := ResolveTemplate(target, cfg.Remotes)
			if err != nil {
				t.Fatalf("failed to resolve %s template: %v", targetName, err)
			}

			// All should resolve to the same repo/ref/path (would share cache)
			if resolved.Repo != universalTemplate.Repo {
				t.Errorf("%s should resolve to shared repo, got %q", targetName, resolved.Repo)
			}
			if resolved.Ref != universalTemplate.Ref {
				t.Errorf("%s should resolve to shared ref, got %q", targetName, resolved.Ref)
			}
			if resolved.Path != universalTemplate.Path {
				t.Errorf("%s should resolve to shared path, got %q", targetName, resolved.Path)
			}
		}
	})
}

// TestRemoteReferenceErrorHandling tests error conditions comprehensively
func TestRemoteReferenceErrorHandling(t *testing.T) {
	errorCases := []struct {
		name          string
		config        string
		expectedError string
	}{
		{
			name: "templateRef_not_found",
			config: `
version: 1
default: test
targets:
  test:
    templateRef: nonexistent
    binary: echo
    fileFlag: -f
`,
			expectedError: "templateRef \"nonexistent\" not found in remotes",
		},
		{
			name: "both_template_and_templateRef",
			config: `
version: 1
default: test
remotes:
  test-remote:
    repo: https://github.com/test/repo.git
    path: test.tpl
targets:
  test:
    template:
      repo: https://github.com/inline/repo.git
      path: inline.tpl
    templateRef: test-remote
    binary: echo
    fileFlag: -f
`,
			expectedError: "'template' and 'templateRef' are mutually exclusive",
		},
		{
			name: "neither_template_nor_templateRef",
			config: `
version: 1
default: test
targets:
  test:
    binary: echo
    fileFlag: -f
`,
			expectedError: "either 'template' or 'templateRef' must be specified",
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg DuckConf
			err := yaml.Unmarshal([]byte(tc.config), &cfg)
			if err != nil {
				t.Fatalf("failed to unmarshal config: %v", err)
			}

			err = cfg.Validate()
			if err == nil {
				t.Fatalf("expected validation error but got none")
			}

			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("expected error containing %q, got %q", tc.expectedError, err.Error())
			}
		})
	}
}

// TestBackwardCompatibilityPreserved ensures existing configurations still work
func TestBackwardCompatibilityPreserved(t *testing.T) {
	// Test that old-style configurations without remotes continue to work
	legacyConfig := `
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
      PROJECT: legacy-project

  test:
    binary: npm
    fileFlag: --
    template:
      repo: https://github.com/test/test-templates.git
      ref: v1.0.0
      path: package.json.tpl
    args: ["test"]
`

	var cfg DuckConf
	err := yaml.Unmarshal([]byte(legacyConfig), &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal legacy config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("legacy config validation failed: %v", err)
	}

	// Should have no remotes
	if len(cfg.Remotes) != 0 {
		t.Errorf("legacy config should have no remotes, got %d", len(cfg.Remotes))
	}

	// All targets should have inline templates
	for name, target := range cfg.Targets {
		if target.Template == nil {
			t.Errorf("legacy target %s should have inline template", name)
		}
		if target.TemplateRef != nil {
			t.Errorf("legacy target %s should not have templateRef", name)
		}
	}

	// Template resolution should work for legacy configs
	buildTarget := cfg.Targets["build"]
	resolved, err := ResolveTemplate(buildTarget, cfg.Remotes)
	if err != nil {
		t.Fatalf("failed to resolve legacy template: %v", err)
	}
	if resolved.Repo != "https://github.com/test/templates.git" {
		t.Errorf("legacy template resolution failed, got repo %q", resolved.Repo)
	}
}
