package test

import (
	"github.com/openshift/source-to-image/pkg/sti/util"
)

type FakeCmdRunner struct {
	Name string
	Args []string
	Opts util.CommandOpts
	Err  error
}

func (f *FakeCmdRunner) RunWithOptions(opts util.CommandOpts, name string, args ...string) error {
	f.Name = name
	f.Args = args
	f.Opts = opts
	return f.Err
}

func (f *FakeCmdRunner) Run(name string, args ...string) error {
	return f.RunWithOptions(util.CommandOpts{}, name, args...)
}
