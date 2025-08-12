//nolint:errcheck
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeListConfig writes a duck.yaml suitable for list command tests.
func writeListConfig(t *testing.T, dir string) {
	t.Helper()
	body := `version: 1
default: build

targets:
  build:
    description: Build target
    binary: make
    fileFlag: -f
    template:
      repo: repoA
      ref: main
      path: Makefile.tpl
    variables:
      APP: myapp
  test:
    description: Test target
    binary: go
    fileFlag: -f
    template:
      repo: repoB
      ref: v1.2.3
      path: test.tpl
    variables:
      ZED: last
      ALPHA: !env HOME
      MID: !cmd echo hi
      FILEX: !file some.file
    args: ["test","./..."]
  docs:
    description: Docs only
    template:
      repo: repoC
      ref: head
      path: docs.tpl
`
	if err := os.WriteFile(filepath.Join(dir, "duck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create referenced file for !file variable (contents irrelevant for list)
	os.WriteFile(filepath.Join(dir, "some.file"), []byte("data"), 0o644)
}

// runList executes the list command with provided flags and returns captured stdout.
func runList(t *testing.T, dir string, flags ...string) string {
	t.Helper()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	// Capture global stdout because list uses fmt.Printf directly
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	args := append([]string{"list"}, flags...)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	_ = rootCmd.Execute()
	w.Close()
	os.Stdout = origStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

// TestListNoFlags ensures output includes headers, target names, descriptions, and
// excludes remote/template lines or exec/variables details when no flags supplied.
func TestListNoFlags(t *testing.T) {
	dir := t.TempDir()
	writeListConfig(t, dir)
	out := runList(t, dir)
	if !strings.Contains(out, "TARGET") || !strings.Contains(out, "BINARY") || !strings.Contains(out, "DESCRIPTION") {
		t.Fatalf("missing headers: \n%s", out)
	}
	if !strings.Contains(out, "build") || !strings.Contains(out, "docs") || !strings.Contains(out, "test") {
		t.Fatalf("missing names: \n%s", out)
	}
	if !strings.Contains(out, "build*") {
		t.Fatalf("missing default symbol after default target name (*): \n%s", out)
	}
	if !strings.Contains(out, "make") || !strings.Contains(out, "-") || !strings.Contains(out, "go") {
		t.Fatalf("missing binary or empty binary symbol (-): %s", out)
	}
	if strings.Contains(out, "repo:") || strings.Contains(out, "variables (") || strings.Contains(out, "exec:") {
		t.Fatalf("unexpected remote/vars/exec sections without flags: %s", out)
	}
}

// TestListRemoteFlag verifies -r adds repo/ref/path lines and they are absent without it.
func TestListRemoteFlag(t *testing.T) {
	dir := t.TempDir()
	writeListConfig(t, dir)
	out := runList(t, dir, "-r")
	if !strings.Contains(out, "repo: repoA") || !strings.Contains(out, "repo: repoB") || !strings.Contains(out, "path: Makefile.tpl") {
		t.Fatalf("expected remote info lines, got: %s", out)
	}
	if !strings.Contains(out, "ref: main") || !strings.Contains(out, "ref: v1.2.3") {
		t.Fatalf("missing ref lines: %s", out)
	}
}

// TestListVarsFlag ensures -v prints a sorted list of variable names with their kinds.
func TestListVarsFlag(t *testing.T) {
	dir := t.TempDir()
	writeListConfig(t, dir)
	out := runList(t, dir, "-v")
	// Find the variables block for target 'test'
	lines := strings.Split(out, "\n")
	varsStart := -1
	for i, l := range lines {
		if strings.Contains(l, "variables (4):") {
			varsStart = i
			break
		}
	}
	if varsStart == -1 {
		t.Fatalf("variables block not found: %s", out)
	}
	listed := []string{}
	for _, l := range lines[varsStart+1:] {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "-") { // variable line like - NAME (kind)
			parts := strings.Fields(l)
			if len(parts) > 1 {
				listed = append(listed, strings.TrimPrefix(parts[0], "-"))
			}
		} else if l == "" || !strings.HasPrefix(l, "-") { // stop when block ends
			break
		}
	}
	expect := append([]string{}, listed...)
	sort.Strings(expect)
	if len(listed) != 4 || !equalSlices(listed, expect) {
		t.Fatalf("expected sorted variable names, got %v", listed)
	}
	// spot check kinds
	if !strings.Contains(out, "ALPHA (env)") || !strings.Contains(out, "MID (cmd)") || !strings.Contains(out, "FILEX (file)") || !strings.Contains(out, "ZED (literal)") {
		t.Fatalf("missing variable kinds: %s", out)
	}
}

// TestListExecFlag checks -e prints exec lines only for targets with a binary.
func TestListExecFlag(t *testing.T) {
	dir := t.TempDir()
	writeListConfig(t, dir)
	out := runList(t, dir, "-e")
	if !strings.Contains(out, "exec: make -f <rendered>") || !strings.Contains(out, "exec: go -f <rendered> test ./...") {
		t.Fatalf("expected exec lines for targets with binaries: %s", out)
	}
	if strings.Contains(out, "docs") && strings.Contains(out, "exec: docs") { // ensure docs target does not have exec line
		t.Fatalf("sync-only docs target should not have exec line: %s", out)
	}
}

// TestListDescriptions ensures each target's description appears in the baseline output.
func TestListDescriptions(t *testing.T) {
	dir := t.TempDir()
	writeListConfig(t, dir)
	out := runList(t, dir)
	if !strings.Contains(out, "Build target") || !strings.Contains(out, "Test target") || !strings.Contains(out, "Docs only") {
		t.Fatalf("missing descriptions: %s", out)
	}
}

// equalSlices compares two string slices element-wise.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
