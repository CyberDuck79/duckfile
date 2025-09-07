package run

import (
	"os"
	"os/exec"
	"time"

	"github.com/CyberDuck79/duckfile/internal/git"
)

// Test seams (overridable in tests for determinism / stubbing)
var (
	nowFunc              = time.Now
	getenvFunc           = os.Getenv
	execCommand          = exec.Command
	cloneFunc            = git.CloneInto
	getRemoteCommitFunc  = git.GetRemoteCommitHash
	getCurrentCommitFunc = git.GetCurrentCommitHash
)

// TestSetCloneFunc overrides the internal cloneFunc seam for external (cmd package) tests.
func TestSetCloneFunc(f func(string, string, string, bool) (string, error)) { cloneFunc = f }

// TestGetCloneFunc returns the current cloneFunc seam.
func TestGetCloneFunc() func(string, string, string, bool) (string, error) { return cloneFunc }

// TestSetExecCommand overrides the execCommand seam.
func TestSetExecCommand(f func(string, ...string) *exec.Cmd) { execCommand = f }

// TestGetExecCommand exposes current execCommand seam.
func TestGetExecCommand() func(string, ...string) *exec.Cmd { return execCommand }
