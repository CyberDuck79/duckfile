package run

import "os/exec"

// TestSetCloneFunc overrides the internal cloneFunc seam for external (cmd package) tests.
func TestSetCloneFunc(f func(string, string, string) (string, error)) { cloneFunc = f }

// TestGetCloneFunc returns the current cloneFunc seam.
func TestGetCloneFunc() func(string, string, string) (string, error) { return cloneFunc }

// TestSetExecCommand overrides the execCommand seam.
func TestSetExecCommand(f func(string, ...string) *exec.Cmd) { execCommand = f }

// TestGetExecCommand exposes current execCommand seam.
func TestGetExecCommand() func(string, ...string) *exec.Cmd { return execCommand }
