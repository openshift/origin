package test

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/openshift/source-to-image/pkg/util"
)

// FakeCmdRunner provider the fake command runner
type FakeCmdRunner struct {
	Name string
	Args []string
	Opts util.CommandOpts
	Err  error
}

// RunWithOptions runs the command runner with extra options
func (f *FakeCmdRunner) RunWithOptions(opts util.CommandOpts, name string, args ...string) error {
	f.Name = name
	f.Args = args
	f.Opts = opts
	return f.Err
}

// Run runs the fake command runner
func (f *FakeCmdRunner) Run(name string, args ...string) error {
	return f.RunWithOptions(util.CommandOpts{}, name, args...)
}

// StartWithStdoutPipe executes a command returning a ReadCloser connected to
// the command's stdout.
func (f *FakeCmdRunner) StartWithStdoutPipe(opts util.CommandOpts, name string, arg ...string) (io.ReadCloser, error) {
	return ioutil.NopCloser(&bytes.Buffer{}), f.Err
}

// Wait waits for the command to exit.
func (f *FakeCmdRunner) Wait() error {
	return f.Err
}
