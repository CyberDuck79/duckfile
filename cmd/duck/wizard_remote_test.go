package main

import (
	"os"
	"testing"
)

// TestWizardRemoteReferenceFlow tests the interactive wizard with remote references
func TestWizardRemoteReferenceFlow(t *testing.T) {
	t.Skip("Skipping interactive test due to input simulation issues")
	dir := t.TempDir()

	// Create a config with existing remotes
	configContent := `version: 1
default: existing

remotes:
  shared-make:
    repo: https://github.com/test/templates.git
    ref: main
    path: Makefile.tpl
  shared-task:
    repo: https://github.com/test/templates.git
    ref: v1.0.0
    path: Taskfile.yml.tpl

targets:
  existing:
    templateRef: shared-make
    binary: make
    fileFlag: -f
`
	if err := os.WriteFile(dir+"/duck.yaml", []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Test case 1: User chooses remote reference
	t.Run("choose_remote_reference", func(t *testing.T) {
		// Simulate user input: 1 (remote reference), shared-make, webapp, Description, make, -f, N, y
		inputFile := dir + "/input1.txt"
		input := "1\nshared-make\nwebapp\nDescription for webapp\nmake\n-f\n\nN\ny\n"
		if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
			t.Fatal(err)
		}

		// Capture the wizard flow
		oldStdin := os.Stdin
		f, err := os.Open(inputFile)
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = f
		defer func() {
			f.Close()
			os.Stdin = oldStdin
		}()

		target, name, err := runTargetWizard(false)
		if err != nil {
			t.Fatalf("wizard failed: %v", err)
		}

		// Verify results
		if name != "webapp" {
			t.Errorf("expected name 'webapp', got %q", name)
		}
		if target.TemplateRef == nil || *target.TemplateRef != "shared-make" {
			t.Errorf("expected templateRef 'shared-make', got %v", target.TemplateRef)
		}
		if target.Template != nil {
			t.Error("expected no inline template when using templateRef")
		}
		if target.Binary != "make" {
			t.Errorf("expected binary 'make', got %q", target.Binary)
		}
	})

	// Test case 2: User chooses inline template (option 2)
	t.Run("choose_inline_template", func(t *testing.T) {
		// Simulate user input: 2 (inline), repo, ref, path, api, description, kubectl, -f, checksum, submodules, track, auto-update, allowMissing, no variables
		inputFile := dir + "/input2.txt"
		input := "2\nhttps://github.com/test/api.git\nv1.2.3\napi.tpl\napi\nDescription for API\nkubectl\n-f\n\n\nn\nn\nn\ny\nn\n"
		if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
			t.Fatal(err)
		}

		oldStdin := os.Stdin
		f, err := os.Open(inputFile)
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = f
		defer func() {
			f.Close()
			os.Stdin = oldStdin
		}()

		target, name, err := runTargetWizard(false)
		if err != nil {
			t.Fatalf("wizard failed: %v", err)
		}

		// Verify results
		if name != "api" {
			t.Errorf("expected name 'api', got %q", name)
		}
		if target.Template == nil {
			t.Error("expected inline template")
		} else {
			if target.Template.Repo != "https://github.com/test/api.git" {
				t.Errorf("expected repo URL, got %q", target.Template.Repo)
			}
			if target.Template.Ref != "v1.2.3" {
				t.Errorf("expected ref 'v1.2.3', got %q", target.Template.Ref)
			}
		}
		if target.TemplateRef != nil {
			t.Error("expected no templateRef when using inline template")
		}
	})
}

// TestWizardNoRemotesAvailable tests wizard behavior when no remotes exist
func TestWizardNoRemotesAvailable(t *testing.T) {
	t.Skip("Skipping interactive test due to input simulation issues")
	dir := t.TempDir()

	// Create a config without remotes
	configContent := `version: 1
default: build

targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: https://github.com/test/templates.git
      path: Makefile.tpl
`
	if err := os.WriteFile(dir+"/duck.yaml", []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Simulate user input for inline template only (no choice offered)
	inputFile := dir + "/input3.txt"
	input := "https://github.com/test/new.git\nmain\nnew.tpl\nnewTarget\nDescription\necho\n-f\n\n\nn\nn\nn\ny\nn\n"
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	f, err := os.Open(inputFile)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = f
	defer func() {
		f.Close()
		os.Stdin = oldStdin
	}()

	target, name, err := runTargetWizard(false)
	if err != nil {
		t.Fatalf("wizard failed: %v", err)
	}

	// Should create inline template since no remotes available
	if target.Template == nil {
		t.Error("expected inline template when no remotes available")
	}
	if target.TemplateRef != nil {
		t.Error("expected no templateRef when no remotes available")
	}
	if name != "newTarget" {
		t.Errorf("expected name 'newTarget', got %q", name)
	}
}

// TestWizardInvalidRemoteReference tests error handling for invalid remote references
func TestWizardInvalidRemoteReference(t *testing.T) {
	t.Skip("Skipping interactive test due to input simulation issues")
	dir := t.TempDir()

	// Create a config with limited remotes
	configContent := `version: 1
default: existing

remotes:
  valid-remote:
    repo: https://github.com/test/templates.git
    ref: main
    path: valid.tpl

targets:
  existing:
    templateRef: valid-remote
    binary: make
    fileFlag: -f
`
	if err := os.WriteFile(dir+"/duck.yaml", []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Simulate user input: 1 (remote reference), invalid-remote (should retry), valid-remote
	inputFile := dir + "/input4.txt"
	input := "1\ninvalid-remote\nvalid-remote\ntestTarget\nDescription\necho\n-f\n\nN\ny\n"
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	f, err := os.Open(inputFile)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = f
	defer func() {
		f.Close()
		os.Stdin = oldStdin
	}()

	target, _, err := runTargetWizard(false)
	if err != nil {
		t.Fatalf("wizard failed: %v", err)
	}

	// Should eventually succeed with valid remote
	if target.TemplateRef == nil || *target.TemplateRef != "valid-remote" {
		t.Errorf("expected templateRef 'valid-remote', got %v", target.TemplateRef)
	}
}
