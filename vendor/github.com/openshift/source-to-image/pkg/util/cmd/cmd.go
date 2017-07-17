package cmd

import (
	"io"
	"os"
	"os/exec"
)

// CommandOpts contains options to attach Stdout/err to a command to run
// or set its initial directory
type CommandOpts struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Dir       string
	EnvAppend []string
}

// CommandRunner executes OS commands with the given parameters and options
type CommandRunner interface {
	RunWithOptions(opts CommandOpts, name string, arg ...string) error
	Run(name string, arg ...string) error
	StartWithStdoutPipe(opts CommandOpts, name string, arg ...string) (io.ReadCloser, error)
	Wait() error
}

// NewCommandRunner creates a new instance of the default implementation of
// CommandRunner
func NewCommandRunner() CommandRunner {
	return &runner{}
}

type runner struct {
	cmd *exec.Cmd
}

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
	if len(opts.EnvAppend) > 0 {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, opts.EnvAppend...)
	}
	return cmd.Run()
}

// Run executes a command with default options
func (c *runner) Run(name string, arg ...string) error {
	return c.RunWithOptions(CommandOpts{}, name, arg...)
}

// StartWithStdoutPipe executes a command returning a ReadCloser connected to
// the command's stdout.
func (c *runner) StartWithStdoutPipe(opts CommandOpts, name string, arg ...string) (io.ReadCloser, error) {
	c.cmd = exec.Command(name, arg...)
	if opts.Stderr != nil {
		c.cmd.Stderr = opts.Stderr
	}
	if opts.Dir != "" {
		c.cmd.Dir = opts.Dir
	}
	if len(opts.EnvAppend) > 0 {
		c.cmd.Env = os.Environ()
		c.cmd.Env = append(c.cmd.Env, opts.EnvAppend...)
	}
	r, err := c.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return r, c.cmd.Start()
}

// Wait waits for the command to exit.
func (c *runner) Wait() error {
	return c.cmd.Wait()
}
