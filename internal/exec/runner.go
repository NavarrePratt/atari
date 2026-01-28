// Package exec provides command execution abstractions for production use.
package exec

import (
	"context"
	"os/exec"
)

// CommandRunner abstracts command execution for dependency injection.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner executes real commands using os/exec.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner for production use.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes a command and returns its combined output.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := execCommand(ctx, name, args...)
	return cmd.Output()
}

// execCommand is a variable to allow testing.
var execCommand = execCommandImpl

func execCommandImpl(ctx context.Context, name string, args ...string) execCmd {
	return realExecCmd{cmd: exec.CommandContext(ctx, name, args...)}
}

// execCmd abstracts exec.Cmd for testing.
type execCmd interface {
	Output() ([]byte, error)
}

type realExecCmd struct {
	cmd *exec.Cmd
}

func (c realExecCmd) Output() ([]byte, error) {
	return c.cmd.Output()
}
