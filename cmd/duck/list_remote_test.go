package main

import (
	"strings"
	"testing"
)

// TestListCommandRemoteReferences tests the list command with remote references
func TestListCommandRemoteReferences(t *testing.T) {
	dir := t.TempDir()

	// Create a config with both inline templates and remote references
	configContent := `version: 1
default: webapp

remotes:
  shared-k8s:
    repo: https://github.com/test/k8s-templates.git
    ref: v2.0.0
    path: deployment.yaml.tpl
    checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  
  shared-docker:
    repo: https://github.com/test/docker-templates.git
    ref: main
    path: Dockerfile.tpl
    allowMissing: true

targets:
  webapp:
    binary: kubectl
    fileFlag: -f
    templateRef: shared-k8s
    variables:
      APP_NAME: 
        value: webapp
      PORT:
        value: 3000

  api:
    binary: kubectl
    fileFlag: -f
    templateRef: shared-k8s
    variables:
      APP_NAME:
        value: api
      PORT:
        value: 8080

  monitoring:
    binary: docker
    fileFlag: -f
    template:
      repo: https://github.com/test/monitoring.git
      ref: stable
      path: docker-compose.yml.tpl
    variables:
      ENV:
        value: production

  docs:
    # No binary - sync only target
    templateRef: shared-docker
`

	writeConfig(t, dir, configContent)

	// Test 1: Basic list without flags
	t.Run("basic_list", func(t *testing.T) {
		out := runList(t, dir)

		// Should show target names and binaries
		if !strings.Contains(out, "webapp*") {
			t.Errorf("expected webapp* (default), got: %s", out)
		}
		if !strings.Contains(out, "api") {
			t.Errorf("expected api target, got: %s", out)
		}
		if !strings.Contains(out, "monitoring") {
			t.Errorf("expected monitoring target, got: %s", out)
		}
		if !strings.Contains(out, "docs") {
			t.Errorf("expected docs target, got: %s", out)
		}

		// Should NOT show detailed remote info without --remote flag
		if strings.Contains(out, "templateRef") || strings.Contains(out, "shared-k8s") {
			t.Errorf("should not show remote details without --remote flag, got: %s", out)
		}
	})

	// Test 2: List with --remote flag
	t.Run("remote_flag", func(t *testing.T) {
		out := runList(t, dir, "--remote")

		// Should show remote reference information
		if !strings.Contains(out, "remote: shared-k8s (references shared template)") {
			t.Errorf("expected remote reference info for webapp, got: %s", out)
		}
		if !strings.Contains(out, "repo: https://github.com/test/k8s-templates.git") {
			t.Errorf("expected repo URL for shared template, got: %s", out)
		}
		if !strings.Contains(out, "path: deployment.yaml.tpl") {
			t.Errorf("expected template path, got: %s", out)
		}

		// Should show inline template info
		if !strings.Contains(out, "template: inline") {
			t.Errorf("expected inline template indicator for monitoring, got: %s", out)
		}
		if !strings.Contains(out, "repo: https://github.com/test/monitoring.git") {
			t.Errorf("expected inline template repo, got: %s", out)
		}
	})

	// Test 3: List with --vars flag
	t.Run("vars_flag", func(t *testing.T) {
		out := runList(t, dir, "--vars")

		// Should show variable information
		if !strings.Contains(out, "variables (2):") {
			t.Errorf("expected variable count for webapp, got: %s", out)
		}
		if !strings.Contains(out, "APP_NAME (literal)") {
			t.Errorf("expected APP_NAME variable, got: %s", out)
		}
		if !strings.Contains(out, "PORT (literal)") {
			t.Errorf("expected PORT variable, got: %s", out)
		}
	})

	// Test 4: Combined --remote and --vars flags
	t.Run("remote_and_vars", func(t *testing.T) {
		out := runList(t, dir, "--remote", "--vars")

		// Should show both remote and variable information
		if !strings.Contains(out, "remote: shared-k8s (references shared template)") {
			t.Errorf("expected remote reference info, got: %s", out)
		}
		if !strings.Contains(out, "variables (2):") {
			t.Errorf("expected variable information, got: %s", out)
		}
		if !strings.Contains(out, "template: inline") {
			t.Errorf("expected inline template indicator, got: %s", out)
		}
	})

	// Test 5: Sync-only target (no binary)
	t.Run("sync_only_target", func(t *testing.T) {
		out := runList(t, dir, "--remote")

		// docs target should show up with no binary
		if !strings.Contains(out, "docs         -") {
			t.Errorf("expected docs target with no binary, got: %s", out)
		}

		// Should still show remote reference for sync-only target
		docLines := extractTargetLines(out, "docs")
		foundRemoteRef := false
		for _, line := range docLines {
			if strings.Contains(line, "remote: shared-docker") {
				foundRemoteRef = true
				break
			}
		}
		if !foundRemoteRef {
			t.Errorf("expected remote reference for docs target, got: %s", out)
		}
	})
}

// TestListCommandRemoteReferencesOnly tests a config with only remote references
func TestListCommandRemoteReferencesOnly(t *testing.T) {
	dir := t.TempDir()

	configContent := `version: 1
default: web

remotes:
  universal-template:
    repo: https://github.com/company/universal.git
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

  worker:
    binary: kubectl
    fileFlag: -f
    templateRef: universal-template
    variables:
      SERVICE_TYPE:
        value: worker
`

	writeConfig(t, dir, configContent)

	out := runList(t, dir, "--remote")

	// Both targets should reference the same remote template
	webLines := extractTargetLines(out, "web")
	workerLines := extractTargetLines(out, "worker")

	// Check that both targets show the same remote reference
	if !containsLine(webLines, "remote: universal-template (references shared template)") {
		t.Errorf("web target should reference universal-template, got: %s", strings.Join(webLines, "\n"))
	}
	if !containsLine(workerLines, "remote: universal-template (references shared template)") {
		t.Errorf("worker target should reference universal-template, got: %s", strings.Join(workerLines, "\n"))
	}

	// Both should show the same resolved repo info
	if !containsLine(webLines, "repo: https://github.com/company/universal.git") {
		t.Errorf("web target should show resolved repo, got: %s", strings.Join(webLines, "\n"))
	}
	if !containsLine(workerLines, "repo: https://github.com/company/universal.git") {
		t.Errorf("worker target should show resolved repo, got: %s", strings.Join(workerLines, "\n"))
	}
}

// Helper functions for test assertions

// extractTargetLines extracts all lines related to a specific target from list output
func extractTargetLines(output, targetName string) []string {
	lines := strings.Split(output, "\n")
	var targetLines []string
	inTarget := false

	for _, line := range lines {
		if strings.HasPrefix(line, targetName) {
			inTarget = true
			targetLines = append(targetLines, line)
		} else if inTarget && strings.HasPrefix(line, "    ") {
			// Indented lines belong to the current target
			targetLines = append(targetLines, line)
		} else if inTarget && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "    ") {
			// Non-indented, non-empty line starts a new target
			break
		}
	}

	return targetLines
}

// containsLine checks if any line in the slice contains the given substring
func containsLine(lines []string, substring string) bool {
	for _, line := range lines {
		if strings.Contains(line, substring) {
			return true
		}
	}
	return false
}
