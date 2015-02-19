package util

import (
	"io"
	"os/exec"
)

// CommandOpts contains options to attach Stdout/err to a command to run
// or set its initial directory
type CommandOpts struct {
	Stdout io.Writer
	Stderr io.Writer
	Dir    string
}

// CommandRunner executes OS commands with the given parameters and options
type CommandRunner interface {
	RunWithOptions(opts CommandOpts, name string, arg ...string) error
	Run(name string, arg ...string) error
}

// NewCommandRunner creates a new instance of the default implementation of
// CommandRunner
func NewCommandRunner() CommandRunner {
	return &runner{}
}

type runner struct{}

// RunWithOptions runs a command with the provided options
func (c *runner) RunWithOptions(opts CommandOpts, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	return cmd.Run()
}

// Run executes a command with default options
func (c *runner) Run(name string, arg ...string) error {
	return c.RunWithOptions(CommandOpts{}, name, arg...)
}
