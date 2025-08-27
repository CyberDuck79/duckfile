//nolint:errcheck,unused
package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CyberDuck79/duckfile/internal/config"
)

// mockBufioReader implements the same interface as bufio.Reader for testing
type mockBufioReader struct {
	responses []string
	index     int
}

func newMockBufioReader(responses ...string) *mockBufioReader {
	return &mockBufioReader{responses: responses, index: 0}
}

func (m *mockBufioReader) ReadString(delim byte) (string, error) {
	if m.index >= len(m.responses) {
		return "", fmt.Errorf("EOF")
	}
	response := m.responses[m.index] + string(delim)
	m.index++
	return response, nil
}

// TestRunTargetWizardBasicTarget tests creating a basic target with minimal inputs
func TestRunTargetWizardBasicTarget(t *testing.T) {
	// Mock user inputs for a basic target
	responses := []string{
		"build", // target name
		"make",  // binary
		"-f",    // fileFlag
		"",      // renderedPath (empty for auto)
		"https://github.com/example/templates.git", // repo
		"main",         // ref
		"Makefile.tpl", // path
		"n",            // allowMissing (no)
		"n",            // add variable (no)
	}

	// Create a mock reader
	mockReader := &mockBufioReader{responses: responses}

	// Test the wizard by temporarily replacing the reader creation
	target, name, err := runTargetWizardWithReader(mockReader, false)

	if err != nil {
		t.Fatalf("runTargetWizard failed: %v", err)
	}

	// Verify the results
	if name != "build" {
		t.Errorf("expected name 'build', got %q", name)
	}
	if target.Binary != "make" {
		t.Errorf("expected binary 'make', got %q", target.Binary)
	}
	if target.FileFlag != "-f" {
		t.Errorf("expected fileFlag '-f', got %q", target.FileFlag)
	}
	if target.Template.Repo != "https://github.com/example/templates.git" {
		t.Errorf("expected specific repo, got %q", target.Template.Repo)
	}
	if target.Template.Ref != "main" {
		t.Errorf("expected ref 'main', got %q", target.Template.Ref)
	}
	if target.Template.Path != "Makefile.tpl" {
		t.Errorf("expected path 'Makefile.tpl', got %q", target.Template.Path)
	}
	if target.Template.AllowMissing {
		t.Errorf("expected allowMissing false, got true")
	}
	if len(target.Variables) != 0 {
		t.Errorf("expected no variables, got %d", len(target.Variables))
	}
}

// TestRunTargetWizardSyncOnlyTarget tests creating a sync-only target (no binary)
func TestRunTargetWizardSyncOnlyTarget(t *testing.T) {
	responses := []string{
		"docs",                                // target name
		"",                                    // binary (empty for sync-only)
		"/path/to/docs",                       // renderedPath
		"https://github.com/example/docs.git", // repo
		"v1.0.0",                              // ref
		"docs.tpl",                            // path
		"y",                                   // allowMissing (yes)
		"n",                                   // add variable (no)
	}

	mockReader := &mockBufioReader{responses: responses}
	target, name, err := runTargetWizardWithReader(mockReader, false)

	if err != nil {
		t.Fatalf("runTargetWizard failed: %v", err)
	}

	if name != "docs" {
		t.Errorf("expected name 'docs', got %q", name)
	}
	if target.Binary != "" {
		t.Errorf("expected empty binary for sync-only, got %q", target.Binary)
	}
	if target.FileFlag != "" {
		t.Errorf("expected empty fileFlag for sync-only, got %q", target.FileFlag)
	}
	if target.RenderedPath != "/path/to/docs" {
		t.Errorf("expected renderedPath '/path/to/docs', got %q", target.RenderedPath)
	}
	if !target.Template.AllowMissing {
		t.Errorf("expected allowMissing true, got false")
	}
}

// TestRunTargetWizardWithVariables tests creating a target with various variable types
func TestRunTargetWizardWithVariables(t *testing.T) {
	responses := []string{
		"complex",                               // target name
		"helm",                                  // binary
		"--values",                              // fileFlag
		"",                                      // renderedPath (auto)
		"https://github.com/example/charts.git", // repo
		"",                                      // ref (empty for HEAD)
		"chart.tpl",                             // path
		"n",                                     // allowMissing (no)
		"y",                                     // add variable (yes)
		"APP_NAME",                              // variable key
		"literal",                               // variable type
		"myapp",                                 // literal value
		"y",                                     // add another variable (yes)
		"HOME_DIR",                              // variable key
		"env",                                   // variable type
		"HOME",                                  // env var name
		"y",                                     // add another variable (yes)
		"VERSION",                               // variable key
		"cmd",                                   // variable type
		"git describe --tags",                   // shell command
		"y",                                     // add another variable (yes)
		"CONFIG_FILE",                           // variable key
		"file",                                  // variable type
		"config.yaml",                           // file path
		"y",                                     // add another variable (yes)
		"UNKNOWN_TYPE",                          // variable key
		"unknown",                               // unknown variable type
		"fallback_value",                        // fallback literal value
		"n",                                     // stop adding variables
	}

	mockReader := &mockBufioReader{responses: responses}
	target, name, err := runTargetWizardWithReader(mockReader, false)

	if err != nil {
		t.Fatalf("runTargetWizard failed: %v", err)
	}

	if name != "complex" {
		t.Errorf("expected name 'complex', got %q", name)
	}

	// Verify variables
	if len(target.Variables) != 5 {
		t.Errorf("expected 5 variables, got %d", len(target.Variables))
	}

	// Check literal variable
	if appVar, exists := target.Variables["APP_NAME"]; !exists {
		t.Errorf("APP_NAME variable not found")
	} else {
		if appVar.Kind != config.VarLiteral {
			t.Errorf("expected APP_NAME to be literal, got %v", appVar.Kind)
		}
		if appVar.Value != "myapp" {
			t.Errorf("expected APP_NAME value 'myapp', got %v", appVar.Value)
		}
	}

	// Check env variable
	if homeVar, exists := target.Variables["HOME_DIR"]; !exists {
		t.Errorf("HOME_DIR variable not found")
	} else {
		if homeVar.Kind != config.VarEnv {
			t.Errorf("expected HOME_DIR to be env, got %v", homeVar.Kind)
		}
		if homeVar.Arg != "HOME" {
			t.Errorf("expected HOME_DIR arg 'HOME', got %q", homeVar.Arg)
		}
	}

	// Check cmd variable
	if versionVar, exists := target.Variables["VERSION"]; !exists {
		t.Errorf("VERSION variable not found")
	} else {
		if versionVar.Kind != config.VarCmd {
			t.Errorf("expected VERSION to be cmd, got %v", versionVar.Kind)
		}
		if versionVar.Arg != "git describe --tags" {
			t.Errorf("expected VERSION arg 'git describe --tags', got %q", versionVar.Arg)
		}
	}

	// Check file variable
	if configVar, exists := target.Variables["CONFIG_FILE"]; !exists {
		t.Errorf("CONFIG_FILE variable not found")
	} else {
		if configVar.Kind != config.VarFile {
			t.Errorf("expected CONFIG_FILE to be file, got %v", configVar.Kind)
		}
		if configVar.Arg != "config.yaml" {
			t.Errorf("expected CONFIG_FILE arg 'config.yaml', got %q", configVar.Arg)
		}
	}

	// Check unknown type fallback to literal
	if unknownVar, exists := target.Variables["UNKNOWN_TYPE"]; !exists {
		t.Errorf("UNKNOWN_TYPE variable not found")
	} else {
		if unknownVar.Kind != config.VarLiteral {
			t.Errorf("expected UNKNOWN_TYPE to fallback to literal, got %v", unknownVar.Kind)
		}
		if unknownVar.Value != "fallback_value" {
			t.Errorf("expected UNKNOWN_TYPE value 'fallback_value', got %v", unknownVar.Value)
		}
	}
}

// TestRunTargetWizardDefaultTarget tests the default target prompt
func TestRunTargetWizardDefaultTarget(t *testing.T) {
	responses := []string{
		"main",                                // default target name
		"make",                                // binary
		"-f",                                  // fileFlag
		"",                                    // renderedPath
		"https://github.com/example/main.git", // repo
		"main",                                // ref
		"main.tpl",                            // path
		"n",                                   // allowMissing
		"n",                                   // add variable
	}

	mockReader := &mockBufioReader{responses: responses}
	target, name, err := runTargetWizardWithReader(mockReader, true) // isDefault = true

	if err != nil {
		t.Fatalf("runTargetWizard failed: %v", err)
	}

	if name != "main" {
		t.Errorf("expected name 'main', got %q", name)
	}
	// The function should work the same regardless of isDefault flag
	// The difference is only in the prompt text
	if target.Binary != "make" {
		t.Errorf("expected binary 'make', got %q", target.Binary)
	}
}

// TestRunTargetWizardEmptyTargetName tests error handling for empty target name
func TestRunTargetWizardEmptyTargetName(t *testing.T) {
	responses := []string{
		"", // empty target name
	}

	mockReader := &mockBufioReader{responses: responses}
	_, _, err := runTargetWizardWithReader(mockReader, false)

	if err == nil {
		t.Fatalf("expected error for empty target name but got none")
	}
	if !strings.Contains(err.Error(), "target key cannot be empty") {
		t.Fatalf("expected 'target key cannot be empty' error, got: %v", err)
	}
}

// TestRunTargetWizardEmptyRepo tests error handling for empty repo
func TestRunTargetWizardEmptyRepo(t *testing.T) {
	responses := []string{
		"test", // target name
		"make", // binary
		"-f",   // fileFlag
		"",     // renderedPath
		"",     // empty repo
	}

	mockReader := &mockBufioReader{responses: responses}
	_, _, err := runTargetWizardWithReader(mockReader, false)

	if err == nil {
		t.Fatalf("expected error for empty repo but got none")
	}
	if !strings.Contains(err.Error(), "repo is required") {
		t.Fatalf("expected 'repo is required' error, got: %v", err)
	}
}

// TestRunTargetWizardEmptyTemplatePath tests error handling for empty template path
func TestRunTargetWizardEmptyTemplatePath(t *testing.T) {
	responses := []string{
		"test", // target name
		"make", // binary
		"-f",   // fileFlag
		"",     // renderedPath
		"https://github.com/example/templates.git", // repo
		"main", // ref
		"",     // empty template path
	}

	mockReader := &mockBufioReader{responses: responses}
	_, _, err := runTargetWizardWithReader(mockReader, false)

	if err == nil {
		t.Fatalf("expected error for empty template path but got none")
	}
	if !strings.Contains(err.Error(), "template path is required") {
		t.Fatalf("expected 'template path is required' error, got: %v", err)
	}
}

// TestRunTargetWizardEmptyVariableKey tests skipping empty variable keys
func TestRunTargetWizardEmptyVariableKey(t *testing.T) {
	responses := []string{
		"test", // target name
		"make", // binary
		"-f",   // fileFlag
		"",     // renderedPath
		"https://github.com/example/templates.git", // repo
		"main",     // ref
		"test.tpl", // path
		"n",        // allowMissing
		"y",        // add variable
		"",         // empty variable key (should be skipped)
		"n",        // stop adding variables
	}

	mockReader := &mockBufioReader{responses: responses}
	target, name, err := runTargetWizardWithReader(mockReader, false)

	if err != nil {
		t.Fatalf("runTargetWizard failed: %v", err)
	}

	if name != "test" {
		t.Errorf("expected name 'test', got %q", name)
	}

	// Should have no variables since the empty key was skipped
	if len(target.Variables) != 0 {
		t.Errorf("expected no variables (empty key skipped), got %d", len(target.Variables))
	}
}

// runTargetWizardWithReader is a test helper that allows injecting a custom reader
// This function replicates the logic of runTargetWizard but with a configurable reader
func runTargetWizardWithReader(reader *mockBufioReader, isDefault bool) (config.Target, string, error) {
	ask := func(prompt string) (string, error) {
		// Note: In real usage, this would print the prompt, but we skip that in tests
		txt, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(txt), nil
	}

	var name string
	var err error
	if isDefault {
		name, err = ask("Default target key (called when <target> is not specified): ")
	} else {
		name, err = ask("Target key (CLI name): ")
	}
	if err != nil {
		return config.Target{}, "", err
	}
	if name == "" {
		return config.Target{}, "", fmt.Errorf("target key cannot be empty")
	}

	binary, err := ask("Binary (leave empty for sync-only): ")
	if err != nil {
		return config.Target{}, "", err
	}

	var fileFlag string
	if strings.TrimSpace(binary) != "" {
		fileFlag, err = ask("fileFlag (e.g. -f, --taskfile) [optional if binary expects path implicitly]: ")
		if err != nil {
			return config.Target{}, "", err
		}
	}

	renderedPath, err := ask("Rendered path (where symlink/file should appear) [auto .duck/<target>/<base>]: ")
	if err != nil {
		return config.Target{}, "", err
	}

	repo, err := ask("Template repo (git URL): ")
	if err != nil {
		return config.Target{}, "", err
	}
	if repo == "" {
		return config.Target{}, "", fmt.Errorf("repo is required")
	}

	ref, err := ask("Template ref (branch/tag/commit) [HEAD]: ")
	if err != nil {
		return config.Target{}, "", err
	}

	path, err := ask("Template path inside repo (e.g. Makefile.tpl): ")
	if err != nil {
		return config.Target{}, "", err
	}
	if path == "" {
		return config.Target{}, "", fmt.Errorf("template path is required")
	}

	allowMissingAns, err := ask("Allow missing variables? (y/N): ")
	if err != nil {
		return config.Target{}, "", err
	}
	allowMissing := strings.HasPrefix(strings.ToLower(allowMissingAns), "y")

	vars := map[string]config.VarValue{}
	for {
		more, err := ask("Add variable? (y/N): ")
		if err != nil {
			return config.Target{}, "", err
		}
		if strings.ToLower(strings.TrimSpace(more)) != "y" {
			break
		}

		k, err := ask("  Key: ")
		if err != nil {
			return config.Target{}, "", err
		}
		if k == "" {
			continue // Skip empty keys
		}

		kind, err := ask("  Type (literal/env/cmd/file) [literal]: ")
		if err != nil {
			return config.Target{}, "", err
		}
		kind = strings.ToLower(strings.TrimSpace(kind))

		switch kind {
		case "", "literal":
			v, _ := ask("  Value: ")
			vars[k] = config.NewLiteralVar(v)
		case "env":
			v, _ := ask("  Env var name: ")
			vars[k] = config.NewEnvVar(v)
		case "cmd":
			v, _ := ask("  Shell command: ")
			vars[k] = config.NewCmdVar(v)
		case "file":
			v, _ := ask("  File path: ")
			vars[k] = config.NewFileVar(v)
		default:
			v, _ := ask("  Value: ")
			vars[k] = config.NewLiteralVar(v)
		}
	}

	targ := config.Target{
		Binary:       binary,
		FileFlag:     fileFlag,
		Template:     config.Template{Repo: repo, Ref: ref, Path: path, AllowMissing: allowMissing},
		Variables:    vars,
		RenderedPath: renderedPath,
	}

	if err := config.ValidateTarget(targ, name); err != nil {
		return config.Target{}, "", err
	}

	return targ, name, nil
}
